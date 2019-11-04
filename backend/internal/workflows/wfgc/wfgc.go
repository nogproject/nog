package wfgc

import (
	"context"
	"time"

	"github.com/nogproject/nog/backend/internal/events"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/internal/workflows/durootwf"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/internal/workflows/pingregistrywf"
	"github.com/nogproject/nog/backend/internal/workflows/splitrootwf"
	"github.com/nogproject/nog/backend/internal/workflows/wfindexes"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const (
	NsFsoRegistryEphemeralWorkflows = "fsoregephwfl"
)

const (
	ConfigDuRootMaxActiveDays = 1
	ConfigDuRootMaxActive     = ConfigDuRootMaxActiveDays * 24 * time.Hour

	ConfigDuRootDeleteAfterDays = 5
	ConfigDuRootDeleteAfter     = ConfigDuRootDeleteAfterDays * 24 * time.Hour

	ConfigPingRegistryMaxActiveDays = 1
	ConfigPingRegistryMaxActive     = ConfigPingRegistryMaxActiveDays * 24 * time.Hour

	ConfigPingRegistryDeleteAfterDays = 5
	ConfigPingRegistryDeleteAfter     = ConfigPingRegistryDeleteAfterDays * 24 * time.Hour

	ConfigSplitRootMaxActiveDays = 5
	ConfigSplitRootMaxActive     = ConfigSplitRootMaxActiveDays * 24 * time.Hour

	ConfigSplitRootDeleteAfterDays = 30
	ConfigSplitRootDeleteAfter     = ConfigSplitRootDeleteAfterDays * 24 * time.Hour
)

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
}

type GarbageCollector struct {
	lg                    Logger
	registries            []string
	names                 *shorteruuid.Names
	ephWorkflowsJ         *events.Journal
	workflowIndexes       *wfindexes.Indexes
	duRootWorkflows       *durootwf.Workflows
	pingRegistryWorkflows *pingregistrywf.Workflows
	splitRootWorkflows    *splitrootwf.Workflows
}

func New(
	lg Logger,
	registries []string,
	names *shorteruuid.Names,
	ephWorkflowsJ *events.Journal,
	workflowIndexes *wfindexes.Indexes,
	duRootWorkflows *durootwf.Workflows,
	pingRegistryWorkflows *pingregistrywf.Workflows,
	splitRootWorkflows *splitrootwf.Workflows,
) *GarbageCollector {
	return &GarbageCollector{
		lg:                    lg,
		registries:            registries,
		names:                 names,
		ephWorkflowsJ:         ephWorkflowsJ,
		workflowIndexes:       workflowIndexes,
		duRootWorkflows:       duRootWorkflows,
		pingRegistryWorkflows: pingRegistryWorkflows,
		splitRootWorkflows:    splitRootWorkflows,
	}
}

func (gc *GarbageCollector) Gc(ctx context.Context) error {
	// Use a fixed time for the entire gc run.  It feels potentially more
	// robust, although it should not matter here.
	now := time.Now()

	for _, r := range gc.registries {
		if err := gc.gcOne(ctx, r, now); err != nil {
			return err
		}
	}
	return nil
}

func (gc *GarbageCollector) gcOne(
	ctx context.Context,
	reg string,
	now time.Time,
) error {
	idxId := gc.names.UUID(NsFsoRegistryEphemeralWorkflows, reg)

	idx, err := loadIndexView(ctx, gc.ephWorkflowsJ, idxId)
	if err != nil {
		return err
	}

	if err := gc.gcDuRoot(ctx, reg, idxId, idx, now); err != nil {
		return err
	}

	if err := gc.gcPingRegistry(ctx, reg, idxId, idx, now); err != nil {
		return err
	}

	if err := gc.gcSplitRoot(ctx, reg, idxId, idx, now); err != nil {
		return err
	}

	if err := gc.snapshot(ctx, reg, idxId); err != nil {
		return err
	}

	return nil
}

