package wfindexes

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

var ErrInvalidCommand = errors.New("invalid workflow index command")
var ErrUnknownWorkflow = errors.New("unknown workflow")
var ErrSmallStorageReduction = errors.New("would not save much storage")
var ErrTooManyWorkflows = errors.New("too many workflows")
var ErrInvalidEventType = errors.New("invalid event type")

const (
	ConfigMinEventsBetweenSnapshots = 1000
	ConfigMaxSnapshotWorkflows      = 200

	ConfigEventConst           = 1
	ConfigSnapshotBaseCost     = 300
	ConfigSnapshotWorkflowCost = 300
)

type State struct {
	id  uuid.I
	vid ulid.I

	activeWorkflows    map[uuid.I]struct{}
	completedWorkflows map[uuid.I]struct{}
	lastCommitted      uuid.I

	nEventsSinceSnapshot int
}

type CmdBeginDuRoot struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalRoot      string
	Host            string
	HostRoot        string
}

type CmdCommitDuRoot struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdDeleteDuRoot struct {
	WorkflowId uuid.I
}

type CmdBeginPingRegistry struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdCommitPingRegistry struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdBeginSplitRoot struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalRoot      string
	Host            string
	HostRoot        string
}

type CmdCommitSplitRoot struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdDeletePingRegistry struct {
	WorkflowId uuid.I
}

type CmdDeleteSplitRoot struct {
	WorkflowId uuid.I
}

type CmdBeginFreezeRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalPath      string
}

type CmdCommitFreezeRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdDeleteFreezeRepo struct {
	WorkflowId uuid.I
}

type CmdBeginUnfreezeRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalPath      string
}

type CmdCommitUnfreezeRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdDeleteUnfreezeRepo struct {
	WorkflowId uuid.I
}

type CmdBeginArchiveRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalPath      string
}

type CmdCommitArchiveRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdDeleteArchiveRepo struct {
	WorkflowId uuid.I
}

type CmdBeginUnarchiveRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalPath      string
}

type CmdCommitUnarchiveRepo struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
}

type CmdDeleteUnarchiveRepo struct {
	WorkflowId uuid.I
}

type CmdSnapshot struct {
	IfStorageReduction bool
}

func (*State) AggregateState()                    {}
func (*CmdBeginDuRoot) AggregateCommand()         {}
func (*CmdCommitDuRoot) AggregateCommand()        {}
func (*CmdDeleteDuRoot) AggregateCommand()        {}
func (*CmdBeginPingRegistry) AggregateCommand()   {}
func (*CmdCommitPingRegistry) AggregateCommand()  {}
func (*CmdBeginSplitRoot) AggregateCommand()      {}
func (*CmdCommitSplitRoot) AggregateCommand()     {}
func (*CmdDeletePingRegistry) AggregateCommand()  {}
func (*CmdDeleteSplitRoot) AggregateCommand()     {}
func (*CmdBeginFreezeRepo) AggregateCommand()     {}
func (*CmdCommitFreezeRepo) AggregateCommand()    {}
func (*CmdDeleteFreezeRepo) AggregateCommand()    {}
func (*CmdBeginUnfreezeRepo) AggregateCommand()   {}
func (*CmdCommitUnfreezeRepo) AggregateCommand()  {}
func (*CmdDeleteUnfreezeRepo) AggregateCommand()  {}
func (*CmdBeginArchiveRepo) AggregateCommand()    {}
func (*CmdCommitArchiveRepo) AggregateCommand()   {}
func (*CmdDeleteArchiveRepo) AggregateCommand()   {}
func (*CmdBeginUnarchiveRepo) AggregateCommand()  {}
func (*CmdCommitUnarchiveRepo) AggregateCommand() {}
func (*CmdDeleteUnarchiveRepo) AggregateCommand() {}
func (*CmdSnapshot) AggregateCommand()            {}

func (s *State) Id() uuid.I        { return s.id }
func (s *State) Vid() ulid.I       { return s.vid }
func (s *State) SetVid(vid ulid.I) { s.vid = vid }

type Behavior struct {
	journal *events.Journal
}
type Event struct{ wfev.Event }

