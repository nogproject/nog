/*

Package `durootwf` implements the du-root ephemeral workflow, which runs the
Unix command `du` on an FSO root.

Workflow Events

The workflow is initialized with `WorkflowEvent_EV_FSO_DU_ROOT_STARTED` on the
workflow aggregated and a corresponding `WorkflowEvent_EV_FSO_DU_ROOT_STARTED`
on the ephemeral workflow index.

Nogfsostad observes the workflow and posts the `du` output as multiple
`WorkflowEvent_EV_FSO_DU_UPDATED` events, followed by
`WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED`.

Nogfsostad commits the workflow, which stores
`WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED` on the workflow index and a final
`WorkflowEvent_EV_FSO_DU_ROOT_COMMITTED` on the workflow.

The final workflow event has no observable side effect.  Its only purpose is to
explicitly confirm termination of the workflow history.  The final event may be
missing if a multi-step command to complete the workflow gets interrupted.

*/
package durootwf

import (
	"errors"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var NoVC = events.NoVC
var RetryNoVC = events.RetryNoVC

// Use same value as `GetSplitRootO_SC_EXPIRED` in case we want to unify in the
// future.
const StatusCodeExpired = 104

var ErrConflictInit = errors.New("workflow init conflict")
var ErrConflictStateAdvanced = errors.New("workflow state has advanced")
var ErrInvalidCommand = errors.New("invalid workflow command")
var ErrUninitialized = errors.New("workflow uninitialized")
var ErrConflictState = errors.New("command conflicts with current state")
var ErrInvalidEventType = errors.New("invalid event type")

type StateCode int

const (
	StateUninitialized StateCode = iota
	StateInitialized
	StateAppending
	StateCompleted
	StateTerminated
)

type State struct {
	id    uuid.I
	vid   ulid.I
	scode StateCode

	registryId uuid.I
	globalRoot string
	host       string
	hostRoot   string
}

type CmdInit struct {
	RegistryId uuid.I
	GlobalRoot string
	Host       string
	HostRoot   string
}

type CmdAppend struct {
	Path  string
	Usage int64
}

type CmdCommit struct{}

type CmdFail struct {
	Code    int32
	Message string
}

type CmdAbortExpired struct{}

type CmdEnd struct{}

type CmdDelete struct{}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()         {}
func (*CmdAppend) AggregateCommand()       {}
func (*CmdCommit) AggregateCommand()       {}
func (*CmdFail) AggregateCommand()         {}
func (*CmdAbortExpired) AggregateCommand() {}
func (*CmdEnd) AggregateCommand()          {}
func (*CmdDelete) AggregateCommand()       {}

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
		return ErrInvalidEventType
	case pb.WorkflowEvent_EV_FSO_DU_ROOT_STARTED:
	case pb.WorkflowEvent_EV_FSO_DU_UPDATED:
	case pb.WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_DU_ROOT_COMMITTED:
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
	case *wfev.EvDuRootStarted:
		st.scode = StateInitialized
		st.registryId = x.RegistryId
		st.globalRoot = x.GlobalRoot
		st.host = x.Host
		st.hostRoot = x.HostRoot
		return st

	case *wfev.EvDuUpdated:
		st.scode = StateAppending
		return st

	case *wfev.EvDuRootCompleted:
		st.scode = StateCompleted
		return st

	case *wfev.EvDuRootCommitted:
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
	case *CmdAppend:
		return tellAppend(st, cmd)
	case *CmdCommit:
		return tellCommit(st, cmd)
	case *CmdFail:
		return tellFail(st, cmd)
	case *CmdAbortExpired:
		return tellAbortExpired(st, cmd)
	case *CmdEnd:
		return tellEnd(st, cmd)
	case *CmdDelete:
		return tellDelete(st, cmd)
	default:
		return nil, ErrInvalidCommand
	}
}

