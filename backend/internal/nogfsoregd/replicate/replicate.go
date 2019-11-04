// `replicate.Processor` watches event journals and replicates selected events
// to other journals.
package replicate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsomain"
	mainpb "github.com/nogproject/nog/backend/internal/fsomainpb"
	"github.com/nogproject/nog/backend/internal/fsoregistry"
	registryev "github.com/nogproject/nog/backend/internal/fsoregistry/pbevents"
	"github.com/nogproject/nog/backend/internal/fsorepos"
	reposev "github.com/nogproject/nog/backend/internal/fsorepos/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	workflowsev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/internal/workflows/moverepowf"
	"github.com/nogproject/nog/backend/internal/workflows/moveshadowwf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const (
	NsFsoRegistry = "fsoreg"
)

type Op int

const (
	OpUnspecified Op = iota
	OpInit
	OpUpdate
)

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type Processor struct {
	lg Logger

	names *shorteruuid.Names

	mainJ      *events.Journal
	registryJ  *events.Journal
	reposJ     *events.Journal
	workflowsJ *events.Journal

	registry            *fsoregistry.Registry
	repos               *fsorepos.Repos
	moveRepoWorkflows   *moverepowf.Workflows
	moveShadowWorkflows *moveshadowwf.Workflows

	mainId uuid.I

	mainTail      ulid.I
	registryTails map[uuid.I]ulid.I
	repoTails     map[uuid.I]ulid.I
	workflowTails map[uuid.I]ulid.I

	registryNames  map[uuid.I]string
	repoRegistries map[uuid.I]uuid.I
}

func NewProcessor(
	lg Logger,
	names *shorteruuid.Names,
	mainJ *events.Journal,
	registryJ *events.Journal,
	reposJ *events.Journal,
	workflowsJ *events.Journal,
	registry *fsoregistry.Registry,
	repos *fsorepos.Repos,
	moveRepoWorkflows *moverepowf.Workflows,
	moveShadowWorkflows *moveshadowwf.Workflows,
	mainId uuid.I,
) *Processor {
	return &Processor{
		lg:                  lg,
		names:               names,
		mainJ:               mainJ,
		registryJ:           registryJ,
		reposJ:              reposJ,
		workflowsJ:          workflowsJ,
		registry:            registry,
		repos:               repos,
		moveRepoWorkflows:   moveRepoWorkflows,
		moveShadowWorkflows: moveShadowWorkflows,
		mainId:              mainId,
		mainTail:            events.EventEpoch,
		registryTails:       make(map[uuid.I]ulid.I),
		repoTails:           make(map[uuid.I]ulid.I),
		workflowTails:       make(map[uuid.I]ulid.I),
		registryNames:       make(map[uuid.I]string),
		repoRegistries:      make(map[uuid.I]uuid.I),
	}
}

