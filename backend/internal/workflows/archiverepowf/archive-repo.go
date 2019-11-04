package archiverepowf

import (
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var NoVC = events.NoVC
var RetryNoVC = events.RetryNoVC

// See package doc for possible state paths.
type StateCode int

const (
	StateUninitialized StateCode = iota
	StateInitialized

	StateFiles
	StateTarttCompleted
	StateSwapStarted
	StateFilesCompleted
	StateFilesFailed
	StateFilesEnded

	StateGcCompleted

	StateCompleted
	StateFailed

	StateTerminated
)

type State struct {
	id    uuid.I
	vid   ulid.I
	scode StateCode

	registryId   uuid.I
	registryName string
	repoId       uuid.I
	globalPath   string

	statusCode    int32
	statusMessage string
}

type CmdInit struct {
	RegistryId       uuid.I
	RegistryName     string
	StartRegistryVid ulid.I
	RepoId           uuid.I
	RepoGlobalPath   string
	StartRepoVid     ulid.I
	AuthorName       string
	AuthorEmail      string
}

type CmdBeginFiles struct {
	AclPolicy *pb.RepoAclPolicy
}

type CmdCommitTartt struct {
	TarPath string
}

type CmdBeginSwap struct {
	WorkingDir string
}

type CmdCommitFiles struct{}

type CmdAbortFiles struct {
	Code    int32
	Message string
}

type CmdEndFiles struct{}

type CmdCommitGc struct{}
type CmdCommit struct{}

type CmdAbort struct {
	Code    int32
	Message string
}

type CmdEnd struct{}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()        {}
func (*CmdBeginFiles) AggregateCommand()  {}
func (*CmdCommitTartt) AggregateCommand() {}
func (*CmdBeginSwap) AggregateCommand()   {}
func (*CmdCommitFiles) AggregateCommand() {}
func (*CmdAbortFiles) AggregateCommand()  {}
func (*CmdEndFiles) AggregateCommand()    {}
func (*CmdCommitGc) AggregateCommand()    {}
func (*CmdCommit) AggregateCommand()      {}
func (*CmdAbort) AggregateCommand()       {}
func (*CmdEnd) AggregateCommand()         {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

type Behavior struct{}
type Event struct{ wfev.Event }

func (Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (Behavior) NewEvent() events.Event          { return &Event{} }
func (Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

// The bools indicate which part of the state has been duplicated.
type Advancer struct {
	state bool // The state itself.
}

func (ev *Event) UnmarshalProto(data []byte) error {
	if err := ev.Event.UnmarshalProto(data); err != nil {
		return err
	}
	switch ev.Event.PbWorkflowEvent().Event {
	default:
		return &EventTypeError{}
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_STARTED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_TARTT_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_SWAP_STARTED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMMITTED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_GC_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMMITTED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_DELETED:
	}
	return nil
}

func (a *Advancer) Advance(s events.State, ev events.Event) events.State {
	st := s.(*State)

	if !a.state {
		dup := *st
		st = &dup
		a.state = true
	}

	var evpb *pb.WorkflowEvent
	switch x := ev.(type) {
	case *Event: // Event from `UnmarshalProto()`
		evpb = x.PbWorkflowEvent()
	case *wfev.Event: // Event from `Tell()`
		evpb = x.PbWorkflowEvent()
	default:
		panic("invalid event")
	}
	switch x := wfev.MustParsePbWorkflowEvent(evpb).(type) {
	case *wfev.EvArchiveRepoStarted:
		st.scode = StateInitialized
		st.registryId = x.RegistryId
		st.registryName = x.RegistryName
		st.repoId = x.RepoId
		st.globalPath = x.RepoGlobalPath
		return st

	case *wfev.EvArchiveRepoFilesStarted:
		st.scode = StateFiles
		return st

	case *wfev.EvArchiveRepoTarttCompleted:
		st.scode = StateTarttCompleted
		return st

	case *wfev.EvArchiveRepoSwapStarted:
		st.scode = StateSwapStarted
		return st

	case *wfev.EvArchiveRepoFilesCompleted:
		st.statusCode = x.StatusCode
		st.statusMessage = x.StatusMessage
		if x.StatusCode == 0 {
			st.scode = StateFilesCompleted
		} else {
			st.scode = StateFilesFailed
		}
		return st

	case *wfev.EvArchiveRepoFilesCommitted:
		st.scode = StateFilesEnded
		return st

	case *wfev.EvArchiveRepoGcCompleted:
		st.scode = StateGcCompleted
		return st

	case *wfev.EvArchiveRepoCompleted:
		st.statusCode = x.StatusCode
		st.statusMessage = x.StatusMessage
		if x.StatusCode == 0 {
			st.scode = StateCompleted
		} else {
			st.scode = StateFailed
		}
		return st

	case *wfev.EvArchiveRepoCommitted:
		st.scode = StateTerminated
		return st

	default:
		panic("invalid event")
	}
}

func (Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	st := s.(*State)
	switch cmd := c.(type) {
	case *CmdInit:
		return tellInit(st, cmd)
	case *CmdBeginFiles:
		return tellBeginFiles(st, cmd)
	case *CmdCommitTartt:
		return tellCommitTartt(st, cmd)
	case *CmdBeginSwap:
		return tellBeginSwap(st, cmd)
	case *CmdCommitFiles:
		return tellCommitFiles(st, cmd)
	case *CmdAbortFiles:
		return tellAbortFiles(st, cmd)
	case *CmdEndFiles:
		return tellEndFiles(st, cmd)
	case *CmdCommitGc:
		return tellCommitGc(st, cmd)
	case *CmdCommit:
		return tellCommit(st, cmd)
	case *CmdAbort:
		return tellAbort(st, cmd)
	case *CmdEnd:
		return tellEnd(st, cmd)
	default:
		return nil, &InvalidCommandError{}
	}
}

func (cmd *CmdInit) isIdempotent(st *State) bool {
	return cmd.RegistryId == st.registryId &&
		cmd.RegistryName == st.registryName &&
		cmd.RepoId == st.repoId &&
		cmd.RepoGlobalPath == st.globalPath
}

func tellInit(st *State, cmd *CmdInit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not advanced
	// beyond init.
	switch st.scode {
	case StateUninitialized:
		break // Init is only allowed as the first command.
	case StateInitialized:
		// Check that args are idempotent.
		if !cmd.isIdempotent(st) {
			return nil, &NotIdempotentError{}
		}
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	// XXX Maybe validate cmd fields.

	ev := &wfev.EvArchiveRepoStarted{
		RegistryId:       cmd.RegistryId,
		RegistryName:     cmd.RegistryName,
		StartRegistryVid: cmd.StartRegistryVid,
		RepoId:           cmd.RepoId,
		RepoGlobalPath:   cmd.RepoGlobalPath,
		StartRepoVid:     cmd.StartRepoVid,
		AuthorName:       cmd.AuthorName,
		AuthorEmail:      cmd.AuthorEmail,
	}
	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoStartedWorkflow(ev),
	))
}

// BeginFiles is only allowed as the first command after init.
func tellBeginFiles(st *State, cmd *CmdBeginFiles) ([]events.Event, error) {
	if cmd.AclPolicy == nil {
		return nil, &ArgumentError{Reason: "nil AclPolicy"}
	}
	switch st.scode {
	case StateInitialized:
		break
	case StateFiles:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoFilesStarted(cmd.AclPolicy),
	))
}

// CommitTartt is only allowed immediately after BeginFiles.
func tellCommitTartt(st *State, cmd *CmdCommitTartt) ([]events.Event, error) {
	switch st.scode {
	case StateFiles:
		break
	case StateTarttCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	if cmd.TarPath == "" {
		return nil, &ArgumentError{
			Reason: "empty tar path",
		}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoTarttCompleted(cmd.TarPath),
	))
}

// BeginSwap is only allowed immediately after CommitTartt.
func tellBeginSwap(st *State, cmd *CmdBeginSwap) ([]events.Event, error) {
	switch st.scode {
	case StateTarttCompleted:
		break
	case StateSwapStarted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoSwapStarted(cmd.WorkingDir),
	))
}

func tellCommitFiles(st *State, cmd *CmdCommitFiles) ([]events.Event, error) {
	switch st.scode {
	case StateSwapStarted:
		break
	case StateFilesCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoFilesCompletedOk(),
	))
}

