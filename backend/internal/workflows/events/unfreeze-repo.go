package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2` aka `EvUnfreezeRepoStarted2`.
// See unfreeze-repo workflow aka unfreezerepowf.
type EvUnfreezeRepoStarted2 struct {
	RegistryId       uuid.I // only unfreezerepowf.
	RegistryName     string // only unfreezerepowf.
	StartRegistryVid ulid.I // only unfreezerepowf (optional).
	RepoId           uuid.I // only unfreezerepowf.
	StartRepoVid     ulid.I // only unfreezerepowf (optional).
	RepoGlobalPath   string // unfreezerepowf and workflow indexes.
	AuthorName       string // only unfreezerepowf.
	AuthorEmail      string // only unfreezerepowf.
	WorkflowId       uuid.I // only workflow indexes.
	WorkflowEventId  ulid.I // only workflow indexes.
}

func (EvUnfreezeRepoStarted2) WorkflowEvent() {}

func (ev *EvUnfreezeRepoStarted2) validateWorkflow() error {
	if ev.RegistryId == uuid.Nil {
		return errors.New("nil RegistryId")
	}
	if ev.RegistryName == "" {
		return errors.New("empty RegistryName")
	}
	// StartRegistryVid may be nil.
	if ev.RepoId == uuid.Nil {
		return errors.New("nil RepoId")
	}
	// StartRepoVid may be nil.
	if ev.RepoGlobalPath == "" {
		return errors.New("empty RepoGlobalPath")
	}
	if ev.AuthorName == "" {
		return errors.New("empty AuthorName")
	}
	if ev.AuthorEmail == "" {
		return errors.New("empty AuthorEmail")
	}
	if ev.WorkflowId != uuid.Nil {
		return errors.New("non-nil WorkflowId")
	}
	if ev.WorkflowEventId != ulid.Nil {
		return errors.New("non-nil WorkflowEventId")
	}
	return ev.validateCommon()
}

func (ev *EvUnfreezeRepoStarted2) validateIndex() error {
	if ev.RegistryId != uuid.Nil {
		return errors.New("non-nil RegistryId")
	}
	if ev.RegistryName != "" {
		return errors.New("non-empty RegistryName")
	}
	if ev.StartRegistryVid != ulid.Nil {
		return errors.New("non-nil StartRegistryVid")
	}
	if ev.RepoId != uuid.Nil {
		return errors.New("non-nil RepoId")
	}
	if ev.StartRepoVid != ulid.Nil {
		return errors.New("non-nil StartRepoVid")
	}
	if ev.RepoGlobalPath == "" {
		return errors.New("empty RepoGlobalPath")
	}
	if ev.AuthorName != "" {
		return errors.New("non-empty AuthorName")
	}
	if ev.AuthorEmail != "" {
		return errors.New("non-empty AuthorEmail")
	}
	if ev.WorkflowId == uuid.Nil {
		return errors.New("nil WorkflowId")
	}
	if ev.WorkflowEventId == ulid.Nil {
		return errors.New("nil WorkflowEventId")
	}
	return ev.validateCommon()
}

func (ev *EvUnfreezeRepoStarted2) validateCommon() error {
	return nil
}

func NewPbUnfreezeRepoStarted2Workflow(ev *EvUnfreezeRepoStarted2) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	evpb := pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2,
		RegistryId:      ev.RegistryId[:],
		FsoRegistryName: ev.RegistryName,
		RepoId:          ev.RepoId[:],
		GitAuthor: &pb.GitUser{
			Name:  ev.AuthorName,
			Email: ev.AuthorEmail,
		},
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.RepoGlobalPath,
		},
	}
	if ev.StartRegistryVid != ulid.Nil {
		evpb.RegistryEventId = ev.StartRegistryVid[:]
	}
	if ev.StartRepoVid != ulid.Nil {
		evpb.RepoEventId = ev.StartRepoVid[:]
	}
	return evpb
}

func NewPbUnfreezeRepoStarted2Index(ev *EvUnfreezeRepoStarted2) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.RepoGlobalPath,
		},
	}
}

func fromPbUnfreezeRepoStarted2(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2 {
		panic("invalid event")
	}
	ev := &EvUnfreezeRepoStarted2{}
	if evpb.RegistryId != nil {
		id, err := uuid.FromBytes(evpb.RegistryId)
		if err != nil {
			return nil, err
		}
		ev.RegistryId = id
	}
	ev.RegistryName = evpb.FsoRegistryName
	if evpb.RegistryEventId != nil {
		vid, err := ulid.ParseBytes(evpb.RegistryEventId)
		if err != nil {
			return nil, err
		}
		ev.StartRegistryVid = vid
	}
	if evpb.RepoId != nil {
		id, err := uuid.FromBytes(evpb.RepoId)
		if err != nil {
			return nil, err
		}
		ev.RepoId = id
	}
	if evpb.RepoEventId != nil {
		vid, err := ulid.ParseBytes(evpb.RepoEventId)
		if err != nil {
			return nil, err
		}
		ev.StartRepoVid = vid
	}
	if inf := evpb.FsoRepoInitInfo; inf != nil {
		ev.RepoGlobalPath = inf.GlobalPath
	}
	if a := evpb.GitAuthor; a != nil {
		ev.AuthorName = a.Name
		ev.AuthorEmail = a.Email
	}
	if evpb.WorkflowId != nil {
		id, err := uuid.FromBytes(evpb.WorkflowId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowId = id
	}
	if evpb.WorkflowEventId != nil {
		vid, err := ulid.ParseBytes(evpb.WorkflowEventId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowEventId = vid
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_STARTED` aka `EvUnfreezeRepoFilesStarted`.
// See unfreeze-repo workflow aka unfreezerepowf.
type EvUnfreezeRepoFilesStarted struct{}

