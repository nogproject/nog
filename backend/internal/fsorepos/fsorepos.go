// vim: sw=8

/*

Package `fsorepos` implements an event-sourced aggregate that contains FSO
repos.  See package `fsomain` for an oveview.

*/
package fsorepos

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/fsorepos/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/gpg"
	"github.com/nogproject/nog/backend/pkg/regexpx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const ConfigMaxStatusMessageLength = 150

// `NoVC` is a sentinel value that can be passed in place of `vid` to indicate
// that concurrency version checks are skipped.
var NoVC = events.NoVC

type State struct {
	id  uuid.I
	vid ulid.I

	registry   string
	globalPath string

	fileHost               string
	hostPath               string
	shadowPath             string
	archiveURL             string
	archiveRecipients      gpg.Fingerprints
	tarttTarPath           string
	shadowBackupURL        string
	shadowBackupRecipients gpg.Fingerprints

	gitlabHost      string
	gitlabPath      string
	gitlabProjectId int64

	gitToNogAddr      string
	gitToNogClonePath string

	errorMessage string

	// Move shadow workflow states:
	//
	//  - `moveShadowWorkflow == nil`: workflow never started.
	//  - `moveShadowWorkflow != nil && newShadowPath != ""`: workflow
	//    active.
	//  - `moveShadowWorkflow=<uuid> && newShadowPath == ""`: workflow
	//    has ended, currently no active workflow.
	//
	moveShadowWorkflow uuid.I
	newShadowPath      string

	// move-repo workflow states:
	//
	//  - `moveRepoWorkflow == nil`: workflow never started
	//  - `moveRepoWorkflow != nil && newGlobalPath != ""`: workflow
	//    active.
	//  - `moveRepoWorkflow != nil && newGlobalPath == ""`: workflow
	//    has ended, currently no active workflow.
	moveRepoWorkflow uuid.I
	newGlobalPath    string

	storageTier StorageTierCode

	// `storageWorkflowId` contains the ID of the last active workflow.  It
	// is used in idempotency checks.
	storageWorkflowId uuid.I
}

// XXX Maybe factor out into a common package, e.g. `storagetier`, that is used
// by packages `fsoregistry` and `fsorepos`.
type StorageTierCode int

const (
	StorageTierUnspecified StorageTierCode = iota
	StorageOnline
	StorageFrozen
	StorageArchived
	StorageFreezing
	StorageFreezeFailed
	StorageUnfreezing
	StorageUnfreezeFailed
	StorageArchiving
	StorageArchiveFailed
	StorageUnarchiving
	StorageUnarchiveFailed
)

type Event struct {
	id     ulid.I
	parent ulid.I
	pb     pb.RepoEvent
}

type SubdirTracking int

const (
	SubdirTrackingUnspecified SubdirTracking = iota
	EnterSubdirs
	BundleSubdirs
	IgnoreSubdirs
	IgnoreMost
)

type CmdInitRepo struct {
	Registry               string
	GlobalPath             string
	CreatorName            string
	CreatorEmail           string
	FileHost               string
	HostPath               string
	GitlabHost             string
	GitlabPath             string
	GitToNogAddr           string
	SubdirTracking         SubdirTracking
	ArchiveRecipients      gpg.Fingerprints
	ShadowBackupRecipients gpg.Fingerprints
}

type CmdConfirmShadow struct {
	ShadowPath string
}

type CmdBeginMoveRepo struct {
	RegistryEventId ulid.I
	WorkflowId      uuid.I
	NewGlobalPath   string
	NewFileHost     string
	NewHostPath     string
}

type CmdCommitMoveRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalPath      string
	FileHost        string
	HostPath        string
	ShadowPath      string
}

type CmdBeginMoveShadow struct {
	WorkflowId    uuid.I
	NewShadowPath string
}

type CmdCommitMoveShadow struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdInitTartt struct {
	TarttURL string
}

type CmdUpdateArchiveRecipients struct {
	Keys gpg.Fingerprints
}

type CmdDeleteArchiveRecipients struct{}

type CmdInitShadowBackup struct {
	ShadowBackupURL string
}

type CmdMoveShadowBackup struct {
	ShadowBackupURL string
}

type CmdUpdateShadowBackupRecipients struct {
	Keys gpg.Fingerprints
}

type CmdDeleteShadowBackupRecipients struct{}

type CmdConfirmGit struct {
	GitlabProjectId int64
}

type CmdEnableGitlab struct {
	GitlabNamespace string
}

type CmdBeginFreeze struct {
	WorkflowId uuid.I
}

type CmdCommitFreeze struct {
	WorkflowId uuid.I
}

type CmdAbortFreeze struct {
	WorkflowId    uuid.I
	StatusCode    int32
	StatusMessage string
}

type CmdBeginUnfreeze struct {
	WorkflowId uuid.I
}

type CmdCommitUnfreeze struct {
	WorkflowId uuid.I
}

type CmdAbortUnfreeze struct {
	WorkflowId    uuid.I
	StatusCode    int32
	StatusMessage string
}

type CmdBeginArchive struct {
	WorkflowId uuid.I
}

type CmdCommitArchive struct {
	WorkflowId uuid.I
	TarPath    string
}

type CmdAbortArchive struct {
	WorkflowId    uuid.I
	StatusCode    int32
	StatusMessage string
}

type CmdBeginUnarchive struct {
	WorkflowId uuid.I
}

type CmdCommitUnarchive struct {
	WorkflowId uuid.I
}

type CmdAbortUnarchive struct {
	WorkflowId    uuid.I
	StatusCode    int32
	StatusMessage string
}

type CmdSetRepoError struct {
	ErrorMessage string
}

type CmdClearRepoError struct {
	ErrorMessage string
}

// XXX Confirm GitToNog not yet implemented.

func (*State) AggregateState() {}

