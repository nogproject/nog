package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RepoEvent_EV_FSO_FREEZE_REPO_STARTED` aka `EvFreezeRepoStarted` is a legacy
// event that was used in the preliminary repo-freeze implementation.
type EvFreezeRepoStarted struct{}

func (EvFreezeRepoStarted) RepoEvent() {}

func fromPbFreezeRepoStarted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED {
		panic("invalid event")
	}
	ev := &EvFreezeRepoStarted{}
	return ev, nil
}

// `RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED` aka `EvFreezeRepoCompleted` is a
// legacy event that was used in the preliminary repo-freeze implementation.
type EvFreezeRepoCompleted struct{}

func (EvFreezeRepoCompleted) RepoEvent() {}

func fromPbFreezeRepoCompleted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED {
		panic("invalid event")
	}
	ev := &EvFreezeRepoCompleted{}
	return ev, nil
}

// `RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2` aka `EvFreezeRepoStarted2`.  See
// freeze-repo workflow aka freezerepowf.
type EvFreezeRepoStarted2 struct {
	WorkflowId uuid.I
}

func (EvFreezeRepoStarted2) RepoEvent() {}

func NewFreezeRepoStarted2(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbFreezeRepoStarted2(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2 {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvFreezeRepoStarted2{
		WorkflowId: workflowId,
	}, nil
}

// `RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2` aka `EvFreezeRepoCompleted2`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoCompleted2 struct {
	WorkflowId uuid.I
	StatusCode int32
}

func (EvFreezeRepoCompleted2) RepoEvent() {}

func NewFreezeRepoCompleted2Ok(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func NewFreezeRepoCompleted2Error(
	workflowId uuid.I, code int32,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	if code == 0 {
		panic("zero code")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbFreezeRepoCompleted2(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2 {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvFreezeRepoCompleted2{
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}, nil
}