func (EvUnfreezeRepoFilesStarted) WorkflowEvent() {}

func NewPbUnfreezeRepoFilesStarted() pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_STARTED,
	}
	return evpb
}

func fromPbUnfreezeRepoFilesStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_STARTED {
		panic("invalid event")
	}
	ev := &EvUnfreezeRepoFilesStarted{}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_COMPLETED` aka `EvUnfreezeRepoFilesCompleted`.
// See unfreeze-repo workflow aka unfreezerepowf.
type EvUnfreezeRepoFilesCompleted struct {
	StatusCode    int32
	StatusMessage string
}

func (EvUnfreezeRepoFilesCompleted) WorkflowEvent() {}

func NewPbUnfreezeRepoFilesCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbUnfreezeRepoFilesCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbUnfreezeRepoFilesCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_COMPLETED {
		panic("invalid event")
	}
	ev := &EvUnfreezeRepoFilesCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2` aka `EvUnfreezeRepoCompleted2`.
// See unfreeze-repo workflow aka unfreezerepowf.
type EvUnfreezeRepoCompleted2 struct {
	StatusCode      int32  // only in unfreezerepowf, repos, and registry.
	StatusMessage   string // only in unfreezerepowf, repos, and registry.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvUnfreezeRepoCompleted2) WorkflowEvent() {}

func NewPbUnfreezeRepoCompleted2Ok() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbUnfreezeRepoCompleted2Error(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func NewPbUnfreezeRepoCompleted2IdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbUnfreezeRepoCompleted2(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2 {
		panic("invalid event")
	}
	ev := &EvUnfreezeRepoCompleted2{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	if evpb.WorkflowId != nil {
		id, err := uuid.FromBytes(evpb.WorkflowId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowId = id
	}
	if evpb.WorkflowEventId != nil {
		vid, err := ulid.ParseBytes(evpb.WorkflowEventId)
		if err != nil {
			return nil, err
		}
		ev.WorkflowEventId = vid
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMMITTED` aka
// `EvUnfreezeRepoCommitted`.
type EvUnfreezeRepoCommitted struct{}

func (EvUnfreezeRepoCommitted) WorkflowEvent() {}

func NewPbUnfreezeRepoCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMMITTED,
	}
}

func fromPbUnfreezeRepoCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvUnfreezeRepoCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_UNFREEZE_REPO_DELETED` aka `EvUnfreezeRepoDeleted`.
// See split-root workflow aka splitrootwf.
type EvUnfreezeRepoDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvUnfreezeRepoDeleted) WorkflowEvent() {}

func NewPbUnfreezeRepoDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_UNFREEZE_REPO_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbUnfreezeRepoDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvUnfreezeRepoDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
