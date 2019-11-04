package workflowproc

import (
	"context"
	"strings"

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
	prefixes       []string
	expectedHosts  map[string]struct{}
	capPath        string
	conn           *grpc.ClientConn
	sysRPCCreds    grpc.CallOption
	workflowEngine grpcentities.RegistryWorkflowEngine
	tarttLimiter   Limiter
}

type indexView struct {
	vid       ulid.I
	prefixes  []string
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
		idx := indexView{
			prefixes: a.prefixes,
		}
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
		idx.unarchive = nil
		return nil

	case *wfevents.EvWorkflowIndexSnapshotState:
		idx.loadSnapshot(x)
		return nil

	case *wfevents.EvUnarchiveRepoStarted:
		if pathIsEqualOrBelowPrefixAny(
			x.RepoGlobalPath, idx.prefixes,
		) {
			idx.unarchive = append(idx.unarchive, x.WorkflowId)
		}
		return nil

	case *wfevents.EvUnarchiveRepoCompleted:
		idx.unarchive = idx.unarchive.delete(x.WorkflowId)
		return nil

	default: // Silently ignore other events.
		return nil
	}
}

func (idx *indexView) loadSnapshot(
	x *wfevents.EvWorkflowIndexSnapshotState,
) {
	for _, w := range x.UnarchiveRepo {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		if !pathIsEqualOrBelowPrefixAny(w.GlobalPath, idx.prefixes) {
			continue
		}
		idx.unarchive = append(idx.unarchive, w.WorkflowId)
	}
}

func (a *indexActivity) processView(
	ctx context.Context,
	idx indexView,
) error {
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
	case *wfevents.EvUnarchiveRepoStarted:
		if !pathIsEqualOrBelowPrefixAny(x.RepoGlobalPath, a.prefixes) {
			return a.doContinue()
		}
		return a.doRetry(a.runUnarchiveRepoWorkflow(ctx, x.WorkflowId))

	// Silently ignore other events.  In particular, there is nothing to do
	// on:
	//
	//  - `EvDuRootCompleted, `EvPingRegistryCompleted`,
	//    `EvSplitRootCompleted`: the workflow activity ran to completion
	//    and handled all the details;
	//  - `EvWorkflowIndexSnapshotState`: the start events must have been
	//    observed before if a snapshot is observed during watch.
	//
	default:
		return a.doContinue()
	}
}

func (a *indexActivity) runUnarchiveRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run unarchive-repo workflow concurrently.  The activity serializes
	// per-repo access if necessary.
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&unarchiveRepoWorkflowActivity{
			lg:            a.lg,
			conn:          a.conn,
			sysRPCCreds:   a.sysRPCCreds,
			expectedHosts: a.expectedHosts,
			capPath:       a.capPath,
			tarttLimiter:  a.tarttLimiter,
		},
	)
}

func (a *indexActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *indexActivity) doRetry(err error) (bool, error) {
	return false, err
}

// `prefixes` with trailing slash.
func pathIsEqualOrBelowPrefixAny(path string, prefixes []string) bool {
	path = ensureTrailingSlash(path)
	for _, pfx := range prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}
