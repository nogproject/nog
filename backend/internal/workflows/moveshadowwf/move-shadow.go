/*

Package `moveshadowwf` implements the move-shadow workflow, which changes the
location of a shadow repo.

Workflow Events

The workflow is initiated on a repo with
`RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED`, currently by an admin using
`nogfsoctl begin-move-shadow`.

Nogfsoregd replicate initializes the workflow aggregate with
`WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED`.  It then posts a notification
on the registry `RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED`, so that
Nogfsostad can observe the start of the move-shadow workflow.

Nogfsostad disables the repo and posts
`WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED` to confirm that it is
disabled.

The workflow completes the move with `WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED`,
currently by an admin using `nogfsoctl commit-move-shadow`.

Nogfsoregd replicate commits the move to `RepoEvent_EV_FSO_SHADOW_REPO_MOVED`
and terminates the workflow with
`WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED`.

*/
package moveshadowwf

import (
	"errors"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `NoVC` is a sentinel value that can be passed in place of `vid` to indicate
// that concurrency version checks are skipped.
var NoVC = events.NoVC

var ErrConflict = errors.New("workflow conflict")
var ErrConflictInit = errors.New("workflow init conflict")
var ErrInvalidCommand = errors.New("invalid workflow command")
var ErrMoved = errors.New("workflow move completed")
var ErrNotMoved = errors.New("workflow did not complete move")
var ErrStaActive = errors.New("nogfsostad has yet not disabled the repo")
var ErrTerminated = errors.New("workflow terminated")
var ErrUninitialized = errors.New("workflow uninitialized")
var ErrInvalidEventType = errors.New("invalid event type")

type State struct {
	id  uuid.I
	vid ulid.I

	repoId      uuid.I
	repoEventId ulid.I

	isStaDisabled bool
	isMoved       bool
	isTerminated  bool
}

type CmdInit struct {
	RepoId      uuid.I
	RepoEventId ulid.I
}

type CmdPostStadDisabled struct{}
type CmdCommit struct{}
type CmdExit struct{}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()             {}
func (*CmdPostStadDisabled) AggregateCommand() {}
func (*CmdCommit) AggregateCommand()           {}
func (*CmdExit) AggregateCommand()             {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

type Behavior struct{}
type Event struct{ wfevents.Event }

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
	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED:
	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED:
	case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED:
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
	case *wfevents.Event: // Event from `Tell()`
		evpb = x.PbWorkflowEvent()
	default:
		panic("invalid event")
	}
	switch x := wfevents.MustParsePbWorkflowEvent(evpb).(type) {
	case *wfevents.EvShadowRepoMoveStarted:
		st.repoId = x.RepoId
		st.repoEventId = x.RepoEventId

	case *wfevents.EvShadowRepoMoveStaDisabled:
		st.isStaDisabled = true

	case *wfevents.EvShadowRepoMoved:
		st.isMoved = true

	case *wfevents.EvShadowRepoMoveCommitted:
		st.isTerminated = true

	default:
		panic("invalid event")
	}

	return st
}

func (Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	st := s.(*State)
	switch cmd := c.(type) {
	case *CmdInit:
		return tellInit(st, cmd)
	case *CmdPostStadDisabled:
		return tellPostStadDisabled(st, cmd)
	case *CmdCommit:
		return tellCommit(st, cmd)
	case *CmdExit:
		return tellExit(st, cmd)
	default:
		return nil, ErrInvalidCommand
	}
}

func tellInit(st *State, cmd *CmdInit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not advanced
	// beyond init.
	if st.isStaDisabled {
		return nil, ErrConflict
	}
	if st.repoId != uuid.Nil {
		if cmd.RepoId != st.repoId {
			return nil, ErrConflictInit
		}
		if cmd.RepoEventId != st.repoEventId {
			return nil, ErrConflictInit
		}
		return nil, nil // idempotent
	}

	// XXX more validation?  Maybe check that the repo exists.  Maybe copy
	// the repo path to the init event in order to use it for workflow
	// access check without reference to the repo.

	return wfevents.NewEvents(
		st.Vid(),
		wfevents.NewPbShadowRepoMoveStarted(
			cmd.RepoId, cmd.RepoEventId,
		),
	)
}

func tellPostStadDisabled(
	st *State, cmd *CmdPostStadDisabled,
) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not completed
	// the move.
	if st.isMoved {
		return nil, ErrMoved
	}
	if st.isStaDisabled {
		return nil, nil // idempotent
	}

	if st.repoId == uuid.Nil {
		return nil, ErrUninitialized
	}

	return wfevents.NewEvents(
		st.Vid(),
		wfevents.NewPbShadowRepoMoveStaDisabled(),
	)
}

func tellCommit(st *State, cmd *CmdCommit) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not
	// terminated.
	if st.isTerminated {
		return nil, ErrTerminated
	}
	if st.isMoved {
		return nil, nil // idempotent
	}

	// Check initialized first for more meaningful error.
	if st.repoId == uuid.Nil {
		return nil, ErrUninitialized
	}
	if !st.isStaDisabled {
		return nil, ErrStaActive
	}

	return wfevents.NewEvents(
		st.Vid(),
		wfevents.NewPbShadowRepoMoved(st.repoId),
	)
}

func tellExit(st *State, cmd *CmdExit) ([]events.Event, error) {
	if st.isTerminated {
		return nil, nil // idempotent
	}

	// Check initialized first for more meaningful error.
	if st.repoId == uuid.Nil {
		return nil, ErrUninitialized
	}
	if !st.isMoved {
		return nil, ErrNotMoved
	}

	return wfevents.NewEvents(
		st.Vid(),
		wfevents.NewPbShadowRepoMoveCommitted(),
	)
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

// `Init()` initializes a workflow.
//
// It is called by Nogfsoregd replicate when it sees a
// `RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED`.
func (r *Workflows) Init(
	id uuid.I, repoId uuid.I, repoEventId ulid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, NoVC, &CmdInit{
		RepoId:      repoId,
		RepoEventId: repoEventId,
	})
}

// `PostStadDisabled()` adds an event that indicates that Nogfsostad has
// disabled the repo.
func (r *Workflows) PostStadDisabled(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdPostStadDisabled{})
}

// `Commit()` completes the move.
func (r *Workflows) Commit(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdCommit{})
}

// `Exit()` terminates the workflow.
//
// The workflow can currently only terminate successfully, which may change in
// the future when we add more error handling.
func (r *Workflows) Exit(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdExit{})
}

func (st *State) RepoId() uuid.I {
	return st.repoId
}

func (st *State) IsTerminated() bool {
	return st.isTerminated
}
