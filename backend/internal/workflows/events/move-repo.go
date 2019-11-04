package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_REPO_MOVE_STARTED` aka `EvRepoMoveStarted` initializes
// a repo-move workflow.  It refers to the corresponding repo event that
// started the workflow.
type EvRepoMoveStarted struct {
	RepoId        uuid.I
	RepoEventId   ulid.I
	OldGlobalPath string
	OldFileHost   string
	OldHostPath   string
	OldShadowPath string
	NewGlobalPath string
	NewFileHost   string
	NewHostPath   string
}

func (EvRepoMoveStarted) WorkflowEvent() {}

func (ev *EvRepoMoveStarted) validate() error {
	if ev.RepoId == uuid.Nil {
		return errors.New("nil RepoId")
	}
	if ev.RepoEventId == ulid.Nil {
		return errors.New("nil RepoEventId")
	}
	if ev.OldGlobalPath == "" ||
		ev.OldFileHost == "" ||
		ev.OldHostPath == "" ||
		ev.OldShadowPath == "" ||
		ev.NewGlobalPath == "" ||
		ev.NewFileHost == "" ||
		ev.NewHostPath == "" {
		return errors.New("some path is empty")
	}
	// Changing the file host is unsupported.
	if ev.OldFileHost != ev.NewFileHost {
		return errors.New("old and new file hosts differ")
	}
	return nil
}

func NewPbRepoMoveStarted(ev *EvRepoMoveStarted) pb.WorkflowEvent {
	if err := ev.validate(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:       pb.WorkflowEvent_EV_FSO_REPO_MOVE_STARTED,
		RepoId:      ev.RepoId[:],
		RepoEventId: ev.RepoEventId[:],
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.OldGlobalPath,
			FileHost:   ev.OldFileHost,
			HostPath:   ev.OldHostPath,
		},
		FsoShadowRepoInfo: &pb.FsoShadowRepoInfo{
			ShadowPath: ev.OldShadowPath,
		},
		NewFsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.NewGlobalPath,
			FileHost:   ev.NewFileHost,
			HostPath:   ev.NewHostPath,
		},
	}
}

func fromPbRepoMoveStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_REPO_MOVE_STARTED {
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
	return &EvRepoMoveStarted{
		RepoId:        repoId,
		RepoEventId:   repoEventId,
		OldGlobalPath: evpb.FsoRepoInitInfo.GlobalPath,
		OldFileHost:   evpb.FsoRepoInitInfo.FileHost,
		OldHostPath:   evpb.FsoRepoInitInfo.HostPath,
		OldShadowPath: evpb.FsoShadowRepoInfo.ShadowPath,
		NewGlobalPath: evpb.NewFsoRepoInitInfo.GlobalPath,
		NewFileHost:   evpb.NewFsoRepoInitInfo.FileHost,
		NewHostPath:   evpb.NewFsoRepoInitInfo.HostPath,
	}, nil
}

// `WorkflowEvent_EV_FSO_REPO_STA_RELEASED` aka `EvRepoMoveStaReleased`
// indicates that Nogfsostad has released the repo.
type EvRepoMoveStaReleased struct{}

func (EvRepoMoveStaReleased) WorkflowEvent() {}

func NewPbRepoMoveStaReleased() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED,
	}
}

func fromPbRepoMoveStaReleased(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED {
		panic("invalid event")
	}
	return &EvRepoMoveStaReleased{}, nil
}

// `WorkflowEvent_EV_FSO_REPO_APP_ACCEPTED` aka `EvRepoMoveAppAccepted`
// indicates that Nogappd has acknowledged the repo move.
type EvRepoMoveAppAccepted struct{}

func (EvRepoMoveAppAccepted) WorkflowEvent() {}

func NewPbRepoMoveAppAccepted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED,
	}
}

func fromPbRepoMoveAppAccepted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED {
		panic("invalid event")
	}
	return &EvRepoMoveAppAccepted{}, nil
}

// `WorkflowEvent_EV_FSO_REPO_MOVED` aka `EvRepoMoved` completes a repo-move
// workflow.
type EvRepoMoved struct {
	RepoId     uuid.I
	GlobalPath string
	FileHost   string
	HostPath   string
	ShadowPath string
}

func (EvRepoMoved) WorkflowEvent() {}

func (ev *EvRepoMoved) validate() error {
	if ev.RepoId == uuid.Nil {
		return errors.New("nil RepoId")
	}
	if ev.GlobalPath == "" ||
		ev.FileHost == "" ||
		ev.HostPath == "" ||
		ev.ShadowPath == "" {
		return errors.New("some path is empty")
	}
	return nil
}

func NewPbRepoMoved(ev *EvRepoMoved) pb.WorkflowEvent {
	if err := ev.validate(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:  pb.WorkflowEvent_EV_FSO_REPO_MOVED,
		RepoId: ev.RepoId[:],
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.GlobalPath,
			FileHost:   ev.FileHost,
			HostPath:   ev.HostPath,
		},
		FsoShadowRepoInfo: &pb.FsoShadowRepoInfo{
			ShadowPath: ev.ShadowPath,
		},
	}
}

func fromPbRepoMoved(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_REPO_MOVED {
		panic("invalid event")
	}
	repoId, err := uuid.FromBytes(evpb.RepoId)
	if err != nil {
		return nil, err
	}
	return &EvRepoMoved{
		RepoId:     repoId,
		GlobalPath: evpb.FsoRepoInitInfo.GlobalPath,
		FileHost:   evpb.FsoRepoInitInfo.FileHost,
		HostPath:   evpb.FsoRepoInitInfo.HostPath,
		ShadowPath: evpb.FsoShadowRepoInfo.ShadowPath,
	}, nil
}

// `WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED` aka
// `EvRepoMoveCommitted` indicates that the workflow completed
// successfully.
type EvRepoMoveCommitted struct{}

func (EvRepoMoveCommitted) WorkflowEvent() {}

func NewPbRepoMoveCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED,
	}
}

func fromPbRepoMoveCommitted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED {
		panic("invalid event")
	}
	return &EvRepoMoveCommitted{}, nil
}