func (*CmdInitRepo) AggregateCommand()                     {}
func (*CmdConfirmShadow) AggregateCommand()                {}
func (*CmdBeginMoveRepo) AggregateCommand()                {}
func (*CmdCommitMoveRepo) AggregateCommand()               {}
func (*CmdBeginMoveShadow) AggregateCommand()              {}
func (*CmdCommitMoveShadow) AggregateCommand()             {}
func (*CmdInitTartt) AggregateCommand()                    {}
func (*CmdUpdateArchiveRecipients) AggregateCommand()      {}
func (*CmdDeleteArchiveRecipients) AggregateCommand()      {}
func (*CmdInitShadowBackup) AggregateCommand()             {}
func (*CmdMoveShadowBackup) AggregateCommand()             {}
func (*CmdUpdateShadowBackupRecipients) AggregateCommand() {}
func (*CmdDeleteShadowBackupRecipients) AggregateCommand() {}
func (*CmdConfirmGit) AggregateCommand()                   {}
func (*CmdEnableGitlab) AggregateCommand()                 {}
func (*CmdBeginFreeze) AggregateCommand()                  {}
func (*CmdCommitFreeze) AggregateCommand()                 {}
func (*CmdAbortFreeze) AggregateCommand()                  {}
func (*CmdBeginUnfreeze) AggregateCommand()                {}
func (*CmdCommitUnfreeze) AggregateCommand()               {}
func (*CmdAbortUnfreeze) AggregateCommand()                {}
func (*CmdBeginArchive) AggregateCommand()                 {}
func (*CmdCommitArchive) AggregateCommand()                {}
func (*CmdAbortArchive) AggregateCommand()                 {}
func (*CmdBeginUnarchive) AggregateCommand()               {}
func (*CmdCommitUnarchive) AggregateCommand()              {}
func (*CmdAbortUnarchive) AggregateCommand()               {}
func (*CmdSetRepoError) AggregateCommand()                 {}
func (*CmdClearRepoError) AggregateCommand()               {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

func newEvents(
	parent ulid.I, pbs ...pb.RepoEvent,
) ([]events.Event, error) {
	evs := make([]events.Event, 0, len(pbs))
	for _, pb := range pbs {
		id, err := ulid.New()
		if err != nil {
			return nil, err
		}
		e := &Event{id: id, parent: parent, pb: pb}
		e.pb.Id = e.id[:]
		e.pb.Parent = e.parent[:]
		evs = append(evs, e)
		parent = id
	}
	return evs, nil
}

func (e *Event) MarshalProto() ([]byte, error) {
	return proto.Marshal(&e.pb)
}

func (e *Event) UnmarshalProto(data []byte) error {
	var err error
	if err = proto.Unmarshal(data, &e.pb); err != nil {
		return err
	}
	if e.id, err = ulid.ParseBytes(e.pb.Id); err != nil {
		return err
	}
	if e.parent, err = ulid.ParseBytes(e.pb.Parent); err != nil {
		return err
	}
	if _, err := pbevents.FromPbValidate(e.pb); err != nil {
		return &EventDetailsError{Err: err}
	}
	return nil
}

func (e *Event) Id() ulid.I     { return e.id }
func (e *Event) Parent() ulid.I { return e.parent }

// Receiver by value.
func (e Event) WithId(id ulid.I) events.Event {
	e.id = id
	e.pb.Id = e.id[:]
	return &e
}

// Receiver by value.
func (e Event) WithParent(parent ulid.I) events.Event {
	e.parent = parent
	e.pb.Parent = e.parent[:]
	return &e
}

func (e *Event) PbRepoEvent() *pb.RepoEvent {
	return &e.pb
}

type Behavior struct{}

func (Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (Behavior) NewEvent() events.Event          { return &Event{} }
func (Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

type Advancer struct {
	main bool
}

func (a *Advancer) Advance(s events.State, ev events.Event) events.State {
	evpb := ev.(*Event).pb
	st := s.(*State)

	if !a.main {
		dup := *st
		st = &dup
		a.main = true
	}

	switch x := pbevents.FromPbMust(evpb).(type) {
	case *pbevents.EvRepoInitStarted:
		// Do not copy creator from event to state, since the creator
		// is only of temporary interest.
		st.registry = x.Registry
		st.globalPath = x.GlobalPath
		st.fileHost = x.FileHost
		st.hostPath = x.HostPath
		st.gitlabHost = x.GitlabHost
		st.gitlabPath = x.GitlabPath
		st.gitToNogAddr = x.GitToNogAddr

	case *pbevents.EvEnableGitlabAccepted:
		st.gitlabHost = x.GitlabHost
		st.gitlabPath = x.GitlabPath

	case *pbevents.EvShadowRepoCreated:
		st.shadowPath = x.ShadowPath
		st.storageTier = StorageOnline

	case *pbevents.EvRepoMoveStarted:
		st.moveRepoWorkflow = x.WorkflowId
		st.newGlobalPath = x.NewGlobalPath

	case *pbevents.EvRepoMoved:
		if x.GlobalPath != st.newGlobalPath {
			panic("new global path mismatch")
		}
		if x.FileHost != st.fileHost {
			panic("the file host cannot change")
		}
		st.globalPath = x.GlobalPath
		st.fileHost = x.FileHost
		st.hostPath = x.HostPath
		st.shadowPath = x.ShadowPath
		st.newGlobalPath = ""

	case *pbevents.EvShadowRepoMoveStarted:
		st.moveShadowWorkflow = x.WorkflowId
		st.newShadowPath = x.NewShadowPath

	case *pbevents.EvShadowRepoMoved:
		st.shadowPath = x.NewShadowPath
		st.newShadowPath = ""

	case *pbevents.EvTarttRepoCreated:
		st.archiveURL = x.ArchiveUrl

	case *pbevents.EvShadowBackupRepoCreated:
		st.shadowBackupURL = x.ShadowBackupUrl
	case *pbevents.EvShadowBackupRepoMoved:
		st.shadowBackupURL = x.ShadowBackupUrl

	case *pbevents.EvGitRepoCreated:
		st.gitlabProjectId = x.GitlabProjectId

	case *pbevents.EvRepoErrorSet:
		st.errorMessage = x.Message

	case *pbevents.EvRepoErrorCleared:
		st.errorMessage = ""

	case *pbevents.EvArchiveRecipientsUpdated:
		keys, err := gpg.ParseFingerprintsBytes(x.Keys...)
		if err != nil {
			panic(err)
		}
		st.archiveRecipients = keys

	case *pbevents.EvShadowBackupRecipientsUpdated:
		keys, err := gpg.ParseFingerprintsBytes(x.Keys...)
		if err != nil {
			panic(err)
		}
		st.shadowBackupRecipients = keys

	// Silently ignore legacy events that were used in preliminary
	// repo-freeze implementation.
	case *pbevents.EvFreezeRepoStarted:
	case *pbevents.EvFreezeRepoCompleted:

	case *pbevents.EvFreezeRepoStarted2:
		st.storageTier = StorageFreezing
		st.storageWorkflowId = x.WorkflowId

	case *pbevents.EvFreezeRepoCompleted2:
		if x.StatusCode == 0 {
			st.storageTier = StorageFrozen
		} else {
			st.storageTier = StorageFreezeFailed
		}

	// Silently ignore legacy events that were used in preliminary
	// repo-freeze implementation.
	case *pbevents.EvUnfreezeRepoStarted:
	case *pbevents.EvUnfreezeRepoCompleted:

	case *pbevents.EvUnfreezeRepoStarted2:
		st.storageTier = StorageUnfreezing
		st.storageWorkflowId = x.WorkflowId

	case *pbevents.EvUnfreezeRepoCompleted2:
		if x.StatusCode == 0 {
			st.storageTier = StorageOnline
		} else {
			st.storageTier = StorageUnfreezeFailed
		}

	case *pbevents.EvArchiveRepoStarted:
		st.storageTier = StorageArchiving
		st.storageWorkflowId = x.WorkflowId

	case *pbevents.EvArchiveRepoCompleted:
		if x.StatusCode == 0 {
			st.storageTier = StorageArchived
			st.tarttTarPath = x.TarPath
		} else {
			st.storageTier = StorageArchiveFailed
		}

	case *pbevents.EvUnarchiveRepoStarted:
		st.storageTier = StorageUnarchiving
		st.storageWorkflowId = x.WorkflowId

	case *pbevents.EvUnarchiveRepoCompleted:
		if x.StatusCode == 0 {
			st.storageTier = StorageFrozen
		} else {
			st.storageTier = StorageUnarchiveFailed
		}

	default:
		panic("invalid event")
	}

	return st
}

func (Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	state := s.(*State)
	switch cmd := c.(type) {
	case *CmdInitRepo:
		return tellInitRepo(state, cmd)
	case *CmdConfirmShadow:
		return tellConfirmShadow(state, cmd)
	case *CmdBeginMoveRepo:
		return tellBeginMoveRepo(state, cmd)
	case *CmdCommitMoveRepo:
		return tellCommitMoveRepo(state, cmd)
	case *CmdBeginMoveShadow:
		return tellBeginMoveShadow(state, cmd)
	case *CmdCommitMoveShadow:
		return tellCommitMoveShadow(state, cmd)
	case *CmdInitTartt:
		return tellInitTartt(state, cmd)
	case *CmdUpdateArchiveRecipients:
		return tellUpdateArchiveRecipients(state, cmd)
	case *CmdDeleteArchiveRecipients:
		return tellDeleteArchiveRecipients(state, cmd)
	case *CmdInitShadowBackup:
		return tellInitShadowBackup(state, cmd)
	case *CmdMoveShadowBackup:
		return tellMoveShadowBackup(state, cmd)
	case *CmdUpdateShadowBackupRecipients:
		return tellUpdateShadowBackupRecipients(state, cmd)
	case *CmdDeleteShadowBackupRecipients:
		return tellDeleteShadowBackupRecipients(state, cmd)
	case *CmdConfirmGit:
		return tellConfirmGit(state, cmd)
	case *CmdEnableGitlab:
		return tellEnableGitlab(state, cmd)
	case *CmdBeginFreeze:
		return tellBeginFreeze(state, cmd)
	case *CmdCommitFreeze:
		return tellCommitFreeze(state, cmd)
	case *CmdAbortFreeze:
		return tellAbortFreeze(state, cmd)
	case *CmdBeginUnfreeze:
		return tellBeginUnfreeze(state, cmd)
	case *CmdCommitUnfreeze:
		return tellCommitUnfreeze(state, cmd)
	case *CmdAbortUnfreeze:
		return tellAbortUnfreeze(state, cmd)
	case *CmdBeginArchive:
		return tellBeginArchive(state, cmd)
	case *CmdCommitArchive:
		return tellCommitArchive(state, cmd)
	case *CmdAbortArchive:
		return tellAbortArchive(state, cmd)
	case *CmdBeginUnarchive:
		return tellBeginUnarchive(state, cmd)
	case *CmdCommitUnarchive:
		return tellCommitUnarchive(state, cmd)
	case *CmdAbortUnarchive:
		return tellAbortUnarchive(state, cmd)
	case *CmdSetRepoError:
		return tellSetRepoError(state, cmd)
	case *CmdClearRepoError:
		return tellClearRepoError(state, cmd)
	default:
		return nil, ErrCommandUnknown
	}
}

func tellInitRepo(state *State, cmd *CmdInitRepo) ([]events.Event, error) {
	if cmd.ArchiveRecipients.HasDuplicate() {
		return nil, ErrDuplicateGPGKeys
	}
	if cmd.ShadowBackupRecipients.HasDuplicate() {
		return nil, ErrDuplicateGPGKeys
	}

	// If already initialized, check that the command is idempotent.
	// XXX Extend check.
	if state.globalPath != "" {
		if cmd.GlobalPath != state.globalPath {
			return nil, ErrInitConflict
		}
		return nil, nil
	}

	// Gitlab must either be:
	//
	// - completely unset: nogfsostad manages locally, including meta.
	// - completely set: nogfsostad pushes, nogfsog2nd manages meta.
	//
	switch {
	case cmd.GitlabHost == "" && cmd.GitlabPath == "": // ok
	case cmd.GitlabHost != "" && cmd.GitlabPath != "": // ok
	default:
		return nil, ErrGitlabConfigInvalid
	}

	// `SubdirTracking` must be explicitly specified for new repos.
	var subdirTracking pb.SubdirTracking
	switch cmd.SubdirTracking {
	case EnterSubdirs:
		subdirTracking = pb.SubdirTracking_ST_ENTER_SUBDIRS
	case BundleSubdirs:
		subdirTracking = pb.SubdirTracking_ST_BUNDLE_SUBDIRS
	case IgnoreSubdirs:
		subdirTracking = pb.SubdirTracking_ST_IGNORE_SUBDIRS
	case IgnoreMost:
		subdirTracking = pb.SubdirTracking_ST_IGNORE_MOST
	default:
		return nil, ErrSubdirTrackingInvalid
	}

	// XXX Validate more.

	evs := make([]pb.RepoEvent, 0, 3)
	evs = append(evs, pbevents.NewRepoInitStarted(
		&pb.FsoRepoInitInfo{
			Registry:       cmd.Registry,
			GlobalPath:     cmd.GlobalPath,
			CreatorName:    cmd.CreatorName,
			CreatorEmail:   cmd.CreatorEmail,
			FileHost:       cmd.FileHost,
			HostPath:       cmd.HostPath,
			GitlabHost:     cmd.GitlabHost,
			GitlabPath:     cmd.GitlabPath,
			GitToNogAddr:   cmd.GitToNogAddr,
			SubdirTracking: subdirTracking,
		},
	))
	if len(cmd.ArchiveRecipients) > 0 {
		evs = append(evs, pbevents.NewArchiveRecipientsUpdated(
			cmd.ArchiveRecipients.Bytes(),
		))
	}
	if len(cmd.ShadowBackupRecipients) > 0 {
		evs = append(evs, pbevents.NewShadowBackupRecipientsUpdated(
			cmd.ShadowBackupRecipients.Bytes(),
		))
	}
	return newEvents(state.Vid(), evs...)
}

func tellConfirmShadow(
	state *State, cmd *CmdConfirmShadow,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrUninitialized
	}

	// XXX Validate more.

	if state.shadowPath != "" {
		if cmd.ShadowPath != state.shadowPath {
			return nil, ErrInitConflict
		}
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewShadowRepoCreated(
		cmd.ShadowPath,
	))
}

func tellBeginMoveRepo(
	state *State, cmd *CmdBeginMoveRepo,
) ([]events.Event, error) {
	// Require that the shadow is initialized, because the real and shadow
	// repo must be moved together.
	if state.shadowPath == "" {
		return nil, ErrMissingShadow
	}

	workflowId := cmd.WorkflowId
	if workflowId == uuid.Nil {
		return nil, ErrMalformedWorkflowId
	}
	if state.HasActiveMoveRepo() {
		if workflowId == state.moveRepoWorkflow {
			return nil, ErrWorkflowActive
		}
		// XXX Maybe check idempotency.
		return nil, ErrConflictWorkflow
	}
	if workflowId == state.moveRepoWorkflow {
		return nil, ErrWorkflowReuse
	}

	ev := &pbevents.EvRepoMoveStarted{
		RegistryEventId: cmd.RegistryEventId,
		WorkflowId:      workflowId,
		OldGlobalPath:   state.globalPath,
		OldFileHost:     state.fileHost,
		OldHostPath:     state.hostPath,
		OldShadowPath:   state.shadowPath,
		NewGlobalPath:   cmd.NewGlobalPath,
		NewFileHost:     cmd.NewFileHost,
		NewHostPath:     cmd.NewHostPath,
	}
	return newEvents(state.Vid(), pbevents.NewPbRepoMoveStarted(ev))
}

func tellCommitMoveRepo(
	state *State, cmd *CmdCommitMoveRepo,
) ([]events.Event, error) {
	if cmd.WorkflowId != state.moveRepoWorkflow {
		return nil, ErrConflictWorkflow
	}
	if !state.HasActiveMoveRepo() {
		// Workflow has already ended.
		return nil, nil // idempotent
	}

	// XXX Maybe check consistency with corresponding EvRepoMoveStarted
	// event.

	ev := &pbevents.EvRepoMoved{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		GlobalPath:      cmd.GlobalPath,
		FileHost:        cmd.FileHost,
		HostPath:        cmd.HostPath,
		ShadowPath:      cmd.ShadowPath,
	}
	return newEvents(state.Vid(), pbevents.NewPbRepoMoved(ev))
}

func tellBeginMoveShadow(
	state *State, cmd *CmdBeginMoveShadow,
) ([]events.Event, error) {
	if state.shadowPath == "" {
		return nil, ErrMissingShadow
	}

	if state.HasActiveMoveShadow() {
		if cmd.WorkflowId != state.moveShadowWorkflow {
			return nil, ErrConflictWorkflow
		}
		if cmd.NewShadowPath != state.newShadowPath {
			return nil, ErrConflictShadowPath
		}
		return nil, nil // idempotent
	}

	if cmd.WorkflowId == state.moveShadowWorkflow {
		return nil, ErrWorkflowReuse
	}

	if cmd.NewShadowPath == state.shadowPath {
		return nil, ErrShadowPathUnchanged
	}

	// XXX Validate more?

	return newEvents(state.Vid(), pbevents.NewShadowRepoMoveStarted(
		cmd.WorkflowId, cmd.NewShadowPath,
	))
}

func tellCommitMoveShadow(
	state *State, cmd *CmdCommitMoveShadow,
) ([]events.Event, error) {
	if cmd.WorkflowId != state.moveShadowWorkflow {
		return nil, ErrConflictWorkflow
	}
	if !state.HasActiveMoveShadow() {
		// Workflow has already ended.
		return nil, nil // idempotent
	}

	return newEvents(state.Vid(), pbevents.NewShadowRepoMoved(
		state.moveShadowWorkflow,
		cmd.WorkflowEventId,
		state.newShadowPath,
	))
}

var rgxTarttURL = regexp.MustCompile(regexpx.Verbose(`
	^
	tartt://
	[a-z0-9.-]+
	/[/a-z0-9_.-]+
	\?
	(
		driver=local |
		driver=localtape&tardir=[/a-z0-9_.-]+
	)
	$
`))

func tellInitTartt(
	state *State, cmd *CmdInitTartt,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrNotInitialized
	}

	if !rgxTarttURL.MatchString(cmd.TarttURL) {
		return nil, ErrMalformedTarttURL
	}

	if state.archiveURL != "" {
		if cmd.TarttURL != state.archiveURL {
			return nil, ErrInitConflict
		}
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewTarttRepoCreated(
		cmd.TarttURL,
	))
}

func (cmd *CmdUpdateArchiveRecipients) checkTell() error {
	if len(cmd.Keys) == 0 {
		return ErrNoGPGKeys
	}
	if cmd.Keys.HasDuplicate() {
		return ErrDuplicateGPGKeys
	}
	return nil
}

func tellUpdateArchiveRecipients(
	st *State, cmd *CmdUpdateArchiveRecipients,
) ([]events.Event, error) {
	if err := cmd.checkTell(); err != nil {
		return nil, err
	}

	if st.globalPath == "" {
		return nil, ErrNotInitialized
	}

	if st.archiveRecipients.Equal(cmd.Keys) {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewArchiveRecipientsUpdated(cmd.Keys.Bytes()),
	)
}

func tellDeleteArchiveRecipients(
	st *State, cmd *CmdDeleteArchiveRecipients,
) ([]events.Event, error) {
	if st.globalPath == "" {
		return nil, ErrNotInitialized
	}

	if len(st.archiveRecipients) == 0 {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewArchiveRecipientsUpdated(nil),
	)
}

var rgxShadowBackupURL = regexp.MustCompile(regexpx.Verbose(`
	^
	nogfsobak://
	[a-z0-9.-]+
	/[/a-z0-9_.-]+
	$
`))

func tellInitShadowBackup(
	state *State, cmd *CmdInitShadowBackup,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrNotInitialized
	}

	if !rgxShadowBackupURL.MatchString(cmd.ShadowBackupURL) {
		return nil, ErrMalformedShadowBackupURL
	}

	if state.shadowBackupURL != "" {
		if cmd.ShadowBackupURL != state.shadowBackupURL {
			return nil, ErrInitConflict
		}
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewShadowBackupRepoCreated(
		cmd.ShadowBackupURL,
	))
}

