package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED` aka `EvArchiveRepoStarted`.  See
// freeze-repo workflow aka freezerepowf.
type EvArchiveRepoStarted struct {
	WorkflowId uuid.I
}

func (EvArchiveRepoStarted) RepoEvent() {}

func NewArchiveRepoStarted(
	workflowId uuid.I,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED,
		WorkflowId: workflowId[:],
	}
	return evpb
}

func fromPbArchiveRepoStarted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvArchiveRepoStarted{
		WorkflowId: workflowId,
	}, nil
}

// `RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` aka `EvArchiveRepoCompleted`.  See
// freeze-repo workflow aka freezerepowf.
type EvArchiveRepoCompleted struct {
	WorkflowId uuid.I
	StatusCode int32
	TarPath    string
}

func (EvArchiveRepoCompleted) RepoEvent() {}

func NewArchiveRepoCompletedOk(
	workflowId uuid.I,
	tarPath string,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	if tarPath == "" {
		panic("empty tar path")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		WorkflowId: workflowId[:],
		TarttTarInfo: &pb.TarttTarInfo{
			Path: tarPath,
		},
	}
	return evpb
}

func NewArchiveRepoCompletedError(
	workflowId uuid.I, code int32,
) pb.RepoEvent {
	if workflowId == uuid.Nil {
		panic("nil workflowId")
	}
	if code == 0 {
		panic("zero code")
	}
	evpb := pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		WorkflowId: workflowId[:],
		StatusCode: code,
	}
	return evpb
}

func fromPbArchiveRepoCompleted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	ev := &EvArchiveRepoCompleted{
		WorkflowId: workflowId,
		StatusCode: evpb.StatusCode,
	}
	if inf := evpb.TarttTarInfo; inf != nil {
		ev.TarPath = inf.Path
	}
	return ev, nil
}