func (p *Processor) Process(ctx context.Context) error {
	// First subscribe, then init, so that no events are lost.

	// Watch mainId on main journal.
	mainUpdates := make(chan uuid.I, 100)
	p.mainJ.Subscribe(mainUpdates, p.mainId)
	defer p.mainJ.Unsubscribe(mainUpdates)

	// Watch all events on the registry, repos, and workflows journals.
	regUpdates := make(chan uuid.I, 100)
	p.registryJ.Subscribe(regUpdates, events.WildcardTopic)
	defer p.registryJ.Unsubscribe(regUpdates)

	repoUpdates := make(chan uuid.I, 100)
	p.reposJ.Subscribe(repoUpdates, events.WildcardTopic)
	defer p.reposJ.Unsubscribe(repoUpdates)

	workflowUpdates := make(chan uuid.I, 100)
	p.workflowsJ.Subscribe(workflowUpdates, events.WildcardTopic)
	defer p.workflowsJ.Unsubscribe(workflowUpdates)

	if err := p.initRetry(ctx); err != nil {
		return err
	}

	// A send to `triggerScan` starts the next background scan.
	triggerScan := make(chan struct{}, 1)
	triggerScan <- struct{}{}

	var wg sync.WaitGroup

Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop

		case <-mainUpdates:
			if err := p.retryUntilCancel(ctx, "update main", func() error {
				return p.updateMain(ctx)
			}); err != nil {
				break Loop
			}

		case regId := <-regUpdates:
			if _, ok := p.registryTails[regId]; !ok {
				// Ignore unknown registries.  Registries are
				// activated via a main event.
				continue
			}
			if err := p.retryUntilCancel(ctx, "update registry", func() error {
				return p.updateRegistry(ctx, regId)
			}); err != nil {
				break Loop
			}

		case repoId := <-repoUpdates:
			if _, ok := p.repoTails[repoId]; !ok {
				// Ignore unknown repos.  Repos are activated
				// via a registry event.
				continue
			}
			if err := p.retryUntilCancel(ctx, "update repo", func() error {
				return p.updateRepo(ctx, repoId)
			}); err != nil {
				break Loop
			}

		case workflowId := <-workflowUpdates:
			if _, ok := p.workflowTails[workflowId]; !ok {
				// Ignore unknown workflows.  Workflows must be
				// initiated from another entity.
				continue
			}
			if err := p.retryUntilCancel(ctx, "update workflow", func() error {
				return p.updateWorkflow(ctx, workflowId)
			}); err != nil {
				break Loop
			}

		case <-triggerScan:
			// Copy the relevant state in the current goroutine and
			// pass it to the new `scan()` goroutine in order to
			// avoid a mutex.
			mainId := p.mainId
			regIds := make([]uuid.I, 0, len(p.registryTails))
			for k, _ := range p.registryTails {
				regIds = append(regIds, k)
			}
			repoIds := make([]uuid.I, 0, len(p.repoTails))
			for k, _ := range p.repoTails {
				repoIds = append(repoIds, k)
			}
			workflowIds := make([]uuid.I, 0, len(p.workflowTails))
			for k, _ := range p.workflowTails {
				workflowIds = append(workflowIds, k)
			}

			p.lg.Infow(
				"Started background scan.",
				"module", "replicate",
			)
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = scan(
					ctx,
					mainUpdates, mainId,
					regUpdates, regIds,
					repoUpdates, repoIds,
					workflowUpdates, workflowIds,
					triggerScan,
				)
			}()
		}
	}

	wg.Wait()
	return ctx.Err()
}