func tellMoveShadowBackup(
	state *State, cmd *CmdMoveShadowBackup,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrNotInitialized
	}
	if state.shadowBackupURL == "" {
		return nil, ErrNotInitializedShadowBackup
	}

	if cmd.ShadowBackupURL == state.shadowBackupURL {
		return nil, nil // state already as desired.
	}

	if !rgxShadowBackupURL.MatchString(cmd.ShadowBackupURL) {
		return nil, ErrMalformedShadowBackupURL
	}

	return newEvents(state.Vid(), pbevents.NewShadowBackupRepoMoved(
		cmd.ShadowBackupURL,
	))
}

func (cmd *CmdUpdateShadowBackupRecipients) checkTell() error {
	if len(cmd.Keys) == 0 {
		return ErrNoGPGKeys
	}
	if cmd.Keys.HasDuplicate() {
		return ErrDuplicateGPGKeys
	}
	return nil
}

func tellUpdateShadowBackupRecipients(
	st *State, cmd *CmdUpdateShadowBackupRecipients,
) ([]events.Event, error) {
	if err := cmd.checkTell(); err != nil {
		return nil, err
	}

	if st.globalPath == "" {
		return nil, ErrNotInitialized
	}

	if st.shadowBackupRecipients.Equal(cmd.Keys) {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewShadowBackupRecipientsUpdated(cmd.Keys.Bytes()),
	)
}

