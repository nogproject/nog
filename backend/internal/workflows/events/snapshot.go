package events

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

//
type EvSnapshotBegin struct{}

func (EvSnapshotBegin) WorkflowEvent() {}

func NewPbSnapshotBegin() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_SNAPSHOT_BEGIN,
	}
}

func fromPbSnapshotBegin(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_SNAPSHOT_BEGIN {
		panic("invalid event")
	}
	ev := &EvSnapshotBegin{}
	return ev, nil
}

//
type EvSnapshotEnd struct{}

func (EvSnapshotEnd) WorkflowEvent() {}

func NewPbSnapshotEnd() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_SNAPSHOT_END,
	}
}

func fromPbSnapshotEnd(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_SNAPSHOT_END {
		panic("invalid event")
	}
	ev := &EvSnapshotEnd{}
	return ev, nil
}

//
type EvWorkflowIndexSnapshotState struct {
	DuRoot        []*WorkflowIndexState_DuRoot
	PingRegistry  []*WorkflowIndexState_PingRegistry
	SplitRoot     []*WorkflowIndexState_SplitRoot
	FreezeRepo    []*WorkflowIndexState_FreezeRepo
	UnfreezeRepo  []*WorkflowIndexState_UnfreezeRepo
	ArchiveRepo   []*WorkflowIndexState_ArchiveRepo
	UnarchiveRepo []*WorkflowIndexState_UnarchiveRepo
}

type WorkflowIndexState_DuRoot struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
	GlobalRoot               string
	Host                     string
	HostRoot                 string
}

type WorkflowIndexState_PingRegistry struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
}

type WorkflowIndexState_SplitRoot struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
	GlobalRoot               string
	Host                     string
	HostRoot                 string
}

type WorkflowIndexState_FreezeRepo struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
	GlobalPath               string
}

type WorkflowIndexState_UnfreezeRepo struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
	GlobalPath               string
}

type WorkflowIndexState_ArchiveRepo struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
	GlobalPath               string
}

type WorkflowIndexState_UnarchiveRepo struct {
	WorkflowId               uuid.I
	StartedWorkflowEventId   ulid.I
	CompletedWorkflowEventId ulid.I
	GlobalPath               string
}

func (EvWorkflowIndexSnapshotState) WorkflowEvent() {}

