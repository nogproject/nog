package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type indexActivity struct {
	lg             Logger
	registry       string
	conn           *grpc.ClientConn
	sysRPCCreds    grpc.CallOption
	workflowEngine grpcentities.RegistryWorkflowEngine
}

type indexView struct {
	vid       ulid.I
	ping      uuidSlice
	split     uuidSlice
	freeze    uuidSlice
	unfreeze  uuidSlice
	archive   uuidSlice
	unarchive uuidSlice
}

type uuidSlice []uuid.I

func (ids uuidSlice) delete(id uuid.I) uuidSlice {
	for i, v := range ids {
		if v == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func (a *indexActivity) ProcessRegistryWorkflowIndexEvents(
	ctx context.Context,
	registry string,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowIndexEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		idx := indexView{}
		if err := wfstreams.LoadRegistryWorkflowIndexEventsNoBlock(
			stream, &idx,
		); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		if err := a.processView(ctx, idx); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		tail = idx.vid
	}

	return wfstreams.WatchRegistryWorkflowIndexEvents(
		ctx, tail, stream, a, nil,
	)
}

func (idx *indexView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	idx.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvSnapshotBegin:
		idx.ping = nil
		idx.split = nil
		idx.freeze = nil
		idx.unfreeze = nil
		idx.archive = nil
		idx.unarchive = nil
		return nil

	case *wfevents.EvWorkflowIndexSnapshotState:
		for _, w := range x.PingRegistry {
			if w.CompletedWorkflowEventId == ulid.Nil {
				idx.ping = append(idx.ping, w.WorkflowId)
			}
		}
		for _, w := range x.SplitRoot {
			if w.CompletedWorkflowEventId == ulid.Nil {
				idx.split = append(idx.split, w.WorkflowId)
			}
		}
		for _, w := range x.FreezeRepo {
			if w.CompletedWorkflowEventId == ulid.Nil {
				idx.freeze = append(idx.freeze, w.WorkflowId)
			}
		}
		for _, w := range x.UnfreezeRepo {
			if w.CompletedWorkflowEventId == ulid.Nil {
				idx.unfreeze = append(idx.unfreeze, w.WorkflowId)
			}
		}
		for _, w := range x.ArchiveRepo {
			if w.CompletedWorkflowEventId == ulid.Nil {
				idx.archive = append(idx.archive, w.WorkflowId)
			}
		}
		for _, w := range x.UnarchiveRepo {
			if w.CompletedWorkflowEventId == ulid.Nil {
				idx.unarchive = append(idx.unarchive, w.WorkflowId)
			}
		}
		return nil

	case *wfevents.EvPingRegistryStarted:
		idx.ping = append(idx.ping, x.WorkflowId)
		return nil

	case *wfevents.EvPingRegistryCompleted:
		idx.ping = idx.ping.delete(x.WorkflowId)
		return nil

	case *wfevents.EvSplitRootStarted:
		idx.split = append(idx.split, x.WorkflowId)
		return nil

	case *wfevents.EvSplitRootCompleted:
		idx.split = idx.split.delete(x.WorkflowId)
		return nil

	case *wfevents.EvFreezeRepoStarted2:
		idx.freeze = append(idx.freeze, x.WorkflowId)
		return nil

	case *wfevents.EvFreezeRepoCompleted2:
		idx.freeze = idx.freeze.delete(x.WorkflowId)
		return nil

	case *wfevents.EvUnfreezeRepoStarted2:
		idx.unfreeze = append(idx.unfreeze, x.WorkflowId)
		return nil

	case *wfevents.EvUnfreezeRepoCompleted2:
		idx.unfreeze = idx.unfreeze.delete(x.WorkflowId)
		return nil

	case *wfevents.EvArchiveRepoStarted:
		idx.archive = append(idx.archive, x.WorkflowId)
		return nil

	case *wfevents.EvArchiveRepoCompleted:
		idx.archive = idx.archive.delete(x.WorkflowId)
		return nil

	case *wfevents.EvUnarchiveRepoStarted:
		idx.unarchive = append(idx.unarchive, x.WorkflowId)
		return nil

	case *wfevents.EvUnarchiveRepoCompleted:
		idx.unarchive = idx.unarchive.delete(x.WorkflowId)
		return nil

	default: // Silently ignore other events.
		return nil
	}
}

func (a *indexActivity) processView(
	ctx context.Context,
	idx indexView,
) error {
	for _, id := range idx.ping {
		if err := a.runPingRegistryWorkflow(ctx, id); err != nil {
			return err
		}
	}

	for _, id := range idx.split {
		if err := a.runSplitRootWorkflow(ctx, id); err != nil {
			return err
		}
	}

	for _, id := range idx.freeze {
		if err := a.runFreezeRepoWorkflow(ctx, id); err != nil {
			return err
		}
	}

	for _, id := range idx.unfreeze {
		if err := a.runUnfreezeRepoWorkflow(ctx, id); err != nil {
			return err
		}
	}

	for _, id := range idx.archive {
		if err := a.runArchiveRepoWorkflow(ctx, id); err != nil {
			return err
		}
	}

	for _, id := range idx.unarchive {
		if err := a.runUnarchiveRepoWorkflow(ctx, id); err != nil {
			return err
		}
	}

	return nil
}

func (a *indexActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	switch x := ev.(type) {
	case *wfevents.EvPingRegistryStarted:
		return a.doRetry(a.runPingRegistryWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvSplitRootStarted:
		return a.doRetry(a.runSplitRootWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvFreezeRepoStarted2:
		return a.doRetry(a.runFreezeRepoWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvUnfreezeRepoStarted2:
		return a.doRetry(a.runUnfreezeRepoWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvArchiveRepoStarted:
		return a.doRetry(a.runArchiveRepoWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvUnarchiveRepoStarted:
		return a.doRetry(a.runUnarchiveRepoWorkflow(ctx, x.WorkflowId))

	// Silently ignore other events.  In particular, there is nothing to do
	// on:
	//
	//  - `EvPingRegistryCompleted`, `EvSplitRootCompleted`: the workflow
	//    activity ran to completion and handled all the details;
	//  - `EvWorkflowIndexSnapshotState`: the start events must have been
	//    observed before if a snapshot is observed during watch.
	//
	default:
		return a.doContinue()
	}
}

// Run ping-registry workflows concurrently.
func (a *indexActivity) runPingRegistryWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&pingRegistryWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			registry:    a.registry,
		},
	)
}

// Run split-root workflows concurrently.
func (a *indexActivity) runSplitRootWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&splitRootWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			registry:    a.registry,
		},
	)
}

// Run freeze-repo workflows concurrently.
func (a *indexActivity) runFreezeRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&freezeRepoWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			registry:    a.registry,
		},
	)
}

// Run unfreeze-repo workflows concurrently.
func (a *indexActivity) runUnfreezeRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&unfreezeRepoWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			registry:    a.registry,
		},
	)
}

// Run archive-repo workflows concurrently.
func (a *indexActivity) runArchiveRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&archiveRepoWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			registry:    a.registry,
		},
	)
}

// Run unarchive-repo workflows concurrently.
func (a *indexActivity) runUnarchiveRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&unarchiveRepoWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			registry:    a.registry,
		},
	)
}

func (a *indexActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *indexActivity) doRetry(err error) (bool, error) {
	return false, err
}
