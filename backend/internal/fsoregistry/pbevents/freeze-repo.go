package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2` aka `EvFreezeRepoStarted2`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoStarted2 struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

func (EvFreezeRepoStarted2) RegistryEvent() {}

func NewFreezeRepoStarted2(
	repoId uuid.I, workflowId uuid.I,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbFreezeRepoStarted2(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2 {
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
	return &EvFreezeRepoStarted2{
		RepoId:     repoId,
		WorkflowId: workflowId,
	}, nil
}

// `RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2` aka `EvFreezeRepoCompleted2`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoCompleted2 struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	StatusCode int32
}

func (EvFreezeRepoCompleted2) RegistryEvent() {}

func NewFreezeRepoCompleted2Ok(
	repoId uuid.I, workflowId uuid.I,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
	}
	return evpb
}

func NewFreezeRepoCompleted2Error(
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
		Event:      pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbFreezeRepoCompleted2(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2 {
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
	return &EvFreezeRepoCompleted2{
		RepoId:     repoId,
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}, nil
}