func tellInit(st *State, cmd *CmdInit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not advanced
	// beyond init.
	switch st.scode {
	case StateUninitialized:
		// Accept command, continue below.
	case StateInitialized:
		// Check that args are idempotent.
		if cmd.RegistryId != st.registryId ||
			cmd.GlobalRoot != st.globalRoot ||
			cmd.Host != st.host ||
			cmd.HostRoot != st.hostRoot {
			return nil, ErrConflictInit
		}
		return nil, nil // idempotent
	default:
		return nil, ErrConflictStateAdvanced
	}

	// XXX Validate command fields.
	// XXX Maybe check that `st.id` is not used elsewhere.

	ev := &wfev.EvDuRootStarted{
		RegistryId: cmd.RegistryId,
		GlobalRoot: cmd.GlobalRoot,
		Host:       cmd.Host,
		HostRoot:   cmd.HostRoot,
	}
	return wfev.NewEvents(st.Vid(), wfev.NewPbDuRootStartedWorkflow(ev))
}

func tellAppend(st *State, cmd *CmdAppend) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		// Ok to start appending, continue below.
	case StateAppending:
		// Ok to continue appending.
		//
		// XXX We could check duplicate paths here.  But it seems not
		// worth it, assuming that Nogfsostad uses version control when
		// appending results.
	default:
		return nil, ErrConflictState
	}

	ev := &wfev.EvDuUpdated{
		Path:  cmd.Path,
		Usage: cmd.Usage,
	}
	return wfev.NewEvents(st.Vid(), wfev.NewPbDuUpdated(ev))
}

func tellCommit(st *State, cmd *CmdCommit) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		// Ok to complete without Append(), continue below.
	case StateAppending:
		// Continue below.
	case StateCompleted:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictState
	}

	return wfev.NewEvents(st.Vid(), wfev.NewPbDuRootCompletedOk())
}

func tellFail(st *State, cmd *CmdFail) ([]events.Event, error) {
	// Same as `tellAppend()`.
	switch st.scode {
	case StateInitialized:
		// Ok to complete without Append(), continue below.
	case StateAppending:
		// Continue below.
	case StateCompleted:
		return nil, nil // idempotent
	default:
		return nil, ErrConflictState
	}

	return wfev.NewEvents(st.Vid(), wfev.NewPbDuRootCompletedError(
		cmd.Code, cmd.Message,
	))
}

// AbortExpired can be used to abort an workflow from any initialized state.
func tellAbortExpired(
	st *State, cmd *CmdAbortExpired,
) ([]events.Event, error) {
	switch st.scode {
	case StateCompleted:
		return nil, nil // effectively idempotent
	case StateTerminated:
		return nil, nil // effectively idempotent
	case StateUninitialized:
		return nil, ErrConflictState
	default:
		break // Abort from any state except the ones above.
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbDuRootCompletedError(
			StatusCodeExpired, "expired",
		),
	)
}

func tellEnd(st *State, cmd *CmdEnd) ([]events.Event, error) {
	switch st.scode {
	case StateCompleted:
		// continue below
	case StateTerminated:
		return nil, nil
	default:
		return nil, ErrConflictState
	}

	return wfev.NewEvents(st.Vid(), wfev.NewPbDuRootCommitted())
}

func tellDelete(st *State, cmd *CmdDelete) ([]events.Event, error) {
	switch st.scode {
	// Unitialized is the idempotent result of `Delete()`.
	case StateUninitialized:
		return nil, nil

	// `Delete()` is allowed if `End()` is missing.
	case StateCompleted:
		return nil, nil

	// `Delete()` is allowed after `End()`.
	case StateTerminated:
		return nil, nil

	default:
		return nil, ErrConflictState
	}
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
		return nil, err
	}
	if st.Vid() == events.EventEpoch {
		return nil, ErrUninitialized
	}
	return st.(*State), nil
}

func (r *Workflows) Init(id uuid.I, cmd *CmdInit) (ulid.I, error) {
	return r.engine.TellIdVid(id, NoVC, cmd)
}

func (r *Workflows) Append(
	id uuid.I, vid ulid.I, cmd *CmdAppend,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Workflows) Commit(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdCommit{})
}

func (r *Workflows) Fail(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdFail{
		Code:    code,
		Message: message,
	})
}

func (r *Workflows) AbortExpired(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdAbortExpired{})
}

func (r *Workflows) End(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdEnd{})
}

func (w *Workflows) Delete(id uuid.I, vid ulid.I) error {
	return w.engine.DeleteIdVid(id, vid, &CmdDelete{})
}

func (st *State) RegistryId() uuid.I {
	return st.registryId
}

func (st *State) GlobalRoot() string {
	return st.globalRoot
}
