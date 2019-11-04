package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED` aka `EvUnarchiveRepoStarted`.
// See freeze-repo workflow aka freezerepowf.
type EvUnarchiveRepoStarted struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

func (EvUnarchiveRepoStarted) RegistryEvent() {}

func NewUnarchiveRepoStarted(
	repoId uuid.I, workflowId uuid.I,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbUnarchiveRepoStarted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, &ParseError{What: "repo ID", Err: err}
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvUnarchiveRepoStarted{
		RepoId:     repoId,
		WorkflowId: workflowId,
	}, nil
}

// `RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` aka `EvUnarchiveRepoCompleted`.
// See freeze-repo workflow aka freezerepowf.
type EvUnarchiveRepoCompleted struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	StatusCode int32
}

func (EvUnarchiveRepoCompleted) RegistryEvent() {}

func NewUnarchiveRepoCompletedOk(
	repoId uuid.I, workflowId uuid.I,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
	}
	return evpb
}

func NewUnarchiveRepoCompletedError(
	repoId uuid.I, workflowId uuid.I, code int32,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	if code == 0 {
		panic("zero code")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbUnarchiveRepoCompleted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, &ParseError{What: "repo ID", Err: err}
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvUnarchiveRepoCompleted{
		RepoId:     repoId,
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}, nil
}
