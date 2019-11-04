package events

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED` aka
// `EvShadowRepoMoveStarted` initializes a shadow-move workflow.  It refers to
// the corresponding repo event that started the workflow.
type EvShadowRepoMoveStarted struct {
	RepoId      uuid.I
	RepoEventId ulid.I
}

func (EvShadowRepoMoveStarted) WorkflowEvent() {}

func NewPbShadowRepoMoveStarted(
	repoId uuid.I, repoEventId ulid.I,
) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:       pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED,
		RepoId:      repoId[:],
		RepoEventId: repoEventId[:],
	}
}

func fromPbShadowRepoMoveStarted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, err
	}
	repoEventId, err := ulid.ParseBytes(evpb.RepoEventId)
	if err != nil {
		return nil, err
	}
	ev := &EvShadowRepoMoveStarted{
		RepoId:      repoId,
		RepoEventId: repoEventId,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED` aka `EvShadowRepoMoved` completes a
// shadow-move workflow.
type EvShadowRepoMoved struct {
	RepoId uuid.I
}

func (EvShadowRepoMoved) WorkflowEvent() {}

func NewPbShadowRepoMoved(repoId uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:  pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED,
		RepoId: repoId[:],
	}
}

func fromPbShadowRepoMoved(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, err
	}
	ev := &EvShadowRepoMoved{
		RepoId: repoId,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED` aka `EvShadowRepoMoveStaDisabled`
// indicates that Nogfsostad has disabled the shadow repo.
type EvShadowRepoMoveStaDisabled struct{}

func (EvShadowRepoMoveStaDisabled) WorkflowEvent() {}

func NewPbShadowRepoMoveStaDisabled() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED,
	}
}

func fromPbShadowRepoMoveStaDisabled(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED {
		panic("invalid event")
	}
	ev := &EvShadowRepoMoveStaDisabled{}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED` aka
// `EvShadowRepoMoveCommitted` indicates that the workflow completed
// successfully.
type EvShadowRepoMoveCommitted struct{}

func (EvShadowRepoMoveCommitted) WorkflowEvent() {}

func NewPbShadowRepoMoveCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED,
	}
}

func fromPbShadowRepoMoveCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED {
		panic("invalid event")
	}
	ev := &EvShadowRepoMoveCommitted{}
	return ev, nil
}