func NewPbWorkflowIndexSnapshotState(
	ev *EvWorkflowIndexSnapshotState,
) pb.WorkflowEvent {
	duRoot := make([]*pb.WorkflowIndexState_DuRoot, 0, len(ev.DuRoot))
	for _, e := range ev.DuRoot {
		p := &pb.WorkflowIndexState_DuRoot{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
			GlobalRoot:             e.GlobalRoot,
			Host:                   e.Host,
			HostRoot:               e.HostRoot,
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		duRoot = append(duRoot, p)
	}

	pingRegistry := make([]*pb.WorkflowIndexState_PingRegistry, 0, len(ev.PingRegistry))
	for _, e := range ev.PingRegistry {
		p := &pb.WorkflowIndexState_PingRegistry{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		pingRegistry = append(pingRegistry, p)
	}

	splitRoot := make([]*pb.WorkflowIndexState_SplitRoot, 0, len(ev.SplitRoot))
	for _, e := range ev.SplitRoot {
		p := &pb.WorkflowIndexState_SplitRoot{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
			GlobalRoot:             e.GlobalRoot,
			Host:                   e.Host,
			HostRoot:               e.HostRoot,
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		splitRoot = append(splitRoot, p)
	}

	freezeRepo := make([]*pb.WorkflowIndexState_FreezeRepo, 0, len(ev.FreezeRepo))
	for _, e := range ev.FreezeRepo {
		p := &pb.WorkflowIndexState_FreezeRepo{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
			GlobalPath:             e.GlobalPath,
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		freezeRepo = append(freezeRepo, p)
	}

	unfreezeRepo := make([]*pb.WorkflowIndexState_UnfreezeRepo, 0, len(ev.UnfreezeRepo))
	for _, e := range ev.UnfreezeRepo {
		p := &pb.WorkflowIndexState_UnfreezeRepo{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
			GlobalPath:             e.GlobalPath,
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		unfreezeRepo = append(unfreezeRepo, p)
	}

	archiveRepo := make([]*pb.WorkflowIndexState_ArchiveRepo, 0, len(ev.ArchiveRepo))
	for _, e := range ev.ArchiveRepo {
		p := &pb.WorkflowIndexState_ArchiveRepo{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
			GlobalPath:             e.GlobalPath,
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		archiveRepo = append(archiveRepo, p)
	}

	unarchiveRepo := make([]*pb.WorkflowIndexState_UnarchiveRepo, 0, len(ev.UnarchiveRepo))
	for _, e := range ev.UnarchiveRepo {
		p := &pb.WorkflowIndexState_UnarchiveRepo{
			WorkflowId:             e.WorkflowId[:],
			StartedWorkflowEventId: e.StartedWorkflowEventId[:],
			GlobalPath:             e.GlobalPath,
		}
		if e.CompletedWorkflowEventId != ulid.Nil {
			p.CompletedWorkflowEventId = e.CompletedWorkflowEventId[:]
		}
		unarchiveRepo = append(unarchiveRepo, p)
	}

	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_WORKFLOW_INDEX_SNAPSHOT_STATE,
		WorkflowIndexState: &pb.WorkflowIndexState{
			DuRoot:        duRoot,
			PingRegistry:  pingRegistry,
			SplitRoot:     splitRoot,
			FreezeRepo:    freezeRepo,
			UnfreezeRepo:  unfreezeRepo,
			ArchiveRepo:   archiveRepo,
			UnarchiveRepo: unarchiveRepo,
		},
	}
}

func fromPbWorkflowIndexSnapshotState(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_WORKFLOW_INDEX_SNAPSHOT_STATE {
		panic("invalid event")
	}

	st := evpb.WorkflowIndexState
	duRoot, err := fromPbWorkflowIndexSnapshotState_DuRoot(st.DuRoot)
	if err != nil {
		return nil, err
	}
	pingRegistry, err := fromPbWorkflowIndexSnapshotState_PingRegistry(st.PingRegistry)
	if err != nil {
		return nil, err
	}
	splitRoot, err := fromPbWorkflowIndexSnapshotState_SplitRoot(st.SplitRoot)
	if err != nil {
		return nil, err
	}
	freezeRepo, err := fromPbWorkflowIndexSnapshotState_FreezeRepo(st.FreezeRepo)
	if err != nil {
		return nil, err
	}
	unfreezeRepo, err := fromPbWorkflowIndexSnapshotState_UnfreezeRepo(st.UnfreezeRepo)
	if err != nil {
		return nil, err
	}
	archiveRepo, err := fromPbWorkflowIndexSnapshotState_ArchiveRepo(st.ArchiveRepo)
	if err != nil {
		return nil, err
	}
	unarchiveRepo, err := fromPbWorkflowIndexSnapshotState_UnarchiveRepo(st.UnarchiveRepo)
	if err != nil {
		return nil, err
	}

	ev := &EvWorkflowIndexSnapshotState{
		DuRoot:        duRoot,
		PingRegistry:  pingRegistry,
		SplitRoot:     splitRoot,
		FreezeRepo:    freezeRepo,
		UnfreezeRepo:  unfreezeRepo,
		ArchiveRepo:   archiveRepo,
		UnarchiveRepo: unarchiveRepo,
	}
	return ev, nil
}

func fromPbWorkflowIndexSnapshotState_DuRoot(
	pbDuRoot []*pb.WorkflowIndexState_DuRoot,
) ([]*WorkflowIndexState_DuRoot, error) {
	duRoot := make([]*WorkflowIndexState_DuRoot, 0, len(pbDuRoot))
	for _, p := range pbDuRoot {
		e := &WorkflowIndexState_DuRoot{
			GlobalRoot: p.GlobalRoot,
			Host:       p.Host,
			HostRoot:   p.HostRoot,
		}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		duRoot = append(duRoot, e)
	}
	return duRoot, nil
}

func fromPbWorkflowIndexSnapshotState_PingRegistry(
	pbPingRegistry []*pb.WorkflowIndexState_PingRegistry,
) ([]*WorkflowIndexState_PingRegistry, error) {
	pingRegistry := make([]*WorkflowIndexState_PingRegistry, 0, len(pbPingRegistry))
	for _, p := range pbPingRegistry {
		e := &WorkflowIndexState_PingRegistry{}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		pingRegistry = append(pingRegistry, e)
	}
	return pingRegistry, nil
}

func fromPbWorkflowIndexSnapshotState_SplitRoot(
	pbSplitRoot []*pb.WorkflowIndexState_SplitRoot,
) ([]*WorkflowIndexState_SplitRoot, error) {
	splitRoot := make([]*WorkflowIndexState_SplitRoot, 0, len(pbSplitRoot))
	for _, p := range pbSplitRoot {
		e := &WorkflowIndexState_SplitRoot{
			GlobalRoot: p.GlobalRoot,
			Host:       p.Host,
			HostRoot:   p.HostRoot,
		}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		splitRoot = append(splitRoot, e)
	}
	return splitRoot, nil
}

func fromPbWorkflowIndexSnapshotState_FreezeRepo(
	pbFreezeRepo []*pb.WorkflowIndexState_FreezeRepo,
) ([]*WorkflowIndexState_FreezeRepo, error) {
	freezeRepo := make([]*WorkflowIndexState_FreezeRepo, 0, len(pbFreezeRepo))
	for _, p := range pbFreezeRepo {
		e := &WorkflowIndexState_FreezeRepo{
			GlobalPath: p.GlobalPath,
		}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		freezeRepo = append(freezeRepo, e)
	}
	return freezeRepo, nil
}

func fromPbWorkflowIndexSnapshotState_UnfreezeRepo(
	pbUnfreezeRepo []*pb.WorkflowIndexState_UnfreezeRepo,
) ([]*WorkflowIndexState_UnfreezeRepo, error) {
	unfreezeRepo := make([]*WorkflowIndexState_UnfreezeRepo, 0, len(pbUnfreezeRepo))
	for _, p := range pbUnfreezeRepo {
		e := &WorkflowIndexState_UnfreezeRepo{
			GlobalPath: p.GlobalPath,
		}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		unfreezeRepo = append(unfreezeRepo, e)
	}
	return unfreezeRepo, nil
}

func fromPbWorkflowIndexSnapshotState_ArchiveRepo(
	pbArchiveRepo []*pb.WorkflowIndexState_ArchiveRepo,
) ([]*WorkflowIndexState_ArchiveRepo, error) {
	archiveRepo := make([]*WorkflowIndexState_ArchiveRepo, 0, len(pbArchiveRepo))
	for _, p := range pbArchiveRepo {
		e := &WorkflowIndexState_ArchiveRepo{
			GlobalPath: p.GlobalPath,
		}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		archiveRepo = append(archiveRepo, e)
	}
	return archiveRepo, nil
}

func fromPbWorkflowIndexSnapshotState_UnarchiveRepo(
	pbUnarchiveRepo []*pb.WorkflowIndexState_UnarchiveRepo,
) ([]*WorkflowIndexState_UnarchiveRepo, error) {
	unarchiveRepo := make([]*WorkflowIndexState_UnarchiveRepo, 0, len(pbUnarchiveRepo))
	for _, p := range pbUnarchiveRepo {
		e := &WorkflowIndexState_UnarchiveRepo{
			GlobalPath: p.GlobalPath,
		}

		id, err := uuid.FromBytes(p.WorkflowId)
		if err != nil {
			return nil, err
		}
		e.WorkflowId = id

		vid, err := ulid.ParseBytes(p.StartedWorkflowEventId)
		if err != nil {
			return nil, err
		}
		e.StartedWorkflowEventId = vid

		if p.CompletedWorkflowEventId != nil {
			vid, err := ulid.ParseBytes(p.CompletedWorkflowEventId)
			if err != nil {
				return nil, err
			}
			e.CompletedWorkflowEventId = vid
		}

		unarchiveRepo = append(unarchiveRepo, e)
	}
	return unarchiveRepo, nil
}