func (gc *GarbageCollector) gcDuRoot(
	ctx context.Context,
	reg string,
	idxId uuid.I,
	idx *indexView,
	now time.Time,
) error {
	nWorkflows, err := gc.gcDuRootActive(
		ctx, idxId, idx.duRootActive, now,
	)
	if err != nil {
		return err
	}
	if nWorkflows == 0 {
		gc.lg.Infow(
			"GC found no expired du-root workflows.",
			"registry", reg,
		)
	} else {
		gc.lg.Infow(
			"GC aborted expired du-root workflows.",
			"registry", reg,
			"nWorkflows", nWorkflows,
		)
	}

	nWorkflows, err = gc.gcDuRootCompleted(
		ctx, idxId, idx.duRootCompleted, now,
	)
	if err != nil {
		return err
	}
	if nWorkflows == 0 {
		gc.lg.Infow(
			"GC found no old completed du-root workflows.",
			"registry", reg,
		)
	} else {
		gc.lg.Infow(
			"GC deleted du-root workflows.",
			"registry", reg,
			"nWorkflows", nWorkflows,
		)
	}

	return nil
}

func (gc *GarbageCollector) gcPingRegistry(
	ctx context.Context,
	reg string,
	idxId uuid.I,
	idx *indexView,
	now time.Time,
) error {
	nWorkflows, err := gc.gcPingRegistryActive(
		ctx, idxId, idx.pingRegistryActive, now,
	)
	if err != nil {
		return err
	}
	if nWorkflows == 0 {
		gc.lg.Infow(
			"GC found no expired ping-registry workflows.",
			"registry", reg,
		)
	} else {
		gc.lg.Infow(
			"GC aborted expired ping-registry workflows.",
			"registry", reg,
			"nWorkflows", nWorkflows,
		)
	}

	nWorkflows, err = gc.gcPingRegistryCompleted(
		ctx, idxId, idx.pingRegistryCompleted, now,
	)
	if err != nil {
		return err
	}
	if nWorkflows == 0 {
		gc.lg.Infow(
			"GC found no old completed ping-registry workflows.",
			"registry", reg,
		)
	} else {
		gc.lg.Infow(
			"GC deleted ping-registry workflows.",
			"registry", reg,
			"nWorkflows", nWorkflows,
		)
	}

	return nil
}

func (gc *GarbageCollector) gcSplitRoot(
	ctx context.Context,
	reg string,
	idxId uuid.I,
	idx *indexView,
	now time.Time,
) error {
	nWorkflows, err := gc.gcSplitRootActive(
		ctx, idxId, idx.splitRootActive, now,
	)
	if err != nil {
		return err
	}
	if nWorkflows == 0 {
		gc.lg.Infow(
			"GC found no expired split-root workflows.",
			"registry", reg,
		)
	} else {
		gc.lg.Infow(
			"GC aborted expired split-root workflows.",
			"registry", reg,
			"nWorkflows", nWorkflows,
		)
	}

	nWorkflows, err = gc.gcSplitRootCompleted(
		ctx, idxId, idx.splitRootCompleted, now,
	)
	if err != nil {
		return err
	}
	if nWorkflows == 0 {
		gc.lg.Infow(
			"GC found no old completed split-root workflows.",
			"registry", reg,
		)
	} else {
		gc.lg.Infow(
			"GC deleted split-root workflows.",
			"registry", reg,
			"nWorkflows", nWorkflows,
		)
	}

	return nil
}

type indexView struct {
	vid                   ulid.I
	duRootActive          map[uuid.I]ulid.I
	duRootCompleted       map[uuid.I]ulid.I
	pingRegistryActive    map[uuid.I]ulid.I
	pingRegistryCompleted map[uuid.I]ulid.I
	splitRootActive       map[uuid.I]ulid.I
	splitRootCompleted    map[uuid.I]ulid.I
}

func (idx *indexView) resetState() {
	idx.duRootActive = make(map[uuid.I]ulid.I)
	idx.duRootCompleted = make(map[uuid.I]ulid.I)
	idx.pingRegistryActive = make(map[uuid.I]ulid.I)
	idx.pingRegistryCompleted = make(map[uuid.I]ulid.I)
	idx.splitRootActive = make(map[uuid.I]ulid.I)
	idx.splitRootCompleted = make(map[uuid.I]ulid.I)
}

