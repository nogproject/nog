package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED` aka `EvArchiveRepoStarted`.
// See freeze-repo workflow aka freezerepowf.
type EvArchiveRepoStarted struct {
	RepoId     uuid.I
	WorkflowId uuid.I
}

func (EvArchiveRepoStarted) RegistryEvent() {}

func NewArchiveRepoStarted(
	repoId uuid.I, workflowId uuid.I,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbArchiveRepoStarted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED {
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
	return &EvArchiveRepoStarted{
		RepoId:     repoId,
		WorkflowId: workflowId,
	}, nil
}

// `RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` aka `EvArchiveRepoCompleted`.
// See freeze-repo workflow aka freezerepowf.
type EvArchiveRepoCompleted struct {
	RepoId     uuid.I
	WorkflowId uuid.I
	StatusCode int32
}

func (EvArchiveRepoCompleted) RegistryEvent() {}

func NewArchiveRepoCompletedOk(
	repoId uuid.I, workflowId uuid.I,
) pb.RegistryEvent {
	if repoId == uuid.Nil {
		panic("nil repoId")
	}
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RegistryEvent{
		Event:      pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
	}
	return evpb
}

func NewArchiveRepoCompletedError(
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
		Event:      pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		RepoId:     repoId[:],
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbArchiveRepoCompleted(
	evpb pb.RegistryEvent,
) (RegistryEvent, error) {
	if evpb.Event != pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED {
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
	return &EvArchiveRepoCompleted{
		RepoId:     repoId,
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}, nil
}
