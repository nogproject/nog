package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_FREEZE_REPO_STARTED_2` aka `EvFreezeRepoStarted2`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoStarted2 struct {
	RegistryId       uuid.I // only freezerepowf.
	RegistryName     string // only freezerepowf.
	StartRegistryVid ulid.I // only freezerepowf (optional).
	RepoId           uuid.I // only freezerepowf.
	StartRepoVid     ulid.I // only freezerepowf (optional).
	RepoGlobalPath   string // freezerepowf and workflow indexes.
	AuthorName       string // only freezerepowf.
	AuthorEmail      string // only freezerepowf.
	WorkflowId       uuid.I // only workflow indexes.
	WorkflowEventId  ulid.I // only workflow indexes.
}

func (EvFreezeRepoStarted2) WorkflowEvent() {}

func (ev *EvFreezeRepoStarted2) validateWorkflow() error {
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

func (ev *EvFreezeRepoStarted2) validateIndex() error {
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

func (ev *EvFreezeRepoStarted2) validateCommon() error {
	return nil
}

func NewPbFreezeRepoStarted2Workflow(ev *EvFreezeRepoStarted2) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	evpb := pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_FREEZE_REPO_STARTED_2,
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

func NewPbFreezeRepoStarted2Index(ev *EvFreezeRepoStarted2) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_FREEZE_REPO_STARTED_2,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.RepoGlobalPath,
		},
	}
}

func fromPbFreezeRepoStarted2(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_FREEZE_REPO_STARTED_2 {
		panic("invalid event")
	}
	ev := &EvFreezeRepoStarted2{}
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

// `WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_STARTED` aka `EvFreezeRepoFilesStarted`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoFilesStarted struct{}

func (EvFreezeRepoFilesStarted) WorkflowEvent() {}

func NewPbFreezeRepoFilesStarted() pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_STARTED,
	}
	return evpb
}

func fromPbFreezeRepoFilesStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_STARTED {
		panic("invalid event")
	}
	ev := &EvFreezeRepoFilesStarted{}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_COMPLETED` aka `EvFreezeRepoFilesCompleted`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoFilesCompleted struct {
	StatusCode    int32
	StatusMessage string
}

func (EvFreezeRepoFilesCompleted) WorkflowEvent() {}

func NewPbFreezeRepoFilesCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbFreezeRepoFilesCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbFreezeRepoFilesCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_FREEZE_REPO_FILES_COMPLETED {
		panic("invalid event")
	}
	ev := &EvFreezeRepoFilesCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2` aka `EvFreezeRepoCompleted2`.
// See freeze-repo workflow aka freezerepowf.
type EvFreezeRepoCompleted2 struct {
	StatusCode      int32  // only in freezerepowf, repos, and registry.
	StatusMessage   string // only in freezerepowf, repos, and registry.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvFreezeRepoCompleted2) WorkflowEvent() {}

func NewPbFreezeRepoCompleted2Ok() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbFreezeRepoCompleted2Error(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func NewPbFreezeRepoCompleted2IdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbFreezeRepoCompleted2(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMPLETED_2 {
		panic("invalid event")
	}
	ev := &EvFreezeRepoCompleted2{
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

// `WorkflowEvent_EV_FSO_FREEZE_REPO_COMMITTED` aka
// `EvFreezeRepoCommitted`.
type EvFreezeRepoCommitted struct{}

func (EvFreezeRepoCommitted) WorkflowEvent() {}

func NewPbFreezeRepoCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_FREEZE_REPO_COMMITTED,
	}
}

func fromPbFreezeRepoCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvFreezeRepoCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_FREEZE_REPO_DELETED` aka `EvFreezeRepoDeleted`.
// See split-root workflow aka splitrootwf.
type EvFreezeRepoDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvFreezeRepoDeleted) WorkflowEvent() {}

func NewPbFreezeRepoDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_FREEZE_REPO_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbFreezeRepoDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvFreezeRepoDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
