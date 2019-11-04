/*

Package `moverepowf` implements the move-repo workflow, which simultaneously
changes the location of a real repo and its shadow repo.

The workflow supports passing the repo to a different Nogfsostad instance.

Workflow Events

The workflow is initiated on a registry with
`RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED`, currently by an admin using
`nogfsoctl begin-move-repo`.

The old Nogfsostad, the new Nogfsostad, and Nogappd observe the registry event
and start watching the workflow.

Nogfsoregd repoinit determines the file host details and posts
`RepoEvent_EV_FSO_REPO_MOVE_STARTED`.

Nogfsoregd replicate initializes the workflow aggregate with
`WorkflowEvent_EV_FSO_REPO_MOVE_STARTED`.

Nogappd posts `WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED` to confirm that the
repo has been disabled and is ready to be moved to the the new repo location.
The event should only be used on a system level, for example to confirm that
Nogappd has updated the MongoDB state.  The event should not be used to
implement a GUI workflow that involves user interaction.  One assumption is
that a workflow makes steady progress if all systems are functioning properly,
without being blocked for an unknown period, because it is waiting for a user.

The old Nogfsostad disables the repo and posts
`WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED` to confirm that the repo has
been released.

The new Nogfsostad determines the new shadow location.  It logs a message that
asks an admin to move the real repo and the shadow repo.  This step should be
automated in the future, by calling a sudo-like service to execute a privileged
operation.  For now, an admin moves the repos, changes ownership if necessary,
and confirms with `nogfsoctl commit-move-repo` which yields
`WorkflowEvent_EV_FSO_REPO_MOVED`.

Nogfsoregd replicate commits the move with `RepoEvent_EV_FSO_REPO_MOVED` and
`RegistryEvent_EV_FSO_REPO_MOVED`.  It terminates the workflow with
`WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED`.

Nogappd and the new Nogfsostad both enable the repo at the new location.

*/
package moverepowf

import (
	"errors"

	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `NoVC` is a sentinel value that can be passed in place of `vid` to indicate
// that concurrency version checks are skipped.
var NoVC = events.NoVC

var ErrConflict = errors.New("workflow conflict")
var ErrConflictInit = errors.New("workflow init conflict")
var ErrConflictStateAdvanced = errors.New("workflow state has advanced")
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

	newGlobalPath string
	newFileHost   string
	newHostPath   string

	isStaReleased  bool
	hasAppAccepted bool
	isMoved        bool
	isTerminated   bool
}

type CmdInit struct {
	RepoId        uuid.I
	RepoEventId   ulid.I
	OldGlobalPath string
	OldFileHost   string
	OldHostPath   string
	OldShadowPath string
	NewGlobalPath string
	NewFileHost   string
	NewHostPath   string
}

type CmdPostStadReleased struct{}
type CmdPostAppAccepted struct{}

type CmdCommit struct {
	NewShadowPath string
}

type CmdExit struct{}

func (*State) AggregateState() {}

func (*CmdInit) AggregateCommand()             {}
func (*CmdPostStadReleased) AggregateCommand() {}
func (*CmdPostAppAccepted) AggregateCommand()  {}
func (*CmdCommit) AggregateCommand()           {}
func (*CmdExit) AggregateCommand()             {}

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
	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_STARTED:
	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED:
	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED:
	case pb.WorkflowEvent_EV_FSO_REPO_MOVED:
	case pb.WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED:
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
	case *wfev.EvRepoMoveStarted:
		st.repoId = x.RepoId
		st.repoEventId = x.RepoEventId
		st.newGlobalPath = x.NewGlobalPath
		st.newFileHost = x.NewFileHost
		st.newHostPath = x.NewHostPath

	case *wfev.EvRepoMoveStaReleased:
		st.isStaReleased = true

	case *wfev.EvRepoMoveAppAccepted:
		st.hasAppAccepted = true

	case *wfev.EvRepoMoved:
		st.isMoved = true

	case *wfev.EvRepoMoveCommitted:
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
	case *CmdPostStadReleased:
		return tellPostStadReleased(st, cmd)
	case *CmdPostAppAccepted:
		return tellPostAppAccepted(st, cmd)
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
	if st.isStaReleased || st.hasAppAccepted {
		return nil, ErrConflictStateAdvanced
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

	// XXX more validation?

	ev := &wfev.EvRepoMoveStarted{
		RepoId:        cmd.RepoId,
		RepoEventId:   cmd.RepoEventId,
		OldGlobalPath: cmd.OldGlobalPath,
		OldFileHost:   cmd.OldFileHost,
		OldHostPath:   cmd.OldHostPath,
		OldShadowPath: cmd.OldShadowPath,
		NewGlobalPath: cmd.NewGlobalPath,
		NewFileHost:   cmd.NewFileHost,
		NewHostPath:   cmd.NewHostPath,
	}
	return wfev.NewEvents(st.Vid(), wfev.NewPbRepoMoveStarted(ev))
}

func tellPostStadReleased(
	st *State, cmd *CmdPostStadReleased,
) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not completed
	// the move.
	if st.isMoved {
		return nil, ErrMoved
	}
	if st.isStaReleased {
		return nil, nil // weakly idempotent
	}

	if st.repoId == uuid.Nil {
		return nil, ErrUninitialized
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbRepoMoveStaReleased(),
	)
}

func tellPostAppAccepted(
	st *State, cmd *CmdPostAppAccepted,
) ([]events.Event, error) {
	// The command can only be idempotent if the workflow has not completed
	// the move.
	if st.isMoved {
		return nil, ErrMoved
	}
	if st.hasAppAccepted {
		return nil, nil // weakly idempotent
	}

	if st.repoId == uuid.Nil {
		return nil, ErrUninitialized
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbRepoMoveAppAccepted(),
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

	// Check initialized to report meaningful error.
	if st.repoId == uuid.Nil {
		return nil, ErrUninitialized
	}
	if !st.isStaReleased || !st.hasAppAccepted {
		return nil, ErrStaActive
	}

	ev := &wfev.EvRepoMoved{
		RepoId:     st.repoId,
		GlobalPath: st.newGlobalPath,
		FileHost:   st.newFileHost,
		HostPath:   st.newHostPath,
		ShadowPath: cmd.NewShadowPath,
	}
	return wfev.NewEvents(st.Vid(), wfev.NewPbRepoMoved(ev))
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

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbRepoMoveCommitted(),
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
// Nogfsoregd replicate calls it on `RepoEvent_EV_FSO_REPO_MOVE_STARTED`.
func (r *Workflows) Init(
	id uuid.I, cmd *CmdInit,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, NoVC, cmd)
}

// `PostStadReleased()` adds an event that indicates that Nogfsostad has
// released the repo.
func (r *Workflows) PostStadReleased(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdPostStadReleased{})
}

// `PostAppAccepted()` adds an event that indicates that Nogappd has
// acknowledged the move.
func (r *Workflows) PostAppAccepted(id uuid.I, vid ulid.I) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdPostAppAccepted{})
}

// `Commit()` completes the move.
func (r *Workflows) Commit(
	id uuid.I, vid ulid.I, newShadowPath string,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdCommit{
		NewShadowPath: newShadowPath,
	})
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