func tellDeleteShadowBackupRecipients(
	st *State, cmd *CmdDeleteShadowBackupRecipients,
) ([]events.Event, error) {
	if st.globalPath == "" {
		return nil, ErrNotInitialized
	}

	if len(st.shadowBackupRecipients) == 0 {
		return nil, nil // idempotent
	}

	return newEvents(st.Vid(),
		pbevents.NewShadowBackupRecipientsUpdated(nil),
	)
}

func tellConfirmGit(
	state *State, cmd *CmdConfirmGit,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrUninitialized
	}

	// XXX Validate more.

	if state.gitlabProjectId != 0 {
		if cmd.GitlabProjectId != state.gitlabProjectId {
			return nil, ErrInitConflict
		}
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewGitRepoCreated(
		&pb.FsoGitRepoInfo{
			GitlabProjectId: cmd.GitlabProjectId,
		},
	))
}

func tellEnableGitlab(
	state *State, cmd *CmdEnableGitlab,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrUninitialized
	}

	gns := cmd.GitlabNamespace
	gns = strings.TrimRight(gns, "/")
	// XXX Should be validated more strictly, probably regex.
	parts := strings.SplitN(gns, "/", 2)
	if len(parts) != 2 {
		return nil, ErrGitlabNamespaceInvalid
	}
	gitlabHost := parts[0]
	gitlabPath := fmt.Sprintf("%s/%s", parts[1], state.id.String())

	isIdempotent := func() bool {
		return state.gitlabHost == gitlabHost &&
			state.gitlabPath == gitlabPath
	}

	if state.gitlabHost != "" {
		if isIdempotent() {
			return nil, nil
		}
		return nil, ErrGitlabPathConflict
	}

	return newEvents(state.Vid(), pbevents.NewEnableGitlabAccepted(
		gitlabHost, gitlabPath,
	))
}

