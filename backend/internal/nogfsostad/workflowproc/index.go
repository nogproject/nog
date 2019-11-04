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
	lg                 Logger
	registry           string
	prefixes           []string
	conn               *grpc.ClientConn
	sysRPCCreds        grpc.CallOption
	workflowEngine     grpcentities.RegistryWorkflowEngine
	repoProc           RepoProcessor
	aclPropagator      AclPropagator
	privs              Privileges
	archiveRepoSpool   string
	unarchiveRepoSpool string
}

type indexView struct {
	vid       ulid.I
	prefixes  []string
	du        uuidSlice
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
		idx.du = nil
		idx.ping = nil
		idx.split = nil
		idx.freeze = nil
		idx.unfreeze = nil
		idx.archive = nil
		idx.unarchive = nil
		return nil

	case *wfevents.EvWorkflowIndexSnapshotState:
		idx.loadSnapshot(x)
		return nil

	case *wfevents.EvDuRootStarted:
		if pathIsEqualOrBelowPrefixAny(x.GlobalRoot, idx.prefixes) {
			idx.du = append(idx.du, x.WorkflowId)
		}
		return nil

	case *wfevents.EvDuRootCompleted:
		idx.du = idx.du.delete(x.WorkflowId)
		return nil

	case *wfevents.EvPingRegistryStarted:
		idx.ping = append(idx.ping, x.WorkflowId)
		return nil

	case *wfevents.EvPingRegistryCompleted:
		idx.ping = idx.ping.delete(x.WorkflowId)
		return nil

	case *wfevents.EvSplitRootStarted:
		if pathIsEqualOrBelowPrefixAny(x.GlobalRoot, idx.prefixes) {
			idx.split = append(idx.split, x.WorkflowId)
		}
		return nil

	case *wfevents.EvSplitRootCompleted:
		idx.split = idx.split.delete(x.WorkflowId)
		return nil

	case *wfevents.EvFreezeRepoStarted2:
		if pathIsEqualOrBelowPrefixAny(
			x.RepoGlobalPath, idx.prefixes,
		) {
			idx.freeze = append(idx.freeze, x.WorkflowId)
		}
		return nil

	case *wfevents.EvFreezeRepoCompleted2:
		idx.freeze = idx.freeze.delete(x.WorkflowId)
		return nil

	case *wfevents.EvUnfreezeRepoStarted2:
		if pathIsEqualOrBelowPrefixAny(
			x.RepoGlobalPath, idx.prefixes,
		) {
			idx.unfreeze = append(idx.unfreeze, x.WorkflowId)
		}
		return nil

	case *wfevents.EvUnfreezeRepoCompleted2:
		idx.unfreeze = idx.unfreeze.delete(x.WorkflowId)
		return nil

	case *wfevents.EvArchiveRepoStarted:
		if pathIsEqualOrBelowPrefixAny(
			x.RepoGlobalPath, idx.prefixes,
		) {
			idx.archive = append(idx.archive, x.WorkflowId)
		}
		return nil

	case *wfevents.EvArchiveRepoCompleted:
		idx.archive = idx.archive.delete(x.WorkflowId)
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
	for _, w := range x.DuRoot {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		if !pathIsEqualOrBelowPrefixAny(w.GlobalRoot, idx.prefixes) {
			continue
		}
		idx.du = append(idx.du, w.WorkflowId)
	}

	for _, w := range x.PingRegistry {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		idx.ping = append(idx.ping, w.WorkflowId)
	}

	for _, w := range x.SplitRoot {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		if !pathIsEqualOrBelowPrefixAny(w.GlobalRoot, idx.prefixes) {
			continue
		}
		idx.split = append(idx.split, w.WorkflowId)
	}

	for _, w := range x.FreezeRepo {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		if !pathIsEqualOrBelowPrefixAny(w.GlobalPath, idx.prefixes) {
			continue
		}
		idx.freeze = append(idx.freeze, w.WorkflowId)
	}

	for _, w := range x.UnfreezeRepo {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		if !pathIsEqualOrBelowPrefixAny(w.GlobalPath, idx.prefixes) {
			continue
		}
		idx.unfreeze = append(idx.unfreeze, w.WorkflowId)
	}

	for _, w := range x.ArchiveRepo {
		if w.CompletedWorkflowEventId != ulid.Nil {
			continue
		}
		if !pathIsEqualOrBelowPrefixAny(w.GlobalPath, idx.prefixes) {
			continue
		}
		idx.archive = append(idx.archive, w.WorkflowId)
	}

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
	for _, id := range idx.du {
		if err := a.runDuRootWorkflow(ctx, id); err != nil {
			return err
		}
	}

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
	case *wfevents.EvDuRootStarted:
		if !pathIsEqualOrBelowPrefixAny(x.GlobalRoot, a.prefixes) {
			return a.doContinue()
		}
		return a.doRetry(a.runDuRootWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvPingRegistryStarted:
		return a.doRetry(a.runPingRegistryWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvSplitRootStarted:
		if !pathIsEqualOrBelowPrefixAny(x.GlobalRoot, a.prefixes) {
			return a.doContinue()
		}
		return a.doRetry(a.runSplitRootWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvFreezeRepoStarted2:
		if !pathIsEqualOrBelowPrefixAny(x.RepoGlobalPath, a.prefixes) {
			return a.doContinue()
		}
		return a.doRetry(a.runFreezeRepoWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvUnfreezeRepoStarted2:
		if !pathIsEqualOrBelowPrefixAny(x.RepoGlobalPath, a.prefixes) {
			return a.doContinue()
		}
		return a.doRetry(a.runUnfreezeRepoWorkflow(ctx, x.WorkflowId))

	case *wfevents.EvArchiveRepoStarted:
		if !pathIsEqualOrBelowPrefixAny(x.RepoGlobalPath, a.prefixes) {
			return a.doContinue()
		}
		return a.doRetry(a.runArchiveRepoWorkflow(ctx, x.WorkflowId))

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

func (a *indexActivity) runDuRootWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run workflow and wait for completion.
	done := make(chan struct{})
	if err := a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&duRootWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			done:        done,
		},
	); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (a *indexActivity) runPingRegistryWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run workflow and wait for completion.
	done := make(chan struct{})
	if err := a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&pingRegistryWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			done:        done,
		},
	); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (a *indexActivity) runSplitRootWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run workflow and wait for completion.
	done := make(chan struct{})
	if err := a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&splitRootWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			done:        done,
		},
	); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (a *indexActivity) runFreezeRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run freeze-repo workflow concurrently.  The activity serializes
	// per-repo access if necessary.
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&freezeRepoWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			repoProc:    a.repoProc,
		},
	)
}

func (a *indexActivity) runUnfreezeRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run unfreeze-repo workflow concurrently.  The activity serializes
	// per-repo access if necessary.
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&unfreezeRepoWorkflowActivity{
			lg:          a.lg,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
			repoProc:    a.repoProc,
		},
	)
}

func (a *indexActivity) runArchiveRepoWorkflow(
	ctx context.Context,
	workflowId uuid.I,
) error {
	// Run archive-repo workflow concurrently.  The activity serializes
	// per-repo access if necessary.
	return a.workflowEngine.StartRegistryWorkflowActivity(
		a.registry, workflowId,
		&archiveRepoWorkflowActivity{
			lg:               a.lg,
			conn:             a.conn,
			sysRPCCreds:      a.sysRPCCreds,
			repoProc:         a.repoProc,
			aclPropagator:    a.aclPropagator,
			privs:            a.privs,
			archiveRepoSpool: a.archiveRepoSpool,
		},
	)
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
			lg:                 a.lg,
			conn:               a.conn,
			sysRPCCreds:        a.sysRPCCreds,
			repoProc:           a.repoProc,
			aclPropagator:      a.aclPropagator,
			privs:              a.privs,
			unarchiveRepoSpool: a.unarchiveRepoSpool,
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
