package wfindexes

import (
	"bytes"
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/events"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

const ConfigMaxProtoSize = 100 * 1024

func snapshot(j *events.Journal, id uuid.I) ([]pb.WorkflowEvent, error) {
	idx, err := loadIndexView(j, id)
	if err != nil {
		return nil, err
	}

	// Large state could in principle be split into chunks to limit the
	// size of the snapshot protobufs.  But we avoid splitting and instead
	// assume that the state is small, which is ensured by the precondition
	// `ConfigMaxSnapshotWorkflows` in `tellSnapshot()`.  `checkSize()`
	// verifies the assumption.
	checkSize := func(ev *pb.WorkflowEvent) {
		if proto.Size(ev) > ConfigMaxProtoSize {
			panic("invalid tellSnapshot() size limit")
		}
	}

	evs := make([]pb.WorkflowEvent, 0, 5)
	evs = append(evs, wfevents.NewPbSnapshotBegin())
	if len(idx.duRoot) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				DuRoot: idx.duRootSorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	if len(idx.pingRegistry) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				PingRegistry: idx.pingRegistrySorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	if len(idx.splitRoot) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				SplitRoot: idx.splitRootSorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	if len(idx.freezeRepo) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				FreezeRepo: idx.freezeRepoSorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	if len(idx.unfreezeRepo) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				UnfreezeRepo: idx.unfreezeRepoSorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	if len(idx.archiveRepo) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				ArchiveRepo: idx.archiveRepoSorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	if len(idx.unarchiveRepo) > 0 {
		ev := wfevents.NewPbWorkflowIndexSnapshotState(
			&wfevents.EvWorkflowIndexSnapshotState{
				UnarchiveRepo: idx.unarchiveRepoSorted(),
			},
		)
		checkSize(&ev)
		evs = append(evs, ev)
	}
	evs = append(evs, wfevents.NewPbSnapshotEnd())

	return evs, nil
}

type indexView struct {
	vid           ulid.I
	duRoot        map[uuid.I]*wfevents.WorkflowIndexState_DuRoot
	pingRegistry  map[uuid.I]*wfevents.WorkflowIndexState_PingRegistry
	splitRoot     map[uuid.I]*wfevents.WorkflowIndexState_SplitRoot
	freezeRepo    map[uuid.I]*wfevents.WorkflowIndexState_FreezeRepo
	unfreezeRepo  map[uuid.I]*wfevents.WorkflowIndexState_UnfreezeRepo
	archiveRepo   map[uuid.I]*wfevents.WorkflowIndexState_ArchiveRepo
	unarchiveRepo map[uuid.I]*wfevents.WorkflowIndexState_UnarchiveRepo
}

func (idx *indexView) resetState() {
	idx.duRoot = make(map[uuid.I]*wfevents.WorkflowIndexState_DuRoot)
	idx.pingRegistry = make(map[uuid.I]*wfevents.WorkflowIndexState_PingRegistry)
	idx.splitRoot = make(map[uuid.I]*wfevents.WorkflowIndexState_SplitRoot)
	idx.freezeRepo = make(map[uuid.I]*wfevents.WorkflowIndexState_FreezeRepo)
	idx.unfreezeRepo = make(map[uuid.I]*wfevents.WorkflowIndexState_UnfreezeRepo)
	idx.archiveRepo = make(map[uuid.I]*wfevents.WorkflowIndexState_ArchiveRepo)
	idx.unarchiveRepo = make(map[uuid.I]*wfevents.WorkflowIndexState_UnarchiveRepo)
}

func (idx *indexView) duRootSorted() []*wfevents.WorkflowIndexState_DuRoot {
	s := make([]*wfevents.WorkflowIndexState_DuRoot, 0, len(idx.duRoot))
	for _, e := range idx.duRoot {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func (idx *indexView) pingRegistrySorted() []*wfevents.WorkflowIndexState_PingRegistry {
	s := make([]*wfevents.WorkflowIndexState_PingRegistry, 0, len(idx.pingRegistry))
	for _, e := range idx.pingRegistry {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func (idx *indexView) splitRootSorted() []*wfevents.WorkflowIndexState_SplitRoot {
	s := make([]*wfevents.WorkflowIndexState_SplitRoot, 0, len(idx.splitRoot))
	for _, e := range idx.splitRoot {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func (idx *indexView) freezeRepoSorted() []*wfevents.WorkflowIndexState_FreezeRepo {
	s := make([]*wfevents.WorkflowIndexState_FreezeRepo, 0, len(idx.freezeRepo))
	for _, e := range idx.freezeRepo {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func (idx *indexView) unfreezeRepoSorted() []*wfevents.WorkflowIndexState_UnfreezeRepo {
	s := make([]*wfevents.WorkflowIndexState_UnfreezeRepo, 0, len(idx.unfreezeRepo))
	for _, e := range idx.unfreezeRepo {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func (idx *indexView) archiveRepoSorted() []*wfevents.WorkflowIndexState_ArchiveRepo {
	s := make([]*wfevents.WorkflowIndexState_ArchiveRepo, 0, len(idx.archiveRepo))
	for _, e := range idx.archiveRepo {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func (idx *indexView) unarchiveRepoSorted() []*wfevents.WorkflowIndexState_UnarchiveRepo {
	s := make([]*wfevents.WorkflowIndexState_UnarchiveRepo, 0, len(idx.unarchiveRepo))
	for _, e := range idx.unarchiveRepo {
		s = append(s, e)
	}
	sort.Slice(s, func(i, j int) bool {
		return bytes.Compare(
			s[i].StartedWorkflowEventId[:],
			s[j].StartedWorkflowEventId[:],
		) < 0
	})
	return s
}

func loadIndexView(j *events.Journal, idxId uuid.I) (*indexView, error) {
	idx := &indexView{}
	idx.resetState()

	iter := j.Find(idxId, events.EventEpoch)
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
		if err := idx.loadEvent(vid, wfev); err != nil {
			return nil, err
		}
	}
	if err := iterClose(); err != nil {
		return nil, err
	}

	return idx, nil
}

func (idx *indexView) loadEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	idx.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvSnapshotBegin:
		idx.resetState()
		return nil

	case *wfevents.EvSnapshotEnd:
		return nil

	case *wfevents.EvWorkflowIndexSnapshotState:
		for _, w := range x.DuRoot {
			idx.duRoot[w.WorkflowId] = w
		}
		for _, w := range x.PingRegistry {
			idx.pingRegistry[w.WorkflowId] = w
		}
		for _, w := range x.SplitRoot {
			idx.splitRoot[w.WorkflowId] = w
		}
		for _, w := range x.FreezeRepo {
			idx.freezeRepo[w.WorkflowId] = w
		}
		for _, w := range x.UnfreezeRepo {
			idx.unfreezeRepo[w.WorkflowId] = w
		}
		for _, w := range x.ArchiveRepo {
			idx.archiveRepo[w.WorkflowId] = w
		}
		for _, w := range x.UnarchiveRepo {
			idx.unarchiveRepo[w.WorkflowId] = w
		}
		return nil

	case *wfevents.EvDuRootStarted:
		xId := x.WorkflowId
		idx.duRoot[xId] = &wfevents.WorkflowIndexState_DuRoot{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
			GlobalRoot:             x.GlobalRoot,
			Host:                   x.Host,
			HostRoot:               x.HostRoot,
		}
		return nil

	case *wfevents.EvDuRootCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.duRoot[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvDuRootDeleted:
		delete(idx.duRoot, x.WorkflowId)
		return nil

	case *wfevents.EvPingRegistryStarted:
		xId := x.WorkflowId
		idx.pingRegistry[xId] = &wfevents.WorkflowIndexState_PingRegistry{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
		}
		return nil

	case *wfevents.EvPingRegistryCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.pingRegistry[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvPingRegistryDeleted:
		delete(idx.pingRegistry, x.WorkflowId)
		return nil

	case *wfevents.EvSplitRootStarted:
		xId := x.WorkflowId
		idx.splitRoot[xId] = &wfevents.WorkflowIndexState_SplitRoot{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
			GlobalRoot:             x.GlobalRoot,
			Host:                   x.Host,
			HostRoot:               x.HostRoot,
		}
		return nil

	case *wfevents.EvSplitRootCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.splitRoot[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvSplitRootDeleted:
		delete(idx.splitRoot, x.WorkflowId)
		return nil

	case *wfevents.EvFreezeRepoStarted2:
		xId := x.WorkflowId
		idx.freezeRepo[xId] = &wfevents.WorkflowIndexState_FreezeRepo{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
			GlobalPath:             x.RepoGlobalPath,
		}
		return nil

	case *wfevents.EvFreezeRepoCompleted2:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.freezeRepo[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvFreezeRepoDeleted:
		delete(idx.freezeRepo, x.WorkflowId)
		return nil

	case *wfevents.EvUnfreezeRepoStarted2:
		xId := x.WorkflowId
		idx.unfreezeRepo[xId] = &wfevents.WorkflowIndexState_UnfreezeRepo{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
			GlobalPath:             x.RepoGlobalPath,
		}
		return nil

	case *wfevents.EvUnfreezeRepoCompleted2:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.unfreezeRepo[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvUnfreezeRepoDeleted:
		delete(idx.unfreezeRepo, x.WorkflowId)
		return nil

	case *wfevents.EvArchiveRepoStarted:
		xId := x.WorkflowId
		idx.archiveRepo[xId] = &wfevents.WorkflowIndexState_ArchiveRepo{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
			GlobalPath:             x.RepoGlobalPath,
		}
		return nil

	case *wfevents.EvArchiveRepoCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.archiveRepo[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvArchiveRepoDeleted:
		delete(idx.archiveRepo, x.WorkflowId)
		return nil

	case *wfevents.EvUnarchiveRepoStarted:
		xId := x.WorkflowId
		idx.unarchiveRepo[xId] = &wfevents.WorkflowIndexState_UnarchiveRepo{
			WorkflowId:             xId,
			StartedWorkflowEventId: x.WorkflowEventId,
			GlobalPath:             x.RepoGlobalPath,
		}
		return nil

	case *wfevents.EvUnarchiveRepoCompleted:
		xId := x.WorkflowId
		xVid := x.WorkflowEventId
		idx.unarchiveRepo[xId].CompletedWorkflowEventId = xVid
		return nil

	case *wfevents.EvUnarchiveRepoDeleted:
		delete(idx.unarchiveRepo, x.WorkflowId)
		return nil

	default:
		panic("invalid event")
	}
}