func (Behavior) NewState(id uuid.I) events.State { return &State{id: id} }
func (Behavior) NewEvent() events.Event          { return &Event{} }
func (Behavior) NewAdvancer() events.Advancer    { return &Advancer{} }

// The bools indicate which part of the state has been duplicated.
type Advancer struct {
	state              bool // The state itself.
	activeWorkflows    bool
	completedWorkflows bool
}

func (ev *Event) UnmarshalProto(data []byte) error {
	if err := ev.Event.UnmarshalProto(data); err != nil {
		return err
	}
	switch ev.Event.PbWorkflowEvent().Event {
	default:
		return ErrInvalidEventType
	case pb.WorkflowEvent_EV_SNAPSHOT_BEGIN:
	case pb.WorkflowEvent_EV_SNAPSHOT_END:
	case pb.WorkflowEvent_EV_WORKFLOW_INDEX_SNAPSHOT_STATE:
	case pb.WorkflowEvent_EV_FSO_DU_ROOT_STARTED:
	case pb.WorkflowEvent_EV_FSO_DU_ROOT_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_DU_ROOT_DELETED:
	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_STARTED:
	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_PING_REGISTRY_DELETED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_STARTED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_SPLIT_ROOT_DELETED:
	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_STARTED_2:
	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
	case pb.WorkflowEvent_EV_FSO_FREEZE_REPO_DELETED:
	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
	case pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_DELETED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_DELETED:
	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
	case pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_DELETED:
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

	detachActiveWorkflows := func() {
		if a.activeWorkflows {
			return
		}
		dup := make(map[uuid.I]struct{})
		for k, v := range st.activeWorkflows {
			dup[k] = v
		}
		st.activeWorkflows = dup
		a.activeWorkflows = true
	}

	detachCompletedWorkflows := func() {
		if a.completedWorkflows {
			return
		}
		dup := make(map[uuid.I]struct{})
		for k, v := range st.completedWorkflows {
			dup[k] = v
		}
		st.completedWorkflows = dup
		a.completedWorkflows = true
	}

	st.nEventsSinceSnapshot++

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
	case *wfev.EvSnapshotBegin:
		st.activeWorkflows = make(map[uuid.I]struct{})
		a.activeWorkflows = true

		st.completedWorkflows = make(map[uuid.I]struct{})
		a.completedWorkflows = true

		return st

	case *wfev.EvSnapshotEnd:
		st.nEventsSinceSnapshot = 0
		return st

	case *wfev.EvWorkflowIndexSnapshotState:
		detachActiveWorkflows()
		detachCompletedWorkflows()
		for _, w := range x.DuRoot {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		for _, w := range x.PingRegistry {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		for _, w := range x.SplitRoot {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		for _, w := range x.FreezeRepo {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		for _, w := range x.UnfreezeRepo {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		for _, w := range x.ArchiveRepo {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		for _, w := range x.UnarchiveRepo {
			id := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				st.activeWorkflows[id] = struct{}{}
			} else {
				st.completedWorkflows[id] = struct{}{}
			}
		}
		return st

	case *wfev.EvDuRootStarted:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvDuRootCompleted:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvDuRootDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	case *wfev.EvPingRegistryStarted:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvPingRegistryCompleted:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvPingRegistryDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	case *wfev.EvSplitRootStarted:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvSplitRootCompleted:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvSplitRootDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	case *wfev.EvFreezeRepoStarted2:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvFreezeRepoCompleted2:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvFreezeRepoDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	case *wfev.EvUnfreezeRepoStarted2:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvUnfreezeRepoCompleted2:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvUnfreezeRepoDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	case *wfev.EvArchiveRepoStarted:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvArchiveRepoCompleted:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvArchiveRepoDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	case *wfev.EvUnarchiveRepoStarted:
		detachActiveWorkflows()
		st.activeWorkflows[x.WorkflowId] = struct{}{}
		st.lastCommitted = uuid.Nil
		return st

	case *wfev.EvUnarchiveRepoCompleted:
		detachActiveWorkflows()
		delete(st.activeWorkflows, x.WorkflowId)
		st.lastCommitted = x.WorkflowId
		detachCompletedWorkflows()
		st.completedWorkflows[x.WorkflowId] = struct{}{}
		return st

	case *wfev.EvUnarchiveRepoDeleted:
		detachCompletedWorkflows()
		delete(st.completedWorkflows, x.WorkflowId)
		return st

	default:
		panic("invalid event")
	}
}

func (bh Behavior) Tell(
	s events.State, c events.Command,
) ([]events.Event, error) {
	st := s.(*State)
	switch cmd := c.(type) {
	case *CmdBeginDuRoot:
		return tellBeginDuRoot(st, cmd)
	case *CmdCommitDuRoot:
		return tellCommitDuRoot(st, cmd)
	case *CmdDeleteDuRoot:
		return tellDeleteDuRoot(st, cmd)
	case *CmdBeginPingRegistry:
		return tellBeginPingRegistry(st, cmd)
	case *CmdCommitPingRegistry:
		return tellCommitPingRegistry(st, cmd)
	case *CmdDeletePingRegistry:
		return tellDeletePingRegistry(st, cmd)
	case *CmdBeginSplitRoot:
		return tellBeginSplitRoot(st, cmd)
	case *CmdCommitSplitRoot:
		return tellCommitSplitRoot(st, cmd)
	case *CmdDeleteSplitRoot:
		return tellDeleteSplitRoot(st, cmd)
	case *CmdBeginFreezeRepo:
		return tellBeginFreezeRepo(st, cmd)
	case *CmdCommitFreezeRepo:
		return tellCommitFreezeRepo(st, cmd)
	case *CmdDeleteFreezeRepo:
		return tellDeleteFreezeRepo(st, cmd)
	case *CmdBeginUnfreezeRepo:
		return tellBeginUnfreezeRepo(st, cmd)
	case *CmdCommitUnfreezeRepo:
		return tellCommitUnfreezeRepo(st, cmd)
	case *CmdDeleteUnfreezeRepo:
		return tellDeleteUnfreezeRepo(st, cmd)
	case *CmdBeginArchiveRepo:
		return tellBeginArchiveRepo(st, cmd)
	case *CmdCommitArchiveRepo:
		return tellCommitArchiveRepo(st, cmd)
	case *CmdDeleteArchiveRepo:
		return tellDeleteArchiveRepo(st, cmd)
	case *CmdBeginUnarchiveRepo:
		return tellBeginUnarchiveRepo(st, cmd)
	case *CmdCommitUnarchiveRepo:
		return tellCommitUnarchiveRepo(st, cmd)
	case *CmdDeleteUnarchiveRepo:
		return tellDeleteUnarchiveRepo(st, cmd)
	case *CmdSnapshot:
		return bh.tellSnapshot(st, cmd)
	default:
		return nil, ErrInvalidCommand
	}
}

func tellBeginDuRoot(st *State, cmd *CmdBeginDuRoot) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active, which is a very loose condition.  A stricter check could
	// check the command arguments.  But a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvDuRootStarted{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		GlobalRoot:      cmd.GlobalRoot,
		Host:            cmd.Host,
		HostRoot:        cmd.HostRoot,
	}
	return wfev.NewEvents(st.Vid(), wfev.NewPbDuRootStartedIndex(ev))
}

func tellCommitDuRoot(
	st *State, cmd *CmdCommitDuRoot,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(st.Vid(), wfev.NewPbDuRootCompletedIdRef(
		cmd.WorkflowId, cmd.WorkflowEventId,
	))
}

func tellDeleteDuRoot(
	st *State, cmd *CmdDeleteDuRoot,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbDuRootDeleted(cmd.WorkflowId),
	)
}

func tellBeginPingRegistry(
	st *State, cmd *CmdBeginPingRegistry,
) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active.  Such a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvPingRegistryStarted{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
	}
	return wfev.NewEvents(st.Vid(), wfev.NewPbPingRegistryStartedIndex(ev))
}

func tellCommitPingRegistry(
	st *State, cmd *CmdCommitPingRegistry,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(st.Vid(), wfev.NewPbPingRegistryCompletedIdRef(
		cmd.WorkflowId, cmd.WorkflowEventId,
	))
}

func tellDeletePingRegistry(
	st *State, cmd *CmdDeletePingRegistry,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbPingRegistryDeleted(cmd.WorkflowId),
	)
}