func tellBeginFreeze(
	st *State, cmd *CmdBeginFreeze,
) ([]events.Event, error) {
	if st.HasActiveMoveRepo() || st.HasActiveMoveShadow() {
		return nil, ErrConflictWorkflow
	}

	switch st.storageTier {
	case StorageOnline:
		if st.errorMessage != "" {
			return nil, ErrConflictRepoError
		}
		break // If data is online, begin freeze.
	case StorageFreezing:
		if st.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictStorageWorkflow
		}
		return nil, nil // idempotent
	case StorageFreezeFailed:
		// If a freeze failed and the error message has been cleared,
		// begin a new freeze.
		if st.errorMessage != "" {
			return nil, ErrConflictStorageWorkflow
		}
		break
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewFreezeRepoStarted2(cmd.WorkflowId),
	)
}

func tellCommitFreeze(
	st *State, cmd *CmdCommitFreeze,
) ([]events.Event, error) {
	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageFreezing:
		break // If freezing, complete freeze.
	case StorageFrozen:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewFreezeRepoCompleted2Ok(cmd.WorkflowId),
	)
}

func tellAbortFreeze(
	st *State, cmd *CmdAbortFreeze,
) ([]events.Event, error) {
	if cmd.StatusCode == 0 {
		return nil, ErrInvalidErrorStatusCode
	}
	if cmd.StatusMessage == "" {
		return nil, ErrInvalidErrorStatusMessage
	}
	if len(cmd.StatusMessage) > ConfigMaxStatusMessageLength {
		return nil, ErrStatusMessageTooLong
	}

	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageFreezing:
		break // If freezing, complete freeze.
	case StorageFreezeFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewFreezeRepoCompleted2Error(
			cmd.WorkflowId, cmd.StatusCode,
		),
		pbevents.NewRepoErrorSet(
			fmt.Sprintf("freeze failed: %s", cmd.StatusMessage),
		),
	)
}

