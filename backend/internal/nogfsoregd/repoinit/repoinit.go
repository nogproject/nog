// `repoinit.Processor` watches a `fsoregistry` event journal and tells
// `fsorepos.Repos` to initialize repo instances.
//
// XXX The processing logic might perhaps be merged with package `replicate`.
// But it is not straightforward, because repoinit needs to maintain state
// about the roots.
package repoinit

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	mainpb "github.com/nogproject/nog/backend/internal/fsomainpb"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	pbevents "github.com/nogproject/nog/backend/internal/fsoregistry/pbevents"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const NsFsoRegistry = "fsoreg"

type Processor struct {
	names         *shorteruuid.Names
	lg            Logger
	mainJ         *events.Journal
	main          *fsomain.Main
	mainId        uuid.I
	registryJ     *events.Journal
	registry      *fsoregistry.Registry
	repos         *fsorepos.Repos
	mainTail      ulid.I
	tails         map[uuid.I]ulid.I
	roots         map[string]*RootInfo
	registryNames map[uuid.I]string
}

type RootInfo struct {
	GlobalRoot             string
	Host                   string
	HostRoot               string
	GitlabNamespace        string
	InitPolicy             *pb.FsoRepoInitPolicy
	ArchiveRecipients      gpg.Fingerprints
	ShadowBackupRecipients gpg.Fingerprints
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

func NewProcessor(
	names *shorteruuid.Names,
	lg Logger,
	mainJ *events.Journal,
	main *fsomain.Main,
	mainId uuid.I,
	registryJ *events.Journal,
	registry *fsoregistry.Registry,
	repos *fsorepos.Repos,
) *Processor {
	return &Processor{
		names:         names,
		lg:            lg,
		mainJ:         mainJ,
		main:          main,
		mainId:        mainId,
		registryJ:     registryJ,
		registry:      registry,
		repos:         repos,
		mainTail:      events.EventEpoch,
		tails:         make(map[uuid.I]ulid.I),
		roots:         make(map[string]*RootInfo),
		registryNames: make(map[uuid.I]string),
	}
}

func (p *Processor) Process(ctx context.Context) error {
	// First subscribe, then init, so that no events can get lost.

	// Watch mainId on main journal.
	mainUpdates := make(chan uuid.I, 100)
	p.mainJ.Subscribe(mainUpdates, p.mainId)
	defer p.mainJ.Unsubscribe(mainUpdates)

	// Watch all events on registry journal.
	regUpdates := make(chan uuid.I, 100)
	p.registryJ.Subscribe(regUpdates, events.WildcardTopic)
	defer p.registryJ.Unsubscribe(regUpdates)

	if err := p.initRetry(ctx); err != nil {
		return err
	}

	// Trigger updateAll() from time to time; just in case.
	updateAllPeriod := 1 * time.Minute
	ticker := time.NewTicker(updateAllPeriod)
	defer ticker.Stop()

	for {
		// Ignore if poll with retry failed.  Ticker will trigger
		// another poll.
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := p.retryUntilCancel(ctx, "update main", func() error {
				return p.updateMain()
			}); err != nil {
				return err
			}

			if err := p.retryUntilCancel(ctx, "update all registries", func() error {
				return p.updateAllRegistries()
			}); err != nil {
				return err
			}

		case <-mainUpdates:
			if err := p.retryUntilCancel(ctx, "update main", func() error {
				return p.updateMain()
			}); err != nil {
				return err
			}

		case regId := <-regUpdates:
			if p.tails[regId] == ulid.Nil {
				// Ignore registries that have not been added
				// via main.
				continue
			}
			if err := p.retryUntilCancel(ctx, "update registry", func() error {
				return p.updateRegistry(regId)
			}); err != nil {
				return err
			}
		}
	}
}

func (p *Processor) initRetry(ctx context.Context) error {
	err := p.retryUntilCancel(ctx, "init", func() error {
		return p.init(ctx)
	})
	return err
}

func (p *Processor) init(ctx context.Context) error {
	main, err := p.main.FindId(p.mainId)
	if err != nil {
		return err
	}

	for _, reg := range main.Registries() {
		if reg.Confirmed {
			if err := p.initRegistry(ctx, reg.Name); err != nil {
				return err
			}
		}
	}

	p.mainTail = main.Vid()
	return nil
}