func loadIndexView(
	ctx context.Context, ephWorkflowsJ *events.Journal, idxId uuid.I,
) (*indexView, error) {
	idx := &indexView{}
	idx.resetState()

	iter := ephWorkflowsJ.Find(idxId, events.EventEpoch)
	iterClose := func() error {
		if iter == nil {
			return nil
		}
		err := iter.Close()
		iter = nil
		return err
	}
	defer func() { _ = iterClose() }()

	// `Next(&ev)` validates the protobuf, so that
	// `MustParsePbWorkflowEvent()` cannot panic.
	var ev wfevents.Event
	for iter.Next(&ev) {
		vid := ev.Id()
		wfev := wfevents.MustParsePbWorkflowEvent(ev.PbWorkflowEvent())
		if err := idx.loadWorkflowEvent(vid, wfev); err != nil {
			return nil, err
		}
	}
	if err := iterClose(); err != nil {
		return nil, err
	}

	return idx, nil
}

func (idx *indexView) loadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	idx.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvSnapshotBegin:
		idx.resetState()
		return nil

	case *wfevents.EvWorkflowIndexSnapshotState:
		for _, w := range x.DuRoot {
			wId := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				wVid := w.StartedWorkflowEventId
				idx.duRootActive[wId] = wVid
			} else {
				wVid := w.CompletedWorkflowEventId
				idx.duRootCompleted[wId] = wVid
			}
		}
		for _, w := range x.PingRegistry {
			wId := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				wVid := w.StartedWorkflowEventId
				idx.pingRegistryActive[wId] = wVid
			} else {
				wVid := w.CompletedWorkflowEventId
				idx.pingRegistryCompleted[wId] = wVid
			}
		}
		for _, w := range x.SplitRoot {
			wId := w.WorkflowId
			if w.CompletedWorkflowEventId == ulid.Nil {
				wVid := w.StartedWorkflowEventId
				idx.splitRootActive[wId] = wVid
			} else {
				wVid := w.CompletedWorkflowEventId
				idx.splitRootCompleted[wId] = wVid
			}
		}
		return nil

	case *wfevents.EvDuRootStarted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.duRootActive[xId] = xVid
		return nil

	case *wfevents.EvDuRootCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		delete(idx.duRootActive, xId)
		idx.duRootCompleted[xId] = xVid
		return nil

	case *wfevents.EvDuRootDeleted:
		xId := x.WorkflowId
		delete(idx.duRootCompleted, xId)
		return nil

	case *wfevents.EvPingRegistryStarted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.pingRegistryActive[xId] = xVid
		return nil

	case *wfevents.EvPingRegistryCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		delete(idx.pingRegistryActive, xId)
		idx.pingRegistryCompleted[xId] = xVid
		return nil

	case *wfevents.EvPingRegistryDeleted:
		xId := x.WorkflowId
		delete(idx.pingRegistryCompleted, xId)
		return nil

	case *wfevents.EvSplitRootStarted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.splitRootActive[xId] = xVid
		return nil

	case *wfevents.EvSplitRootCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		delete(idx.splitRootActive, xId)
		idx.splitRootCompleted[xId] = xVid
		return nil

	case *wfevents.EvSplitRootDeleted:
		xId := x.WorkflowId
		delete(idx.splitRootCompleted, xId)
		return nil

	default: // Silently ignore other events.
		return nil
	}
}