func tellAbortFiles(st *State, cmd *CmdAbortFiles) ([]events.Event, error) {
	switch st.scode {
	case StateFiles:
		break
	case StateTarttCompleted:
		break
	case StateSwapStarted:
		break
	case StateFilesFailed:
		// XXX Maybe check that the cmd fields do not obviously
		// conflict with idempotency.
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoFilesCompletedError(cmd.Code, cmd.Message),
	))
}

func tellEndFiles(st *State, cmd *CmdEndFiles) ([]events.Event, error) {
	switch st.scode {
	case StateFilesCompleted, StateFilesFailed:
		// `EndFiles()` is allowed after `CommitFiles()` or
		// `AbortFiles()`.
		break
	case StateFilesEnded:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoFilesCommitted(),
	))
}

func tellCommitGc(st *State, cmd *CmdCommitGc) ([]events.Event, error) {
	switch st.scode {
	case StateFilesEnded:
		break
	case StateGcCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoGcCompleted(),
	))
}

func tellCommit(st *State, cmd *CmdCommit) ([]events.Event, error) {
	switch st.scode {
	case StateGcCompleted:
		// Ok to complete if files ok.
		if st.statusCode != 0 {
			return nil, &StateConflictError{}
		}
		break
	case StateCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoCompletedOk(),
	))
}

