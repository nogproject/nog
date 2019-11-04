package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED` aka `EvUnarchiveRepoStarted`.  See
// freeze-repo workflow aka freezerepowf.
type EvUnarchiveRepoStarted struct {
	WorkflowId uuid.I
}

func (EvUnarchiveRepoStarted) RepoEvent() {}

func NewUnarchiveRepoStarted(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbUnarchiveRepoStarted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvUnarchiveRepoStarted{
		WorkflowId: workflowId,
	}, nil
}

// `RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` aka `EvUnarchiveRepoCompleted`.
// See freeze-repo workflow aka freezerepowf.
type EvUnarchiveRepoCompleted struct {
	WorkflowId uuid.I
	StatusCode int32
}

func (EvUnarchiveRepoCompleted) RepoEvent() {}

func NewUnarchiveRepoCompletedOk(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func NewUnarchiveRepoCompletedError(
	workflowId uuid.I, code int32,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	if code == 0 {
		panic("zero code")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbUnarchiveRepoCompleted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvUnarchiveRepoCompleted{
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}, nil
}