func tellBeginUnfreeze(
	st *State, cmd *CmdBeginUnfreeze,
) ([]events.Event, error) {
	if st.HasActiveMoveRepo() || st.HasActiveMoveShadow() {
		return nil, ErrConflictWorkflow
	}

	switch st.storageTier {
	case StorageFrozen:
		if st.errorMessage != "" {
			return nil, ErrConflictRepoError
		}
		break // If data is frozen, begin unfreeze.
	case StorageUnfreezing:
		if st.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictStorageWorkflow
		}
		return nil, nil // idempotent
	case StorageUnfreezeFailed:
		// If an unfreeze failed and the error message has been
		// cleared, begin a new unfreeze.
		if st.errorMessage != "" {
			return nil, ErrConflictStorageWorkflow
		}
		break
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnfreezeRepoStarted2(cmd.WorkflowId),
	)
}

func tellCommitUnfreeze(
	st *State, cmd *CmdCommitUnfreeze,
) ([]events.Event, error) {
	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageUnfreezing:
		break // If unfreezing, complete unfreeze.
	case StorageOnline:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnfreezeRepoCompleted2Ok(cmd.WorkflowId),
	)
}

func tellAbortUnfreeze(
	st *State, cmd *CmdAbortUnfreeze,
) ([]events.Event, error) {
	if cmd.StatusCode == 0 {
		return nil, ErrInvalidErrorStatusCode
	}
	if cmd.StatusMessage == "" {
		return nil, ErrInvalidErrorStatusMessage
	}
	if len(cmd.StatusMessage) > ConfigMaxStatusMessageLength {
		return nil, ErrStatusMessageTooLong
	}

	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageUnfreezing:
		break // If unfreezing, complete unfreeze.
	case StorageUnfreezeFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnfreezeRepoCompleted2Error(
			cmd.WorkflowId, cmd.StatusCode,
		),
		pbevents.NewRepoErrorSet(
			fmt.Sprintf("freeze failed: %s", cmd.StatusMessage),
		),
	)
}

// `MayArchiveRepo()` is used in `BeginArchiveRepo()` to check preconditions
// before initializing an archive-repo workflow.
func (st *State) MayArchive() (ok bool, reason string) {
	switch st.storageTier {
	case StorageFrozen:
		break // Data that is frozen can be archived.
	case StorageArchiveFailed:
		break // A failed archive can be retried.
	default:
		return false, "conflicting repo storage state"
	}
	if st.errorMessage != "" {
		return false, "repo has stored error"
	}
	if st.archiveURL == "" {
		return false, "no tartt repo"
	}
	return true, ""
}

func tellBeginArchive(
	st *State, cmd *CmdBeginArchive,
) ([]events.Event, error) {
	if st.HasActiveMoveRepo() || st.HasActiveMoveShadow() {
		return nil, ErrConflictWorkflow
	}

	switch st.storageTier {
	case StorageFrozen:
		if st.errorMessage != "" {
			return nil, ErrConflictRepoError
		}
		break // If data is frozen, begin archive.
	case StorageArchiving:
		if st.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictStorageWorkflow
		}
		return nil, nil // idempotent
	case StorageArchiveFailed:
		// If archive failed and the error message has been cleared,
		// begin archive again.
		if st.errorMessage != "" {
			return nil, ErrConflictStorageWorkflow
		}
		break
	default:
		return nil, ErrConflictStorageWorkflow
	}

	if st.archiveURL == "" {
		return nil, ErrNoTarttRepo
	}

	return newEvents(
		st.Vid(),
		pbevents.NewArchiveRepoStarted(cmd.WorkflowId),
	)
}

func tellCommitArchive(
	st *State, cmd *CmdCommitArchive,
) ([]events.Event, error) {
	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageArchiving:
		break // If archiving, complete archive.
	case StorageArchived:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	if cmd.TarPath == "" {
		return nil, ErrEmptyTarPath
	}

	return newEvents(
		st.Vid(),
		pbevents.NewArchiveRepoCompletedOk(
			cmd.WorkflowId,
			cmd.TarPath,
		),
	)
}

func tellAbortArchive(
	st *State, cmd *CmdAbortArchive,
) ([]events.Event, error) {
	if cmd.StatusCode == 0 {
		return nil, ErrInvalidErrorStatusCode
	}
	if cmd.StatusMessage == "" {
		return nil, ErrInvalidErrorStatusMessage
	}
	if len(cmd.StatusMessage) > ConfigMaxStatusMessageLength {
		return nil, ErrStatusMessageTooLong
	}

	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageArchiving:
		break // If archiving, complete archive.
	case StorageArchiveFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewArchiveRepoCompletedError(
			cmd.WorkflowId, cmd.StatusCode,
		),
		pbevents.NewRepoErrorSet(
			fmt.Sprintf("archive failed: %s", cmd.StatusMessage),
		),
	)
}

