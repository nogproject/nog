/*

Package `pingregistrywf` implements the ping-registry ephemeral workflow, which
gathers pings from the daemons that watch a registry.  The workflow illustrates
how to implement a workflow that requires coordination between multiple servers
and an admin.

Workflow Events

An admin uses `nogfsoctl ping-registry begin` to initialized the workflow with
`WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED` on the workflow and a
corresponding `WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED` on the ephemeral
workflow index.

The Nogfsostad servers that watch the registry observe the workflow, and each
of them posts a specified number of `WorkflowEvent_EV_FSO_SERVER_PINGED`.

Nogfsoregd observes the workflow and posts a specified number of
`WorkflowEvent_EV_FSO_SERVER_PINGED`.  It then waits until the workflow
deadline and summarizes the pings in a
`WorkflowEvent_EV_FSO_SERVER_PINGS_GATHERED`.

An admin uses `nogfsoctl ping-registry commit` to complete the workflow with
`WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED`, followed by
`WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED` on the workflow index, followed
by a final `WorkflowEvent_EV_FSO_PING_REGISTRY_COMMITTED` on the workflow.

The final workflow event has no observable side effect.  Its only purpose is to
explicitly confirm termination of the workflow history.  The final event may be
missing if a multi-step command to complete the workflow gets interrupted.

Workflows are eventually deleted from the index with
`WorkflowEvent_EV_FSO_PING_REGISTRY_DELETED` on the index.  Workflows may be
deleted with or without the final
`WorkflowEvent_EV_FSO_PING_REGISTRY_COMMITTED` on the workflow.

*/
package pingregistrywf

import (
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var NoVC = events.NoVC
var RetryNoVC = events.RetryNoVC

type StateCode int

const (
	StateUninitialized StateCode = iota
	StateInitialized
	StateAppending
	StateSummarized
	StateCompleted
	StateTerminated
)

type Status struct {
	Code    int32
	Message string
	EventId ulid.I
}

type State struct {
	id         uuid.I
	vid        ulid.I
	scode      StateCode
	registryId uuid.I
	nPings     int
	pings      []Status
	summary    Status
}

type CmdInit struct {
	RegistryId uuid.I
}

type CmdAppendPing struct {
	Code    int32
	Message string
}

type CmdPostSummary struct {
	Code    int32
	Message string
}

type CmdCommit struct{}

type CmdAbortExpired struct{}

type CmdEnd struct{}

type CmdDelete struct{}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()         {}
func (*CmdAppendPing) AggregateCommand()   {}
func (*CmdPostSummary) AggregateCommand()  {}
func (*CmdCommit) AggregateCommand()       {}
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
		return &EventTypeError{}
	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED:
	case pb.WorkflowEvent_EV_FSO_SERVER_PINGED:
	case pb.WorkflowEvent_EV_FSO_SERVER_PINGS_GATHERED:
	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMMITTED:
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
	case *wfev.EvPingRegistryStarted:
		st.scode = StateInitialized
		st.registryId = x.RegistryId
		return st

	case *wfev.EvServerPinged:
		st.scode = StateAppending
		st.nPings++
		st.pings = append(st.pings, Status{
			Code:    x.StatusCode,
			Message: x.StatusMessage,
			EventId: ev.Id(),
		})
		return st

	case *wfev.EvServerPingsGathered:
		st.scode = StateSummarized
		st.summary = Status{
			Code:    x.StatusCode,
			Message: x.StatusMessage,
			EventId: ev.Id(),
		}
		return st

	case *wfev.EvPingRegistryCompleted:
		st.scode = StateCompleted
		return st

	case *wfev.EvPingRegistryCommitted:
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
	case *CmdAppendPing:
		return tellAppendPing(st, cmd)
	case *CmdPostSummary:
		return tellPostSummary(st, cmd)
	case *CmdCommit:
		return tellCommit(st, cmd)
	case *CmdAbortExpired:
		return tellAbortExpired(st, cmd)
	case *CmdEnd:
		return tellEnd(st, cmd)
	case *CmdDelete:
		return tellDelete(st, cmd)
	default:
		return nil, &InvalidCommandError{}
	}
}