func scan(
	ctx context.Context,
	mainUpdates chan<- uuid.I, mainId uuid.I,
	regUpdates chan<- uuid.I, regIds []uuid.I,
	repoUpdates chan<- uuid.I, repoIds []uuid.I,
	workflowUpdates chan<- uuid.I, workflowIds []uuid.I,
	triggerScan chan<- struct{},
) error {
	// A tick every 500 ms means approximately 150k ticks per day, which
	// should be sufficient for several 10k repos, assuming a background
	// recheck once per day is enough.  Background rechecking is only a
	// fallback, needed if an event got dropped due to a full subscribe
	// channel.  If this happens, we may need to reconsider how to improve
	// throughput anyway.
	//
	// Sleep longer between scans to avoid spaming the log with frequent
	// scans if there are few repos.
	tick := 500 * time.Millisecond
	nextScanSleep := 1 * time.Hour

	sleep := func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(tick):
		}
		return nil
	}

	if err := sleep(); err != nil {
		return err
	}
	select {
	default: // non-blocking
	case <-ctx.Done():
		return ctx.Err()
	case mainUpdates <- mainId:
	}

	for _, regId := range regIds {
		if err := sleep(); err != nil {
			return err
		}
		select {
		default: // non-blocking
		case <-ctx.Done():
			return ctx.Err()
		case regUpdates <- regId:
		}
	}

	for _, repoId := range repoIds {
		if err := sleep(); err != nil {
			return err
		}
		select {
		default: // non-blocking
		case <-ctx.Done():
			return ctx.Err()
		case repoUpdates <- repoId:
		}
	}

	for _, workflowId := range workflowIds {
		if err := sleep(); err != nil {
			return err
		}
		select {
		default: // non-blocking
		case <-ctx.Done():
			return ctx.Err()
		case workflowUpdates <- workflowId:
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(nextScanSleep):
		triggerScan <- struct{}{}
		return nil
	}
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
			"module", "replicate",
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

func (p *Processor) initRetry(ctx context.Context) error {
	err := p.retryUntilCancel(ctx, "init", func() error {
		return p.init(ctx)
	})
	return err
}

func (p *Processor) init(ctx context.Context) error {
	if err := p.initMain(ctx); err != nil {
		return err
	}
	if err := p.initRegistries(ctx); err != nil {
		return err
	}
	if err := p.initRepos(ctx); err != nil {
		return err
	}
	if err := p.initWorkflows(ctx); err != nil {
		return err
	}
	return nil
}

func (p *Processor) initMain(ctx context.Context) error {
	return p.pollMain(OpInit, ctx)
}

func (p *Processor) updateMain(ctx context.Context) error {
	return p.pollMain(OpUpdate, ctx)
}

func (p *Processor) pollMain(op Op, ctx context.Context) error {
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
			// Not interested in accept.  Wait for confirmed.

		case mainpb.Event_EV_FSO_REGISTRY_CONFIRMED:
			name := mainEv.FsoRegistryName
			regId := p.names.UUID(NsFsoRegistry, name)
			p.registryTails[regId] = events.EventEpoch
			p.registryNames[regId] = name
			if op == OpUpdate {
				err := p.updateRegistry(ctx, regId)
				if err != nil {
					return err
				}
			}

		default:
			p.lg.Warnw(
				"Ignored unknown main event.",
				"module", "replicate",
				"event", mainEv.Event,
			)
		}
		p.mainTail = ev.Id()
	}
	return iterClose()
}