func tellBeginSplitRoot(
	st *State, cmd *CmdBeginSplitRoot,
) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active.  Such a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvSplitRootStarted{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		GlobalRoot:      cmd.GlobalRoot,
		Host:            cmd.Host,
		HostRoot:        cmd.HostRoot,
	}
	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootStartedIndex(ev),
	)
}

func tellCommitSplitRoot(
	st *State, cmd *CmdCommitSplitRoot,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootCompletedIdRef(
			cmd.WorkflowId, cmd.WorkflowEventId,
		),
	)
}

func tellDeleteSplitRoot(
	st *State, cmd *CmdDeleteSplitRoot,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbSplitRootDeleted(cmd.WorkflowId),
	)
}

func tellBeginFreezeRepo(
	st *State, cmd *CmdBeginFreezeRepo,
) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active.  Such a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvFreezeRepoStarted2{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		RepoGlobalPath:  cmd.GlobalPath,
	}
	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbFreezeRepoStarted2Index(ev),
	)
}

func tellCommitFreezeRepo(
	st *State, cmd *CmdCommitFreezeRepo,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbFreezeRepoCompleted2IdRef(
			cmd.WorkflowId, cmd.WorkflowEventId,
		),
	)
}

func tellDeleteFreezeRepo(
	st *State, cmd *CmdDeleteFreezeRepo,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbFreezeRepoDeleted(cmd.WorkflowId),
	)
}