func tellBeginUnarchive(
	st *State, cmd *CmdBeginUnarchive,
) ([]events.Event, error) {
	if st.HasActiveMoveRepo() || st.HasActiveMoveShadow() {
		return nil, ErrConflictWorkflow
	}

	switch st.storageTier {
	case StorageArchived:
		if st.errorMessage != "" {
			return nil, ErrConflictRepoError
		}
		break // If data is archived, begin unarchive.
	case StorageUnarchiving:
		if st.storageWorkflowId != cmd.WorkflowId {
			return nil, ErrConflictStorageWorkflow
		}
		return nil, nil // idempotent
	case StorageUnarchiveFailed:
		// If unarchive failed and the error message has been cleared,
		// begin unarchive again.
		if st.errorMessage != "" {
			return nil, ErrConflictStorageWorkflow
		}
		break
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnarchiveRepoStarted(cmd.WorkflowId),
	)
}

func tellCommitUnarchive(
	st *State, cmd *CmdCommitUnarchive,
) ([]events.Event, error) {
	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageUnarchiving:
		break // If unarchiving, complete unarchive.
	case StorageFrozen:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnarchiveRepoCompletedOk(cmd.WorkflowId),
	)
}

func tellAbortUnarchive(
	st *State, cmd *CmdAbortUnarchive,
) ([]events.Event, error) {
	if cmd.StatusCode == 0 {
		return nil, ErrInvalidErrorStatusCode
	}
	if cmd.StatusMessage == "" {
		return nil, ErrInvalidErrorStatusMessage
	}
	if len(cmd.StatusMessage) > ConfigMaxStatusMessageLength {
		return nil, ErrStatusMessageTooLong
	}

	if st.storageWorkflowId != cmd.WorkflowId {
		return nil, ErrConflictStorageWorkflow
	}
	switch st.storageTier {
	case StorageUnarchiving:
		break // If unarchiving, complete unarchive.
	case StorageUnarchiveFailed:
		// XXX Maybe check that `StatusCode` is idempotent.
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStorageWorkflow
	}

	return newEvents(
		st.Vid(),
		pbevents.NewUnarchiveRepoCompletedError(
			cmd.WorkflowId, cmd.StatusCode,
		),
		pbevents.NewRepoErrorSet(
			fmt.Sprintf("unarchive failed: %s", cmd.StatusMessage),
		),
	)
}

func tellSetRepoError(
	state *State, cmd *CmdSetRepoError,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrUninitialized
	}

	if state.errorMessage != "" {
		if cmd.ErrorMessage != state.errorMessage {
			return nil, ErrCommandConflict
		}
		return nil, nil
	}

	return newEvents(state.Vid(), pbevents.NewRepoErrorSet(
		cmd.ErrorMessage,
	))
}

func tellClearRepoError(
	state *State, cmd *CmdClearRepoError,
) ([]events.Event, error) {
	if state.globalPath == "" {
		return nil, ErrUninitialized
	}

	if cmd.ErrorMessage == "" {
		return nil, ErrClearMessageEmpty
	}
	if cmd.ErrorMessage != state.errorMessage {
		return nil, ErrClearMessageMismatch
	}

	return newEvents(state.Vid(), pbevents.NewRepoErrorCleared())
}

type Repos struct {
	engine *events.Engine
}

func New(journal *events.Journal) *Repos {
	eng := events.NewEngine(journal, Behavior{})
	return &Repos{engine: eng}
}

func (r *Repos) Init(id uuid.I, info *CmdInitRepo) (ulid.I, error) {
	cmd := info
	return r.engine.TellIdVid(id, NoVC, cmd)
}

func (r *Repos) ConfirmShadow(
	id uuid.I, vid ulid.I, shadowPath string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdConfirmShadow{
		ShadowPath: shadowPath,
	})
}

// `BeginMoveRepo()` is used by repoinit to start a move-repo workflow from on
// a corresponding `RegistryEvent`.  See `moverepowf` for details.
func (r *Repos) BeginMoveRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginMoveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `CommitMoveRepo()` completes the workflow that started with
// `BeginMoveRepo()`.
func (r *Repos) CommitMoveRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitMoveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `BeginMoveShadow()` starts a workflow to change the shadow location.
// `workflowId` is a client-generated nonce that is used to identify the
// workflow instance and to handle concurrent and repeated invocations.
func (r *Repos) BeginMoveShadow(
	id uuid.I, vid ulid.I, workflowId uuid.I, newShadowPath string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdBeginMoveShadow{
		WorkflowId:    workflowId,
		NewShadowPath: newShadowPath,
	})
}

// `CommitMoveShadow()` completes the workflow that started with
// `BeginMoveShadow()`.
func (r *Repos) CommitMoveShadow(
	id uuid.I, vid ulid.I, workflowId uuid.I, workflowEventId ulid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdCommitMoveShadow{
		WorkflowId:      workflowId,
		WorkflowEventId: workflowEventId,
	})
}

// `InitTartt()` sets the archive URL.
func (r *Repos) InitTartt(
	id uuid.I, vid ulid.I, tarttURL string,
) (ulid.I, error) {
	cmd := CmdInitTartt{TarttURL: tarttURL}
	return r.engine.TellIdVid(id, vid, &cmd)
}

// `UpdateArchiveRecipients()` sets the archive GPG keys, enabling encryption.
func (r *Repos) UpdateArchiveRecipients(
	id uuid.I, vid ulid.I, keys gpg.Fingerprints,
) (*State, error) {
	cmd := &CmdUpdateArchiveRecipients{
		Keys: keys,
	}
	st, err := r.engine.TellIdVidState(id, vid, cmd)
	if err != nil {
		return nil, err
	}
	return st.(*State), nil
}

// `DeleteArchiveRecipients()` disables archive encryption.
func (r *Repos) DeleteArchiveRecipients(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	cmd := &CmdDeleteArchiveRecipients{}
	return r.engine.TellIdVid(id, vid, cmd)
}