func tellAbort(st *State, cmd *CmdAbort) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		break // Ok to abort if some BeginX fails.
	case StateGcCompleted:
		// Ok to abort if files failed.
		if st.statusCode == 0 {
			return nil, &StateConflictError{}
		}
		break
	case StateFailed:
		// Abort is always considered idempotent without checking the
		// status code and message.  This may avoid confusion when
		// retrying abort along different code paths.
		return nil, nil // idempotent
	case StateTerminated:
		return nil, &AlreadyTerminatedError{}
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoCompletedError(cmd.Code, cmd.Message),
	))
}

func tellEnd(st *State, cmd *CmdEnd) ([]events.Event, error) {
	switch st.scode {
	case StateCompleted:
		break // `End()` is allowed after `Commit()`.
	case StateFailed:
		break // `End()` is allowed after `Abort()`.
	case StateTerminated:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEventsNewEventsError(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoCommitted(),
	))
}

type Workflows struct {
	engine *events.Engine
}

func New(journal *events.Journal) *Workflows {
	return &Workflows{
		engine: events.NewEngine(journal, Behavior{}),
	}
}

func (r *Workflows) FindId(id uuid.I) (*State, error) {
	st, err := r.engine.FindId(id)
	if err != nil {
		return nil, &JournalError{Err: err}
	}
	if st.Vid() == events.EventEpoch {
		return nil, &UninitializedError{}
	}
	return st.(*State), nil
}

func (r *Workflows) Init(id uuid.I, cmd *CmdInit) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, NoVC, cmd))
}

func (r *Workflows) BeginFiles(
	id uuid.I, vid ulid.I, cmd *CmdBeginFiles,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) CommitTartt(
	id uuid.I, vid ulid.I, cmd *CmdCommitTartt,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) BeginSwap(
	id uuid.I, vid ulid.I, cmd *CmdBeginSwap,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) CommitFiles(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	cmd := &CmdCommitFiles{}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) AbortFiles(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	cmd := &CmdAbortFiles{
		Code:    code,
		Message: message,
	}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) EndFiles(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, &CmdEndFiles{}))
}

func (r *Workflows) CommitGc(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	cmd := &CmdCommitGc{}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) Commit(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	cmd := &CmdCommit{}
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, cmd))
}

func (r *Workflows) Abort(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, &CmdAbort{
		Code:    code,
		Message: message,
	}))
}

func (r *Workflows) End(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVidJournalError(r.engine.TellIdVid(id, vid, &CmdEnd{}))
}

func (st *State) StateCode() StateCode {
	return st.scode
}

func (st *State) RegistryId() uuid.I {
	return st.registryId
}

func (st *State) RegistryName() string {
	return st.registryName
}

func (st *State) RepoId() uuid.I {
	return st.repoId
}

func (st *State) RepoGlobalPath() string {
	return st.globalPath
}

func (st *State) StatusCode() int32 {
	return st.statusCode
}

func (st *State) StatusMessage() string {
	return st.statusMessage
}
