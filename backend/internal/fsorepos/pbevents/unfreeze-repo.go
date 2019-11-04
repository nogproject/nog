package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED` aka `EvUnfreezeRepoStarted` is a
// legacy event that was used in the preliminary repo-freeze implementation.
type EvUnfreezeRepoStarted struct{}

func (EvUnfreezeRepoStarted) RepoEvent() {}

func fromPbUnfreezeRepoStarted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED {
		panic("invalid event")
	}
	ev := &EvUnfreezeRepoStarted{}
	return ev, nil
}

// `RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED` aka `EvUnfreezeRepoCompleted` is
// a legacy event that was used in the preliminary repo-freeze implementation.
type EvUnfreezeRepoCompleted struct{}

func (EvUnfreezeRepoCompleted) RepoEvent() {}

func fromPbUnfreezeRepoCompleted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED {
		panic("invalid event")
	}
	ev := &EvUnfreezeRepoCompleted{}
	return ev, nil
}

// `RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2` aka `EvUnfreezeRepoStarted2`.
// See freeze-repo workflow aka freezerepowf.
type EvUnfreezeRepoStarted2 struct {
	WorkflowId uuid.I
}

func (EvUnfreezeRepoStarted2) RepoEvent() {}

func NewUnfreezeRepoStarted2(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbUnfreezeRepoStarted2(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2 {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvUnfreezeRepoStarted2{
		WorkflowId: workflowId,
	}, nil
}

// `RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2` aka `EvUnfreezeRepoCompleted2`.
// See freeze-repo workflow aka freezerepowf.
type EvUnfreezeRepoCompleted2 struct {
	WorkflowId uuid.I
	StatusCode int32
}

func (EvUnfreezeRepoCompleted2) RepoEvent() {}

func NewUnfreezeRepoCompleted2Ok(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func NewUnfreezeRepoCompleted2Error(
	workflowId uuid.I, code int32,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	if code == 0 {
		panic("zero code")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2,
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbUnfreezeRepoCompleted2(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2 {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvUnfreezeRepoCompleted2{
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}, nil
}