func tellInit(st *State, cmd *CmdInit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not advanced
	// beyond init.
	switch st.scode {
	case StateUninitialized:
		break // Init is only allowed as the first command.
	case StateInitialized:
		// Check that args are idempotent.
		if cmd.RegistryId != st.registryId {
			return nil, &ArgumentNotIdempotentError{
				Arg: "RegistryId",
			}
		}
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	ev := &wfev.EvPingRegistryStarted{
		RegistryId: cmd.RegistryId,
	}
	return wrapEvents(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbPingRegistryStartedWorkflow(ev),
	))
}

func tellAppendPing(st *State, cmd *CmdAppendPing) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		break // Start appending.
	case StateAppending:
		break // Append more.
	default:
		return nil, &StateConflictError{}
	}

	return wrapEvents(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbServerPinged(cmd.Code, cmd.Message),
	))
}

func tellPostSummary(st *State, cmd *CmdPostSummary) ([]events.Event, error) {
	switch st.scode {
	case StateInitialized:
		break // Summarize without ping.
	case StateAppending:
		break // Summarize after appending.
	default:
		return nil, &StateConflictError{}
	}

	return wrapEvents(wfev.NewEvents(
		st.Vid(),
		wfev.NewPbServerPingsGathered(cmd.Code, cmd.Message),
	))
}

func tellCommit(st *State, cmd *CmdCommit) ([]events.Event, error) {
	switch st.scode {
	case StateSummarized:
		break // `Commit()` is only allowed after `PostSummary()`.
	case StateCompleted:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEvents(wfev.NewEvents(
		st.Vid(), wfev.NewPbPingRegistryCompleted(),
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
		return nil, &StateConflictError{}
	default:
		break // Abort from any state except the ones above.
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbPingRegistryCompleted(),
	)
}

func tellEnd(st *State, cmd *CmdEnd) ([]events.Event, error) {
	switch st.scode {
	case StateCompleted:
		break // `End()` is only allowed after `Commit()`.
	case StateTerminated:
		return nil, nil // idempotent
	default:
		return nil, &StateConflictError{}
	}

	return wrapEvents(wfev.NewEvents(
		st.Vid(), wfev.NewPbPingRegistryCommitted(),
	))
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
		return nil, &StateConflictError{}
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
		return nil, &JournalError{Err: err}
	}
	if st.Vid() == events.EventEpoch {
		return nil, &UninitializedError{}
	}
	return st.(*State), nil
}

func (r *Workflows) Init(id uuid.I, cmd *CmdInit) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, NoVC, cmd))
}

func (r *Workflows) AppendPing(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdAppendPing{
		Code:    code,
		Message: message,
	}))
}

func (r *Workflows) PostSummary(
	id uuid.I, vid ulid.I, code int32, message string,
) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdPostSummary{
		Code:    code,
		Message: message,
	}))
}

func (r *Workflows) AbortExpired(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdAbortExpired{}))
}

func (r *Workflows) Commit(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdCommit{}))
}

func (r *Workflows) End(id uuid.I, vid ulid.I) (ulid.I, error) {
	return wrapVid(r.engine.TellIdVid(id, vid, &CmdEnd{}))
}

func (w *Workflows) Delete(id uuid.I, vid ulid.I) error {
	return wrapJournal(w.engine.DeleteIdVid(id, vid, &CmdDelete{}))
}

func (st *State) RegistryId() uuid.I {
	return st.registryId
}

func (st *State) NumPings() int {
	return st.nPings
}

func (st *State) Pings() []Status {
	return st.pings
}

func (st *State) StateCode() StateCode {
	return st.scode
}

func (st *State) SummaryStatus() Status {
	return st.summary
}