// `initRegistry()` loads events, via `loadRegistryState()`, because the
// required state is not readily available from the `fsoregistry` aggregate
// state; specifically the following information is not available:
//
//  - The repo creator.
//  - The init policy.
//  - State of move-repo workflows.
//
func (p *Processor) initRegistry(ctx context.Context, regName string) error {
	regId := p.names.UUID(NsFsoRegistry, regName)

	st, err := p.loadRegistryState(ctx, regId)
	if err != nil {
		return err
	}

	// Merge the state before processing commands, because some commands,
	// for example `initRepo()`, require the merged state.
	p.mergeRegistryState(st)

	for _, inf := range st.initRepo {
		if err := p.initRepo(regId, inf); err != nil {
			return err
		}
	}
	for _, inf := range st.beginMoveRepo {
		if err := p.beginMoveRepo(regId, inf); err != nil {
			return err
		}
	}
	for repoId, gitlabNamespace := range st.enableGitlab {
		if err := p.enableGitlab(repoId, gitlabNamespace); err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) mergeRegistryState(st *registryState) {
	p.registryNames[st.id] = st.name
	for k, v := range st.roots {
		p.roots[k] = v
	}
	p.tails[st.id] = st.vid
}

type moveRepoInfo struct {
	RegistryEventId ulid.I
	RepoId          uuid.I
	WorkflowId      uuid.I
	NewGlobalPath   string
}

type registryState struct {
	id            uuid.I
	vid           ulid.I
	name          string
	roots         map[string]*RootInfo
	initRepo      map[uuid.I]*pb.FsoRepoInfo
	beginMoveRepo map[uuid.I]*moveRepoInfo
	enableGitlab  map[uuid.I]string
}

func newRegistryState() *registryState {
	return &registryState{
		roots:         make(map[string]*RootInfo),
		initRepo:      make(map[uuid.I]*pb.FsoRepoInfo),
		enableGitlab:  make(map[uuid.I]string),
		beginMoveRepo: make(map[uuid.I]*moveRepoInfo),
	}
}

func (p *Processor) loadRegistryState(
	ctx context.Context, regId uuid.I,
) (*registryState, error) {
	st := newRegistryState()
	st.id = regId
	err := p.forEachRegistryEventAfter(regId, events.EventEpoch, func(
		vid ulid.I, regEv *pb.RegistryEvent,
	) error {
		if err := st.applyEvent(p, vid, regEv); err != nil {
			return err
		}
		st.vid = vid
		return nil
	})
	return st, err
}

func (st *registryState) applyEvent(
	p *Processor, regVid ulid.I, regEv *pb.RegistryEvent,
) error {
	switch regEv.Event {
	case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
		st.name = regEv.FsoRegistryInfo.Name
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
		inf := regEv.FsoRootInfo
		st.roots[inf.GlobalRoot] = &RootInfo{
			GlobalRoot:      inf.GlobalRoot,
			Host:            inf.Host,
			HostRoot:        inf.HostRoot,
			GitlabNamespace: inf.GitlabNamespace,
		}
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
		inf := regEv.FsoRootInfo
		root := st.roots[inf.GlobalRoot]
		root.Host = inf.Host
		root.HostRoot = inf.HostRoot
		root.GitlabNamespace = inf.GitlabNamespace
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
		inf := regEv.FsoRootInfo
		delete(st.roots, inf.GlobalRoot)
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED:
		pol := regEv.FsoRepoInitPolicy
		st.roots[pol.GlobalRoot].InitPolicy = pol
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
		rootPath := regEv.FsoRootInfo.GlobalRoot
		root := st.roots[rootPath]
		keys, err := gpg.ParseFingerprintsBytes(
			regEv.FsoGpgKeyFingerprints...,
		)
		if err != nil {
			return err
		}
		root.ArchiveRecipients = keys
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		rootPath := regEv.FsoRootInfo.GlobalRoot
		root := st.roots[rootPath]
		keys, err := gpg.ParseFingerprintsBytes(
			regEv.FsoGpgKeyFingerprints...,
		)
		if err != nil {
			return err
		}
		root.ShadowBackupRecipients = keys
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
		inf := regEv.FsoRepoInfo
		repoId, err := uuid.FromBytes(inf.Id)
		if err != nil {
			return err
		}
		st.initRepo[repoId] = inf
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		inf := regEv.FsoRepoInfo
		repoId, err := uuid.FromBytes(inf.Id)
		if err != nil {
			return err
		}
		delete(st.initRepo, repoId)
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED:
		x := pbevents.FromPbMust(*regEv).(*pbevents.EvRepoMoveAccepted)
		st.beginMoveRepo[x.RepoId] = &moveRepoInfo{
			RegistryEventId: regVid,
			RepoId:          x.RepoId,
			WorkflowId:      x.WorkflowId,
			NewGlobalPath:   x.NewGlobalPath,
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVED:
		x := pbevents.FromPbMust(*regEv).(*pbevents.EvRepoMoved)
		delete(st.beginMoveRepo, x.RepoId)
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
		x := pbevents.FromPbMust(*regEv).(*pbevents.EvRepoEnableGitlabAccepted)
		repo, err := p.repos.FindId(x.RepoId)
		if err != nil {
			return err
		}
		if repo.GitlabLocation() == "" {
			st.enableGitlab[x.RepoId] = x.GitlabNamespace
		}
		return nil

	// Ignore unrelated:
	case pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED:
		return nil

	// `EV_FSO_REPO_REINIT_ACCEPTED` only tells
	// `nogfsostad.Observer` to retry init.
	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
		return nil

	// `*_REPO_NAMING_*` is only relevant when listing untracked
	// repos.  No need to propagate information to existing repos.
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
		return nil

	// `RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED` is
	// replicated from the repo.
	case pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return nil

	// Ignore splitrootwf:
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_PATH_FLAG_SET:
		return nil
	case pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET:
		return nil

	// Ignore freezerepowf.
	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return nil
	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return nil

	// Ignore unfreezerepowf.
	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return nil
	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return nil

	// Ignore archiverepowf.
	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return nil

	// Ignore unarchiverepowf.
	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return nil

	default: // Ignore unknown.
		p.lg.Errorw(
			"Ignored unknown registry event.",
			"module", "repoinit",
			"event", regEv.Event,
		)
		return nil
	}
}

func (p *Processor) updateMain() error {
	iter := p.mainJ.Find(p.mainId, p.mainTail)
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var ev fsomain.Event
	for iter.Next(&ev) {
		mainEv := ev.PbMainEvent()
		switch mainEv.Event {
		case mainpb.Event_EV_FSO_MAIN_INITIALIZED:
			// Not interested in name of main.

		case mainpb.Event_EV_FSO_REGISTRY_ACCEPTED:
			// Not interested in accept.

		case mainpb.Event_EV_FSO_REGISTRY_CONFIRMED:
			err := p.addRegistry(mainEv.FsoRegistryName)
			if err != nil {
				return err
			}

		default:
			// Ignore unknown.
			p.lg.Warnw(
				"Ignored unknown main event.",
				"module", "repoinit",
				"event", mainEv.Event,
			)
		}
		p.mainTail = ev.Id()
	}
	if err := iterClose(); err != nil {
		return err
	}

	return nil
}

func (p *Processor) addRegistry(name string) error {
	id := p.names.UUID(NsFsoRegistry, name)
	if p.tails[id] != ulid.Nil {
		// Already known
		return nil
	}
	err := p.updateRegistry(id)
	if err != nil {
		return err
	}
	p.lg.Infow(
		"Added registry poll for new repos.",
		"module", "repoinit",
		"registry", name,
	)
	return nil
}

func (p *Processor) updateAllRegistries() error {
	nErr := 0
	for id, _ := range p.tails {
		err := p.updateRegistry(id)
		if err != nil {
			p.lg.Errorw(
				"Registry poll failed during poll all.",
				"module", "repoinit",
				"err", err,
				"registryId", id,
			)
			nErr++
		}
	}
	if nErr > 0 {
		err := fmt.Errorf("%d updates failed", nErr)
		return err
	}

	return nil
}

func (p *Processor) updateRegistry(regId uuid.I) error {
	tail, ok := p.tails[regId]
	if !ok {
		tail = events.EventEpoch
	}
	return p.forEachRegistryEventAfter(regId, tail, func(
		vid ulid.I, regEv *pb.RegistryEvent,
	) error {
		if err := p.applyRegistryEvent(regId, vid, regEv); err != nil {
			return err
		}
		p.tails[regId] = vid
		return nil
	})
}

func (p *Processor) applyRegistryEvent(
	regId uuid.I, regVid ulid.I, regEv *pb.RegistryEvent,
) error {
	switch regEv.Event {
	case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
		p.registryNames[regId] = regEv.FsoRegistryInfo.Name
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
		inf := regEv.FsoRootInfo
		p.roots[inf.GlobalRoot] = &RootInfo{
			GlobalRoot:      inf.GlobalRoot,
			Host:            inf.Host,
			HostRoot:        inf.HostRoot,
			GitlabNamespace: inf.GitlabNamespace,
		}
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
		inf := regEv.FsoRootInfo
		root := p.roots[inf.GlobalRoot]
		root.Host = inf.Host
		root.HostRoot = inf.HostRoot
		root.GitlabNamespace = inf.GitlabNamespace
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
		delete(p.roots, regEv.FsoRootInfo.GlobalRoot)
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED:
		pol := regEv.FsoRepoInitPolicy
		p.roots[pol.GlobalRoot].InitPolicy = pol
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
		rootPath := regEv.FsoRootInfo.GlobalRoot
		root := p.roots[rootPath]
		keys, err := gpg.ParseFingerprintsBytes(
			regEv.FsoGpgKeyFingerprints...,
		)
		if err != nil {
			return err
		}
		root.ArchiveRecipients = keys
		return nil

	case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		rootPath := regEv.FsoRootInfo.GlobalRoot
		root := p.roots[rootPath]
		keys, err := gpg.ParseFingerprintsBytes(
			regEv.FsoGpgKeyFingerprints...,
		)
		if err != nil {
			return err
		}
		root.ShadowBackupRecipients = keys
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
		inf := regEv.FsoRepoInfo
		return p.initRepo(regId, inf)
	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED:
		x := pbevents.FromPbMust(*regEv).(*pbevents.EvRepoMoveAccepted)
		return p.beginMoveRepo(regId, &moveRepoInfo{
			RegistryEventId: regVid,
			RepoId:          x.RepoId,
			WorkflowId:      x.WorkflowId,
			NewGlobalPath:   x.NewGlobalPath,
		})
	case pb.RegistryEvent_EV_FSO_REPO_MOVED:
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
		x := pbevents.FromPbMust(*regEv).(*pbevents.EvRepoEnableGitlabAccepted)
		return p.enableGitlab(x.RepoId, x.GitlabNamespace)

	// Ignore unrelated:
	case pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED:
		return nil

	// `EV_FSO_REPO_REINIT_ACCEPTED` only tells
	// `nogfsostad.Observer` to retry init.
	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
		return nil

	// `*_REPO_NAMING_*` is only relevant when listing untracked
	// repos.  No need to propagate them to existing repos.
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
		return nil

	// `RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED` is
	// replicated from the repo.
	case pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return nil

	// Ignore splitrootwf:
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_PATH_FLAG_SET:
		return nil
	case pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET:
		return nil

	default: // Ignore unknown.
		p.lg.Errorw(
			"Ignored unknown registry event.",
			"module", "repoinit",
			"event", regEv.Event,
		)
		return nil
	}
}

func (p *Processor) initRepo(regId uuid.I, inf *pb.FsoRepoInfo) error {
	regName := p.registryNames[regId]

	repoId, err := uuid.FromBytes(inf.Id)
	if err != nil {
		return err
	}

	root := findRepoRoot(p.roots, inf.GlobalPath)
	if root == nil {
		err := fmt.Errorf("unknown root")
		return err
	}

	hostPath := path.Join(
		root.HostRoot,
		strings.TrimPrefix(inf.GlobalPath, root.GlobalRoot),
	)

	gitHost, gitPath, err := gitLocation(root.GitlabNamespace, repoId)
	if err != nil {
		return err
	}

	subdirTracking := p.whichSubdirTracking(
		root.InitPolicy, inf.GlobalPath,
	)

	vid, err := p.repos.Init(repoId, &fsorepos.CmdInitRepo{
		Registry:               regName,
		GlobalPath:             inf.GlobalPath,
		CreatorName:            inf.CreatorName,
		CreatorEmail:           inf.CreatorEmail,
		FileHost:               root.Host,
		HostPath:               hostPath,
		GitlabHost:             gitHost,
		GitlabPath:             gitPath,
		SubdirTracking:         subdirTracking,
		ArchiveRecipients:      root.ArchiveRecipients,
		ShadowBackupRecipients: root.ShadowBackupRecipients,
	})
	if err != nil {
		return err
	}

	// Do not `registry.ConfirmRepo()` here.  `../replicate/replicate.go`
	// handles it.

	p.lg.Infow(
		"Initialized repo.",
		"module", "repoinit",
		"repoGlobalPath", inf.GlobalPath,
		"repoId", repoId.String(),
		"repoVid", vid.String(),
	)

	return nil
}

func (p *Processor) beginMoveRepo(regId uuid.I, inf *moveRepoInfo) error {
	registryEventId := inf.RegistryEventId
	repoId := inf.RepoId
	workflowId := inf.WorkflowId
	gpath := inf.NewGlobalPath

	root := findRepoRoot(p.roots, gpath)
	if root == nil {
		err := fmt.Errorf("unknown root")
		return err
	}

	hostPath := path.Join(
		root.HostRoot,
		strings.TrimPrefix(gpath, root.GlobalRoot),
	)
	cmd := &fsorepos.CmdBeginMoveRepo{
		RegistryEventId: registryEventId,
		WorkflowId:      workflowId,
		NewGlobalPath:   gpath,
		NewFileHost:     root.Host,
		NewHostPath:     hostPath,
	}
	vid, err := p.repos.BeginMoveRepo(repoId, fsorepos.NoVC, cmd)
	// Ignore:
	//
	// - ErrWorkflowActive: the workflow has started and is still active.
	// - ErrWorkflowReuse: the workflow has completed.
	//
	switch {
	case err == fsorepos.ErrWorkflowActive:
		return nil
	case err == fsorepos.ErrWorkflowReuse:
		return nil
	case err != nil:
		return err
	}

	p.lg.Infow(
		"Started move repo.",
		"module", "repoinit",
		"repoGlobalPath", gpath,
		"repoId", repoId.String(),
		"repoVid", vid.String(),
	)
	return nil
}

func (p *Processor) whichSubdirTracking(
	policy *pb.FsoRepoInitPolicy, globalPath string,
) fsorepos.SubdirTracking {
	tracking, err := fsoregistry.WhichSubdirTracking(policy, globalPath)
	if err != nil {
		p.lg.Warnw(
			"Failed to determine subdir tracking; "+
				"using BundleSubdirs.",
			"err", err,
		)
		return fsorepos.BundleSubdirs
	}
	switch tracking {
	case pb.SubdirTracking_ST_ENTER_SUBDIRS:
		return fsorepos.EnterSubdirs
	case pb.SubdirTracking_ST_BUNDLE_SUBDIRS:
		return fsorepos.BundleSubdirs
	case pb.SubdirTracking_ST_IGNORE_SUBDIRS:
		return fsorepos.IgnoreSubdirs
	case pb.SubdirTracking_ST_IGNORE_MOST:
		return fsorepos.IgnoreMost
	default:
		panic("invalid SubdirTracking")
	}
}

func (p *Processor) enableGitlab(repoId uuid.I, gitlabNamespace string) error {
	vid, err := p.repos.EnableGitlab(
		repoId, fsorepos.NoVC, gitlabNamespace,
	)
	switch {
	case err == fsorepos.ErrGitlabPathConflict:
		p.lg.Errorw(
			"Ignored GitLab path conflict.",
			"module", "repoinit",
			"repoId", repoId.String(),
		)
		return nil
	case err != nil:
		return err
	}

	p.lg.Infow(
		"Enabled GitLab",
		"module", "repoinit",
		"repoId", repoId.String(),
		"repoVid", vid.String(),
	)
	return nil
}

func (p *Processor) forEachRegistryEventAfter(
	regId uuid.I, after ulid.I,
	fn func(ulid.I, *pb.RegistryEvent) error,
) error {
	iter := p.registryJ.Find(regId, after)
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var ev fsoregistry.Event
	for iter.Next(&ev) {
		if err := fn(ev.Id(), ev.PbRegistryEvent()); err != nil {
			return err
		}
	}
	return iterClose()
}

func (p *Processor) retryUntilCancel(
	ctx context.Context, what string, fn func() error,
) error {
	for {
		err := fn()
		if err == nil {
			return nil
		}
		wait := 20 * time.Second
		p.lg.Errorw(
			fmt.Sprintf("Will retry %s.", what),
			"module", "repoinit",
			"err", err,
			"retryIn", wait,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func findRepoRoot(
	roots map[string]*RootInfo, path string,
) *RootInfo {
	for _, inf := range roots {
		if pathIsEqualOrBelowPrefix(path, inf.GlobalRoot) {
			return inf
		}
	}
	return nil
}

func pathIsEqualOrBelowPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	// Equal or slash right after prefix.
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

func gitLocation(namespace string, id uuid.I) (host, path string, err error) {
	// An empty root Gitlab `namespace` indicates that nogfsostad manages
	// the repo only locally.  Indicate it as empty `host` and `path` on
	// the repo.
	if namespace == "" {
		return "", "", nil
	}

	parts := strings.SplitN(namespace, "/", 2)
	if len(parts) != 2 {
		err := fmt.Errorf("invalid Gitlab namespace")
		return "", "", err
	}
	host = parts[0]
	nsPath := parts[1]
	path = fmt.Sprintf("%s/%s", nsPath, id.String())
	return host, path, nil
}