func (gc *GarbageCollector) gcDuRootActive(
	ctx context.Context,
	idxId uuid.I,
	duRootActive map[uuid.I]ulid.I,
	now time.Time,
) (int, error) {
	n := 0
	cutoff := now.Add(-ConfigDuRootMaxActive)
	for wfId, wfVid := range duRootActive {
		startTime := ulid.Time(wfVid)
		if startTime.Before(cutoff) {
			if err := gc.expireDuRoot(
				ctx, idxId, wfId,
			); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (gc *GarbageCollector) expireDuRoot(
	ctx context.Context,
	idxId uuid.I,
	wfId uuid.I,
) error {
	wfVid, err := gc.duRootWorkflows.AbortExpired(
		wfId, durootwf.RetryNoVC,
	)
	if err != nil {
		return err
	}

	idxVid, err := gc.workflowIndexes.CommitDuRoot(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitDuRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
		},
	)
	if err != nil {
		return err
	}

	wfVid2, err := gc.duRootWorkflows.End(wfId, wfVid)
	if err != nil {
		return err
	}

	gc.lg.Infow(
		"GC aborted expired du-root workflow.",
		"workflowId", wfId,
		"workflowVid", wfVid2,
		"workflowIndexVid", idxVid,
	)

	return nil
}

func (gc *GarbageCollector) gcDuRootCompleted(
	ctx context.Context,
	idxId uuid.I,
	duRootCompleted map[uuid.I]ulid.I,
	now time.Time,
) (int, error) {
	n := 0
	cutoff := now.Add(-ConfigDuRootDeleteAfter)
	for wfId, wfVid := range duRootCompleted {
		completeTime := ulid.Time(wfVid)
		if completeTime.Before(cutoff) {
			if err := gc.deleteDuRoot(
				ctx, idxId, wfId,
			); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (gc *GarbageCollector) deleteDuRoot(
	ctx context.Context,
	idxId uuid.I,
	wfId uuid.I,
) error {
	err := gc.duRootWorkflows.Delete(wfId, durootwf.NoVC)
	if err != nil {
		return err
	}

	vid2, err := gc.workflowIndexes.DeleteDuRoot(
		idxId, wfindexes.NoVC, wfId,
	)
	if err != nil {
		return err
	}

	gc.lg.Infow(
		"GC deleted du-root workflow.",
		"workflowId", wfId,
		"workflowIndexVid", vid2,
	)

	return nil
}

func (gc *GarbageCollector) gcPingRegistryActive(
	ctx context.Context,
	idxId uuid.I,
	pingRegistryActive map[uuid.I]ulid.I,
	now time.Time,
) (int, error) {
	n := 0
	cutoff := now.Add(-ConfigPingRegistryMaxActive)
	for wfId, wfVid := range pingRegistryActive {
		startTime := ulid.Time(wfVid)
		if startTime.Before(cutoff) {
			if err := gc.expirePingRegistry(
				ctx, idxId, wfId,
			); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (gc *GarbageCollector) expirePingRegistry(
	ctx context.Context,
	idxId uuid.I,
	wfId uuid.I,
) error {
	wfVid, err := gc.pingRegistryWorkflows.AbortExpired(
		wfId, pingregistrywf.RetryNoVC,
	)
	if err != nil {
		return err
	}

	idxVid, err := gc.workflowIndexes.CommitPingRegistry(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitPingRegistry{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
		},
	)
	if err != nil {
		return err
	}

	wfVid2, err := gc.pingRegistryWorkflows.End(wfId, wfVid)
	if err != nil {
		return err
	}

	gc.lg.Infow(
		"GC aborted expired ping-registry workflow.",
		"workflowId", wfId,
		"workflowVid", wfVid2,
		"workflowIndexVid", idxVid,
	)

	return nil
}

func (gc *GarbageCollector) gcPingRegistryCompleted(
	ctx context.Context,
	idxId uuid.I,
	pingRegistryCompleted map[uuid.I]ulid.I,
	now time.Time,
) (int, error) {
	n := 0
	cutoff := now.Add(-ConfigPingRegistryDeleteAfter)
	for wfId, wfVid := range pingRegistryCompleted {
		completeTime := ulid.Time(wfVid)
		if completeTime.Before(cutoff) {
			if err := gc.deletePingRegistry(
				ctx, idxId, wfId,
			); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (gc *GarbageCollector) deletePingRegistry(
	ctx context.Context,
	idxId uuid.I,
	wfId uuid.I,
) error {
	err := gc.pingRegistryWorkflows.Delete(wfId, pingregistrywf.NoVC)
	if err != nil {
		return err
	}

	vid2, err := gc.workflowIndexes.DeletePingRegistry(
		idxId, wfindexes.NoVC, wfId,
	)
	if err != nil {
		return err
	}

	gc.lg.Infow(
		"GC deleted ping-registry workflow.",
		"workflowId", wfId,
		"workflowIndexVid", vid2,
	)

	return nil
}

func (gc *GarbageCollector) gcSplitRootActive(
	ctx context.Context,
	idxId uuid.I,
	splitRootActive map[uuid.I]ulid.I,
	now time.Time,
) (int, error) {
	n := 0
	cutoff := now.Add(-ConfigSplitRootMaxActive)
	for wfId, wfVid := range splitRootActive {
		startTime := ulid.Time(wfVid)
		if startTime.Before(cutoff) {
			if err := gc.expireSplitRoot(
				ctx, idxId, wfId,
			); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (gc *GarbageCollector) expireSplitRoot(
	ctx context.Context,
	idxId uuid.I,
	wfId uuid.I,
) error {
	wfVid, err := gc.splitRootWorkflows.AbortExpired(
		wfId, splitrootwf.RetryNoVC,
	)
	if err != nil {
		return err
	}

	idxVid, err := gc.workflowIndexes.CommitSplitRoot(
		idxId, wfindexes.RetryNoVC, &wfindexes.CmdCommitSplitRoot{
			WorkflowId:      wfId,
			WorkflowEventId: wfVid,
		},
	)
	if err != nil {
		return err
	}

	wfVid2, err := gc.splitRootWorkflows.End(wfId, wfVid)
	if err != nil {
		return err
	}

	gc.lg.Infow(
		"GC aborted expired split-root workflow.",
		"workflowId", wfId,
		"workflowVid", wfVid2,
		"workflowIndexVid", idxVid,
	)

	return nil
}

func (gc *GarbageCollector) gcSplitRootCompleted(
	ctx context.Context,
	idxId uuid.I,
	splitRootCompleted map[uuid.I]ulid.I,
	now time.Time,
) (int, error) {
	n := 0
	cutoff := now.Add(-ConfigSplitRootDeleteAfter)
	for wfId, wfVid := range splitRootCompleted {
		completeTime := ulid.Time(wfVid)
		if completeTime.Before(cutoff) {
			if err := gc.deleteSplitRoot(
				ctx, idxId, wfId,
			); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (gc *GarbageCollector) deleteSplitRoot(
	ctx context.Context,
	idxId uuid.I,
	wfId uuid.I,
) error {
	err := gc.splitRootWorkflows.Delete(wfId, splitrootwf.NoVC)
	if err != nil {
		return err
	}

	vid2, err := gc.workflowIndexes.DeleteSplitRoot(
		idxId, wfindexes.NoVC, wfId,
	)
	if err != nil {
		return err
	}

	gc.lg.Infow(
		"GC deleted split-root workflow.",
		"workflowId", wfId,
		"workflowIndexVid", vid2,
	)

	return nil
}

func (gc *GarbageCollector) snapshot(
	ctx context.Context,
	reg string,
	idxId uuid.I,
) error {
	idx, err := gc.workflowIndexes.FindId(idxId)
	if err != nil {
		return err
	}
	vid := idx.Vid()

	vid2, err := gc.workflowIndexes.Snapshot(idxId, vid)
	switch {
	case err == wfindexes.ErrTooManyWorkflows:
		gc.lg.Infow(
			"GC skipped workflow index snapshot.",
			"registry", reg,
			"reason", "too open workflows",
		)
		return nil

	case err == wfindexes.ErrSmallStorageReduction:
		gc.lg.Infow(
			"GC skipped workflow index snapshot.",
			"registry", reg,
			"reason", "storage reduction too small",
		)
		return nil

	case vid2 == vid:
		gc.lg.Infow(
			"GC skipped workflow index snapshot.",
			"registry", reg,
			"reason", "latest event is already a snapshot",
		)
		return nil

	case err != nil:
		return err

	default:
		gc.lg.Infow(
			"GC created workflow index snapshot.",
			"registry", reg,
			"vid", vid2,
		)
		return nil
	}

}