func tellBeginUnfreezeRepo(
	st *State, cmd *CmdBeginUnfreezeRepo,
) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active.  Such a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvUnfreezeRepoStarted2{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		RepoGlobalPath:  cmd.GlobalPath,
	}
	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbUnfreezeRepoStarted2Index(ev),
	)
}

func tellCommitUnfreezeRepo(
	st *State, cmd *CmdCommitUnfreezeRepo,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbUnfreezeRepoCompleted2IdRef(
			cmd.WorkflowId, cmd.WorkflowEventId,
		),
	)
}

func tellDeleteUnfreezeRepo(
	st *State, cmd *CmdDeleteUnfreezeRepo,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbUnfreezeRepoDeleted(cmd.WorkflowId),
	)
}

func tellBeginArchiveRepo(
	st *State, cmd *CmdBeginArchiveRepo,
) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active.  Such a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvArchiveRepoStarted{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		RepoGlobalPath:  cmd.GlobalPath,
	}
	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoStartedIndex(ev),
	)
}

func tellCommitArchiveRepo(
	st *State, cmd *CmdCommitArchiveRepo,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoCompletedIdRef(
			cmd.WorkflowId, cmd.WorkflowEventId,
		),
	)
}

func tellDeleteArchiveRepo(
	st *State, cmd *CmdDeleteArchiveRepo,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbArchiveRepoDeleted(cmd.WorkflowId),
	)
}

func tellBeginUnarchiveRepo(
	st *State, cmd *CmdBeginUnarchiveRepo,
) ([]events.Event, error) {
	// The command is considered idempotent if the workflow is already
	// active.  Such a loose check seems sufficient.
	if st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, nil // idempotent
	}

	// XXX Validate command fields.

	ev := &wfev.EvUnarchiveRepoStarted{
		WorkflowId:      cmd.WorkflowId,
		WorkflowEventId: cmd.WorkflowEventId,
		RepoGlobalPath:  cmd.GlobalPath,
	}
	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbUnarchiveRepoStartedIndex(ev),
	)
}

func tellCommitUnarchiveRepo(
	st *State, cmd *CmdCommitUnarchiveRepo,
) ([]events.Event, error) {
	if cmd.WorkflowId == st.lastCommitted {
		return nil, nil // idempotent
	}
	if !st.isActiveWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbUnarchiveRepoCompletedIdRef(
			cmd.WorkflowId, cmd.WorkflowEventId,
		),
	)
}

func tellDeleteUnarchiveRepo(
	st *State, cmd *CmdDeleteUnarchiveRepo,
) ([]events.Event, error) {
	if !st.isCompletedWorkflow(cmd.WorkflowId) {
		return nil, ErrUnknownWorkflow
	}

	return wfev.NewEvents(
		st.Vid(),
		wfev.NewPbUnarchiveRepoDeleted(cmd.WorkflowId),
	)
}