func (p *Processor) initRegistries(ctx context.Context) error {
	for id, _ := range p.registryTails {
		if err := p.initRegistry(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) initRegistry(ctx context.Context, regId uuid.I) error {
	return p.pollRegistry(OpInit, ctx, regId)
}

func (p *Processor) updateRegistry(ctx context.Context, regId uuid.I) error {
	return p.pollRegistry(OpUpdate, ctx, regId)
}

func (p *Processor) pollRegistry(
	op Op, ctx context.Context, regId uuid.I,
) error {
	iter := p.registryJ.Find(regId, p.registryTails[regId])
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
		if err := p.applyRegistryEvent(
			op, ctx, regId, ev,
		); err != nil {
			return err
		}
		p.registryTails[regId] = ev.Id()
	}
	return iterClose()
}

func (p *Processor) applyRegistryEvent(
	op Op, ctx context.Context, regId uuid.I, ev fsoregistry.Event,
) error {
	pbev := ev.PbRegistryEvent()

	switch pbev.Event {
	// Process only the following:
	//
	//  - events that add a repo: initialize the repo state;
	//  - events that have been replicated from repo events: set
	//    `repoTails` during init.
	//
	case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
		return nil
	case pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
		return nil
	case pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED:
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
		// Enable repo processing.
		repoId, err := uuid.FromBytes(pbev.FsoRepoInfo.Id)
		if err != nil {
			return err
		}
		p.repoTails[repoId] = events.EventEpoch
		p.repoRegistries[repoId] = regId
		if op == OpUpdate {
			err := p.updateRepo(ctx, repoId)
			if err != nil {
				return err
			}
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		// During init, set `repoTails` to start processing
		// repo events after the original event.
		if op != OpInit {
			return nil
		}

		// Legacy events lack a ref to the repo event.
		// `repoTails`, therefore, cannot be updated.  But
		// `RepoEvent_EV_FSO_REPO_INIT_STARTED` will double
		// check the registry state before replicating the
		// event.
		if pbev.RepoEventId == nil {
			return nil
		}

		// For modern events, set `repoTails`, so that
		// `initRepo()` starts processing after the original
		// event.
		repoId, err := uuid.FromBytes(pbev.FsoRepoInfo.Id)
		if err != nil {
			return err
		}
		repoEventId, err := ulid.ParseBytes(pbev.RepoEventId)
		if err != nil {
			return err
		}
		p.repoTails[repoId] = repoEventId
		return nil

	default:
		// continue with next switch.
	}

	// moverepowf
	regEv, err := registryev.FromPbValidate(*pbev)
	if err != nil {
		p.lg.Errorw(
			"Ignored decode registry event error.",
			"err", err,
		)
	}
	switch x := regEv.(type) {
	// `RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED`.
	case *registryev.EvRepoMoveAccepted:
		// Handled by repoinit.
		return nil

	// `RegistryEvent_EV_FSO_REPO_MOVED`.
	case *registryev.EvRepoMoved:
		repoId := x.RepoId
		repoEventId := x.RepoEventId
		workflowId := x.WorkflowId

		needsExit := true
		if op == OpInit {
			workflow, err := p.moveRepoWorkflows.FindId(
				workflowId,
			)
			if err != nil {
				return err
			}
			needsExit = !workflow.IsTerminated()
		}
		if needsExit {
			workflowVid, err := p.moveRepoWorkflows.Exit(
				workflowId, moverepowf.NoVC,
			)
			if err != nil {
				return err
			}
			p.lg.Infow(
				"Terminated move-repo workflow.",
				"module", "replicate",
				"workflowId", workflowId.String(),
				"workflowVid", workflowVid.String(),
			)
		}

		// Tell `initRepo()` to process only later events.
		p.repoTails[repoId] = repoEventId
		return nil

	default:
		// continue with next switch.
	}

	// moveshadowwf
	switch pbev.Event {
	case pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		// During init, set `repoTails` to start processing
		// repo events after the original event.
		if op != OpInit {
			return nil
		}

		repoId, err := uuid.FromBytes(pbev.RepoId)
		if err != nil {
			return err
		}
		repoEventId, err := ulid.ParseBytes(pbev.RepoEventId)
		if err != nil {
			return err
		}
		workflowId, err := uuid.FromBytes(pbev.WorkflowId)
		if err != nil {
			return err
		}

		p.repoTails[repoId] = repoEventId
		// Enable event processing for the workflow.
		p.workflowTails[workflowId] = events.EventEpoch
		return nil

	default:
		// continue with next switch.
	}

	// Ignore splitrootwf:
	switch pbev.Event {
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
	default:
		// continue with next switch.
	}

	// Ignore freezerepowf.
	switch pbev.Event {
	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return nil
	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unfreezerepowf.
	switch pbev.Event {
	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return nil
	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore archiverepowf.
	switch pbev.Event {
	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unarchiverepowf.
	switch pbev.Event {
	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return nil
	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	p.lg.Errorw(
		"Ignored unknown registry event.",
		"module", "replicate",
		"event", pbev.Event,
	)
	return nil
}

func (p *Processor) initRepos(ctx context.Context) error {
	for id, _ := range p.repoTails {
		if err := p.initRepo(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) initRepo(ctx context.Context, repoId uuid.I) error {
	// Initialize workflows from a view that is based on the full event
	// history.
	repo, err := p.loadRepo(ctx, repoId)
	if err != nil {
		return err
	}
	if repo.initMoveRepoWorkflow != nil {
		err := p.initMoveRepoWorkflow(repo.initMoveRepoWorkflow)
		if err != nil {
			return err
		}
	}

	// Then poll after the latest event that has already been replicated to
	// the registry.
	return p.pollRepo(OpInit, ctx, repoId)
}

type repoView struct {
	id                   uuid.I
	vid                  ulid.I
	initMoveRepoWorkflow *cmdInitMoveRepoWorkflow
}

// `loadRepo()` loads all repo events to construct and return the current state
// without creating side effects.
func (p *Processor) loadRepo(
	ctx context.Context, repoId uuid.I,
) (*repoView, error) {
	iter := p.reposJ.Find(repoId, events.EventEpoch)
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	repo := repoView{id: repoId}
	var ev fsorepos.Event
	for iter.Next(&ev) {
		if err := repo.applyEvent(ctx, p, ev); err != nil {
			return nil, err
		}
		repo.vid = ev.Id()
	}
	return &repo, iterClose()
}

func (repo *repoView) applyEvent(
	ctx context.Context, p *Processor, ev fsorepos.Event,
) error {
	repoEv := ev.PbRepoEvent()

	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_GIT_TO_NOG_CLONED:
		return nil
	case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
		return nil
	case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
		return nil
	case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
		return nil
	case pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		return nil
	default:
		// continue with next switch.
	}

	switch x := reposev.FromPbMust(*repoEv).(type) {
	case *reposev.EvRepoMoveStarted: // `RepoEvent_EV_FSO_REPO_MOVE_STARTED`
		repo.initMoveRepoWorkflow = &cmdInitMoveRepoWorkflow{
			WorkflowId:    x.WorkflowId,
			RepoId:        repo.id,
			RepoVid:       ev.Id(),
			OldGlobalPath: x.OldGlobalPath,
			OldFileHost:   x.OldFileHost,
			OldHostPath:   x.OldHostPath,
			OldShadowPath: x.OldShadowPath,
			NewGlobalPath: x.NewGlobalPath,
			NewFileHost:   x.NewFileHost,
			NewHostPath:   x.NewHostPath,
		}
		return nil

	case *reposev.EvRepoMoved: // `RepoEvent_EV_FSO_REPO_MOVED`
		repo.initMoveRepoWorkflow = nil
		return nil

	default:
		// continue with next switch.
	}

	// Ignore freezerepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unfreezerepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore archiverepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unarchiverepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	p.lg.Errorw(
		"Ignored unknown repo event.",
		"module", "replicate",
		"event", repoEv.Event,
	)
	return nil
}

func (p *Processor) updateRepo(ctx context.Context, repoId uuid.I) error {
	return p.pollRepo(OpUpdate, ctx, repoId)
}

func (p *Processor) pollRepo(
	op Op, ctx context.Context, repoId uuid.I,
) error {
	iter := p.reposJ.Find(repoId, p.repoTails[repoId])
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var ev fsorepos.Event
	for iter.Next(&ev) {
		if err := p.applyRepoEvent(
			op, ctx, repoId, ev,
		); err != nil {
			return err
		}
		p.repoTails[repoId] = ev.Id()
	}
	return iterClose()
}

func (p *Processor) applyRepoEvent(
	op Op, ctx context.Context, repoId uuid.I, ev fsorepos.Event,
) error {
	repoEv := ev.PbRepoEvent()

	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
		regId, ok := p.repoRegistries[repoId]
		if !ok {
			panic("missing repo registry")
		}

		// During init, avoid re-replication of legacy events.
		// For legacy events that have no `RepoEventId`,
		// `RegistryEvent_EV_FSO_REPO_ADDED` could not update
		// `repoTails`.  The repo event, therefore, is
		// processed again.
		//
		// During update, the event must be replicated, because
		// it is the cause of the corresponding registry event.
		needsConfirm := true
		if op == OpInit {
			registry, err := p.registry.FindId(regId)
			if err != nil {
				return err
			}
			repo, ok := registry.RepoById(repoId)
			needsConfirm = (ok && !repo.Confirmed)
		}
		if needsConfirm {
			repoVid := ev.Id()
			regVid, err := p.registry.ConfirmRepo(
				regId, fsoregistry.NoVC,
				repoId, repoVid,
			)
			if err != nil {
				return err
			}
			p.lg.Infow(
				"Replicated repo event.",
				"module", "replicate",
				"fromRepoEvent", repoEv.Event.String(),
				"repoId", repoId.String(),
				"repoVid", repoVid.String(),
				"toRegistryEvent", "EV_FSO_REPO_ADDED",
				"registryVid", regVid.String(),
			)
		}
		return nil

	case pb.RepoEvent_EV_FSO_REPO_MOVE_STARTED:
		if op == OpInit {
			// See `loadRepo()` for event handling during init.
			return nil
		}
		x := reposev.FromPbMust(*repoEv).(*reposev.EvRepoMoveStarted)
		return p.initMoveRepoWorkflow(&cmdInitMoveRepoWorkflow{
			WorkflowId:    x.WorkflowId,
			RepoId:        repoId,
			RepoVid:       ev.Id(),
			OldGlobalPath: x.OldGlobalPath,
			OldFileHost:   x.OldFileHost,
			OldHostPath:   x.OldHostPath,
			OldShadowPath: x.OldShadowPath,
			NewGlobalPath: x.NewGlobalPath,
			NewFileHost:   x.NewFileHost,
			NewHostPath:   x.NewHostPath,
		})

	case pb.RepoEvent_EV_FSO_REPO_MOVED:
		x := reposev.FromPbMust(*repoEv).(*reposev.EvRepoMoved)
		regId, ok := p.repoRegistries[repoId]
		if !ok {
			panic("missing repo registry")
		}
		repoVid := ev.Id()
		cmd := &fsoregistry.CmdCommitMoveRepo{
			RepoId:      repoId,
			WorkflowId:  x.WorkflowId,
			RepoEventId: repoVid,
			GlobalPath:  x.GlobalPath,
		}
		regVid, err := p.registry.CommitMoveRepo(
			regId, fsoregistry.NoVC, cmd,
		)
		if err != nil {
			return err
		}
		p.lg.Infow(
			"Replicated event.",
			"module", "replicate",
			"fromRepoEvent", "EV_FSO_REPO_MOVED",
			"repoId", repoId.String(),
			"repoVid", repoVid.String(),
			"toRegistryEvent", "EV_FSO_REPO_MOVED",
			"registryVid", regVid.String(),
		)
		return nil

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		regId, ok := p.repoRegistries[repoId]
		if !ok {
			panic("missing repo registry")
		}
		repoVid := ev.Id()
		workflowId, err := uuid.FromBytes(repoEv.WorkflowId)
		if err != nil {
			return err
		}

		// The order is crucial: first init the workflow, then
		// replicate to the registry.  The registry event is
		// the indicator that the repo event has been fully
		// replicated.
		workflowVid, err := p.moveShadowWorkflows.Init(
			workflowId, repoId, repoVid,
		)
		if err != nil {
			return err
		}
		p.lg.Infow(
			"Replicated repo event.",
			"module", "replicate",
			"fromRepoEvent", repoEv.Event.String(),
			"repoId", repoId.String(),
			"repoVid", repoVid.String(),
			"toWorkflowEvent", "EV_FSO_SHADOW_REPO_MOVE_STARTED",
			"workflowVid", workflowVid.String(),
		)

		// Enable event processing for the workflow.
		p.workflowTails[workflowId] = events.EventEpoch

		regVid, err := p.registry.PostShadowRepoMoveStarted(
			regId, fsoregistry.NoVC,
			repoId, repoVid, workflowId,
		)
		if err != nil {
			return err
		}
		p.lg.Infow(
			"Replicated event.",
			"module", "replicate",
			"fromRepoEvent", repoEv.Event.String(),
			"repoId", repoId.String(),
			"repoVid", repoVid.String(),
			"toRegistryEvent", "EV_FSO_SHADOW_REPO_MOVE_STARTED",
			"registryVid", regVid.String(),
		)
		return nil

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED:
		// Do not disable the workflow here.  Let it disable itself.
		return nil

	// Ignore events that need not be replicated.
	case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_GIT_TO_NOG_CLONED:
		return nil
	case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
		return nil
	case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
		return nil
	case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
		return nil
	case pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED:
		return nil
	case pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
		return nil
	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		return nil

	default:
		// continue with next switch.
	}

	// Ignore freezerepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unfreezerepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore archiverepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	// Ignore unarchiverepowf.
	switch repoEv.Event {
	case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return nil
	default:
		// continue with next switch.
	}

	p.lg.Errorw(
		"Ignored unknown repo event.",
		"module", "replicate",
		"event", repoEv.Event,
	)
	return nil
}

type cmdInitMoveRepoWorkflow struct {
	WorkflowId    uuid.I
	RepoId        uuid.I
	RepoVid       ulid.I
	OldGlobalPath string
	OldFileHost   string
	OldHostPath   string
	OldShadowPath string
	NewGlobalPath string
	NewFileHost   string
	NewHostPath   string
}

func (p *Processor) initMoveRepoWorkflow(cmd *cmdInitMoveRepoWorkflow) error {
	// Enable event processing for the workflow.
	p.workflowTails[cmd.WorkflowId] = events.EventEpoch

	workflowVid, err := p.moveRepoWorkflows.Init(
		cmd.WorkflowId,
		&moverepowf.CmdInit{
			RepoId:        cmd.RepoId,
			RepoEventId:   cmd.RepoVid,
			OldGlobalPath: cmd.OldGlobalPath,
			OldFileHost:   cmd.OldFileHost,
			OldHostPath:   cmd.OldHostPath,
			OldShadowPath: cmd.OldShadowPath,
			NewGlobalPath: cmd.NewGlobalPath,
			NewFileHost:   cmd.NewFileHost,
			NewHostPath:   cmd.NewHostPath,
		},
	)
	// Ignore if the workflow has advanced beyond init.
	if err == moverepowf.ErrConflictStateAdvanced {
		return nil
	}
	if err != nil {
		return err
	}
	p.lg.Infow(
		"Replicated repo event.",
		"module", "replicate",
		"fromRepoEvent", "EV_FSO_REPO_MOVE_STARTED",
		"repoId", cmd.RepoId.String(),
		"repoVid", cmd.RepoVid.String(),
		"toWorkflowEvent", "EV_FSO_REPO_MOVE_STARTED",
		"workflowVid", workflowVid.String(),
	)
	return nil
}

func (p *Processor) initWorkflows(ctx context.Context) error {
	for id, _ := range p.workflowTails {
		if err := p.initWorkflow(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) initWorkflow(ctx context.Context, workflowId uuid.I) error {
	return p.pollWorkflow(OpInit, ctx, workflowId)
}

func (p *Processor) updateWorkflow(ctx context.Context, workflowId uuid.I) error {
	return p.pollWorkflow(OpUpdate, ctx, workflowId)
}

func (p *Processor) pollWorkflow(op Op, ctx context.Context, workflowId uuid.I) error {
	iter := p.workflowsJ.Find(workflowId, p.workflowTails[workflowId])
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	var ev workflowsev.Event
	for iter.Next(&ev) {
		if err := p.applyWorkflowEvent(
			op, ctx, workflowId, ev,
		); err != nil {
			return err
		}
		p.workflowTails[workflowId] = ev.Id()
	}
	return iterClose()
}

func (p *Processor) applyWorkflowEvent(
	op Op, ctx context.Context, workflowId uuid.I, ev workflowsev.Event,
) error {
	workflowEv := ev.PbWorkflowEvent()

	// Workflows that need not be handled, because they are not polled:
	//
	//  - pingregistrywf
	//  - splitrootwf
	//

	// moverepowf
	switch x := workflowsev.MustParsePbWorkflowEvent(workflowEv).(type) {
	case *workflowsev.EvRepoMoveStarted: // `WorkflowEvent_EV_FSO_REPO_MOVE_STARTED`
		return nil

	case *workflowsev.EvRepoMoveStaReleased: // `WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED`
		return nil

	case *workflowsev.EvRepoMoveAppAccepted: // `WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED`
		return nil

	case *workflowsev.EvRepoMoved: // `WorkflowEvent_EV_FSO_REPO_MOVED`
		// During init, check whether the event has already been
		// applied.  If so, skip commit.
		//
		// During update, the event must always be applied, because it
		// is the cause of the corresponding repo event.
		needsCommit := true
		if op == OpInit {
			repoId := x.RepoId
			repo, err := p.repos.FindId(repoId)
			if err != nil {
				return err
			}
			needsCommit = (repo.HasActiveMoveRepo() &&
				repo.MoveRepoId() == workflowId)
		}
		if needsCommit {
			repoId := x.RepoId
			workflowEventId := ev.Id()
			cmd := &fsorepos.CmdCommitMoveRepo{
				WorkflowId:      workflowId,
				WorkflowEventId: workflowEventId,
				GlobalPath:      x.GlobalPath,
				FileHost:        x.FileHost,
				HostPath:        x.HostPath,
				ShadowPath:      x.ShadowPath,
			}
			repoVid, err := p.repos.CommitMoveRepo(
				repoId, fsorepos.NoVC, cmd,
			)
			if err != nil {
				return err
			}
			p.lg.Infow(
				"Replicated move-repo workflow event.",
				"module", "replicate",
				"fromWorkflowEvent", "EV_FSO_REPO_MOVED",
				"workflowId", workflowId.String(),
				"workflowVid", workflowEventId.String(),
				"toRepoEvent", "EV_FSO_REPO_MOVED",
				"repoId", repoId.String(),
				"repoVid", repoVid.String(),
			)
		}

		// Do not exit the workflow here.  See
		// `RepoEvent_EV_FSO_REPO_MOVED` instead.
		return nil

	case *workflowsev.EvRepoMoveCommitted: // `WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED`
		// Disable event processing when the workflow has terminated.
		delete(p.workflowTails, workflowId)
		return nil

	default:
		// continue with next switch.
	}

	// moveshadowwf
	switch workflowEv.Event {
	// `WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED` requires no
	// action.  It has been replicated from a corresponding
	// `RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED`.
	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return nil

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED:
		return nil

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED:
		repoId, err := uuid.FromBytes(workflowEv.RepoId)
		if err != nil {
			return err
		}

		// During init, check whether the event has already been
		// applied.  If so, skip commit.
		//
		// During update, the event must always be applied, because it
		// is the cause of the corresponding repo event.
		needsCommit := true
		if op == OpInit {
			repo, err := p.repos.FindId(repoId)
			if err != nil {
				return err
			}
			needsCommit = (repo.HasActiveMoveShadow() &&
				repo.MoveShadowId() == workflowId)
		}
		if needsCommit {
			workflowEventId := ev.Id()
			repoVid, err := p.repos.CommitMoveShadow(
				repoId, fsorepos.NoVC,
				workflowId, workflowEventId,
			)
			if err != nil {
				return err
			}
			p.lg.Infow(
				"Replicated move-shadow workflow event.",
				"module", "replicate",
				"fromWorkflowEvent", workflowEv.Event.String(),
				"workflowId", workflowId.String(),
				"workflowVid", workflowEventId.String(),
				"toRepoEvent", "EV_FSO_SHADOW_REPO_MOVED",
				"repoId", repoId.String(),
				"repoVid", repoVid.String(),
			)
		}

		needsExit := true
		if op == OpInit {
			workflow, err := p.moveShadowWorkflows.FindId(
				workflowId,
			)
			if err != nil {
				return err
			}
			needsExit = !workflow.IsTerminated()
		}
		if needsExit {
			workflowVid, err := p.moveShadowWorkflows.Exit(
				workflowId, moveshadowwf.NoVC,
			)
			if err != nil {
				return err
			}
			p.lg.Infow(
				"Terminated move-shadow workflow.",
				"module", "replicate",
				"workflowId", workflowId.String(),
				"workflowVid", workflowVid.String(),
			)
		}

		return nil

	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED:
		// The workflow has terminated.
		delete(p.workflowTails, workflowId)
		return nil

	default:
		// continue with next switch.
	}

	p.lg.Errorw(
		"Ignored unknown workflow event.",
		"module", "replicate",
		"event", workflowEv.Event,
	)
	return nil
}