// `InitShadowBackup()` sets the archive URL.
func (r *Repos) InitShadowBackup(
	id uuid.I, vid ulid.I, shadowBackupURL string,
) (ulid.I, error) {
	cmd := CmdInitShadowBackup{ShadowBackupURL: shadowBackupURL}
	return r.engine.TellIdVid(id, vid, &cmd)
}

// `UpdateShadowBackupRecipients()` sets the shadow backup GPG keys, enabling
// encryption.
func (r *Repos) UpdateShadowBackupRecipients(
	id uuid.I, vid ulid.I, keys gpg.Fingerprints,
) (*State, error) {
	cmd := &CmdUpdateShadowBackupRecipients{
		Keys: keys,
	}
	st, err := r.engine.TellIdVidState(id, vid, cmd)
	if err != nil {
		return nil, err
	}
	return st.(*State), nil
}

// `DeleteShadowBackupRecipients()` disables shadow backup encryption.
func (r *Repos) DeleteShadowBackupRecipients(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	cmd := &CmdDeleteShadowBackupRecipients{}
	return r.engine.TellIdVid(id, vid, cmd)
}

// `MoveShadowBackup()` changes the archive URL.
func (r *Repos) MoveShadowBackup(
	id uuid.I, vid ulid.I, shadowBackupURL string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdMoveShadowBackup{
		ShadowBackupURL: shadowBackupURL,
	})
}

func (r *Repos) ConfirmGit(
	id uuid.I, vid ulid.I, gitlabProjectId int64,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdConfirmGit{
		GitlabProjectId: gitlabProjectId,
	})
}

// Internal use only.  External clients must enable GitLab via fsoregistry.
func (r *Repos) EnableGitlab(
	id uuid.I, vid ulid.I, gitlabNamespace string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdEnableGitlab{
		GitlabNamespace: gitlabNamespace,
	})
}

func (r *Repos) SetRepoError(
	id uuid.I, vid ulid.I, cmd *CmdSetRepoError,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Repos) ClearRepoError(
	id uuid.I, vid ulid.I, cmd *CmdClearRepoError,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `BeginFreeze()` starts a freeze.
func (r *Repos) BeginFreeze(
	id uuid.I, vid ulid.I, cmd *CmdBeginFreeze,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `CommitFreeze()` completes a successful freeze.
func (r *Repos) CommitFreeze(
	id uuid.I, vid ulid.I, cmd *CmdCommitFreeze,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `AbortFreeze()` completes a failed freeze.
func (r *Repos) AbortFreeze(
	id uuid.I, vid ulid.I, cmd *CmdAbortFreeze,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `BeginUnfreeze()` starts a freeze.
func (r *Repos) BeginUnfreeze(
	id uuid.I, vid ulid.I, cmd *CmdBeginUnfreeze,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `CommitUnfreeze()` completes a successful freeze.
func (r *Repos) CommitUnfreeze(
	id uuid.I, vid ulid.I, cmd *CmdCommitUnfreeze,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `AbortUnfreeze()` completes a failed freeze.
func (r *Repos) AbortUnfreeze(
	id uuid.I, vid ulid.I, cmd *CmdAbortUnfreeze,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `BeginArchive()` starts a freeze.
func (r *Repos) BeginArchive(
	id uuid.I, vid ulid.I, cmd *CmdBeginArchive,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `CommitArchive()` completes a successful freeze.
func (r *Repos) CommitArchive(
	id uuid.I, vid ulid.I, cmd *CmdCommitArchive,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `AbortArchive()` completes a failed freeze.
func (r *Repos) AbortArchive(
	id uuid.I, vid ulid.I, cmd *CmdAbortArchive,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `BeginUnarchive()` starts a freeze.
func (r *Repos) BeginUnarchive(
	id uuid.I, vid ulid.I, cmd *CmdBeginUnarchive,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `CommitUnarchive()` completes a successful freeze.
func (r *Repos) CommitUnarchive(
	id uuid.I, vid ulid.I, cmd *CmdCommitUnarchive,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

// `AbortUnarchive()` completes a failed freeze.
func (r *Repos) AbortUnarchive(
	id uuid.I, vid ulid.I, cmd *CmdAbortUnarchive,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Repos) FindId(id uuid.I) (*State, error) {
	s, err := r.engine.FindId(id)
	if err != nil {
		return nil, err
	}
	if s.Vid() == events.EventEpoch {
		return nil, ErrUninitialized
	}
	return s.(*State), nil
}

func (s *State) Registry() string { return s.registry }

func (s *State) GlobalPath() string { return s.globalPath }

func (s *State) FileLocation() string {
	return fmt.Sprintf("%s:%s", s.fileHost, s.hostPath)
}

func (s *State) ArchiveURL() string {
	return s.archiveURL
}

func (s *State) ArchiveRecipients() gpg.Fingerprints {
	return s.archiveRecipients
}

func (s *State) TarttTarPath() string {
	return s.tarttTarPath
}

func (s *State) ShadowBackupURL() string {
	return s.shadowBackupURL
}

func (s *State) ShadowBackupRecipients() gpg.Fingerprints {
	return s.shadowBackupRecipients
}

func (s *State) ShadowLocation() string {
	if s.shadowPath == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", s.fileHost, s.shadowPath)
}

func (s *State) HasActiveMoveRepo() bool {
	return s.moveRepoWorkflow != uuid.Nil && s.newGlobalPath != ""
}

func (s *State) MoveRepoId() uuid.I {
	return s.moveRepoWorkflow
}

func (s *State) HasActiveMoveShadow() bool {
	return s.moveShadowWorkflow != uuid.Nil && s.newShadowPath != ""
}

func (s *State) MoveShadowId() uuid.I {
	return s.moveShadowWorkflow
}

func (s *State) GitlabLocation() string {
	if s.gitlabPath == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", s.gitlabHost, s.gitlabPath)
}

func (s *State) GitlabProjectId() int64 {
	return s.gitlabProjectId
}

func (s *State) ErrorMessage() string {
	return s.errorMessage
}

func (st *State) StorageTier() StorageTierCode {
	return st.storageTier
}