func (bh Behavior) tellSnapshot(
	st *State, cmd *CmdSnapshot,
) ([]events.Event, error) {
	if st.nEventsSinceSnapshot == 0 {
		return nil, nil // Idempotent if latest is snapshot.
	}

	// Avoid big snapshots.  GC needs to be tuned such that enough expired
	// workflows are aborted and completed workflows deleted before taking
	// a snapshot.
	nWorkflows := len(st.activeWorkflows) + len(st.completedWorkflows)
	if nWorkflows > ConfigMaxSnapshotWorkflows {
		return nil, ErrTooManyWorkflows
	}

	if cmd.IfStorageReduction {
		if st.nEventsSinceSnapshot < ConfigMinEventsBetweenSnapshots {
			return nil, ErrSmallStorageReduction
		}

		snapCost := ConfigSnapshotBaseCost +
			nWorkflows*ConfigSnapshotWorkflowCost
		evsCost := ConfigEventConst * st.nEventsSinceSnapshot
		if snapCost >= evsCost {
			return nil, ErrSmallStorageReduction
		}
	}

	evs, err := snapshot(bh.journal, st.Id())
	if err != nil {
		return nil, err
	}

	return wfev.NewEvents(st.Vid(), evs...)
}

type Indexes struct {
	engine *events.Engine
}

func New(journal *events.Journal) *Indexes {
	journal.SetTrimPolicy(&trimPolicy{})
	return &Indexes{
		engine: events.NewEngine(journal, Behavior{journal: journal}),
	}
}

func (r *Indexes) FindId(id uuid.I) (*State, error) {
	st, err := r.engine.FindId(id)
	if err != nil {
		return nil, err
	}
	// Uninitialized is not an error.  `Indexes` may be used with an
	// ephemeral journal, which may be reset at any time.
	return st.(*State), nil
}

func (r *Indexes) BeginDuRoot(
	id uuid.I, vid ulid.I, cmd *CmdBeginDuRoot,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitDuRoot(
	id uuid.I, vid ulid.I, cmd *CmdCommitDuRoot,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeleteDuRoot(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteDuRoot{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) BeginPingRegistry(
	id uuid.I, vid ulid.I, cmd *CmdBeginPingRegistry,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitPingRegistry(
	id uuid.I, vid ulid.I, cmd *CmdCommitPingRegistry,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeletePingRegistry(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeletePingRegistry{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) BeginSplitRoot(
	id uuid.I, vid ulid.I, cmd *CmdBeginSplitRoot,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitSplitRoot(
	id uuid.I, vid ulid.I, cmd *CmdCommitSplitRoot,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeleteSplitRoot(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteSplitRoot{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) BeginFreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginFreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitFreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitFreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeleteFreezeRepo(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteFreezeRepo{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) BeginUnfreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginUnfreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitUnfreezeRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitUnfreezeRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeleteUnfreezeRepo(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteUnfreezeRepo{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) BeginArchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginArchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitArchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitArchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeleteArchiveRepo(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteArchiveRepo{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) BeginUnarchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdBeginUnarchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) CommitUnarchiveRepo(
	id uuid.I, vid ulid.I, cmd *CmdCommitUnarchiveRepo,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, cmd)
}

func (r *Indexes) DeleteUnarchiveRepo(
	id uuid.I, vid ulid.I, workflowId uuid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdDeleteUnarchiveRepo{
		WorkflowId: workflowId,
	})
}

func (r *Indexes) Snapshot(
	id uuid.I, vid ulid.I,
) (ulid.I, error) {
	return r.engine.TellIdVid(id, vid, &CmdSnapshot{
		IfStorageReduction: true,
	})
}

func (st *State) isActiveWorkflow(id uuid.I) bool {
	if st.activeWorkflows == nil {
		return false
	}
	_, ok := st.activeWorkflows[id]
	return ok
}

func (st *State) isCompletedWorkflow(id uuid.I) bool {
	if st.completedWorkflows == nil {
		return false
	}
	_, ok := st.completedWorkflows[id]
	return ok
}
