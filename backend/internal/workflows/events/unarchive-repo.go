package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED` aka `EvUnarchiveRepoStarted`.
// See unarchive-repo workflow aka unarchiverepowf.
type EvUnarchiveRepoStarted struct {
	RegistryId       uuid.I // only unarchiverepowf.
	RegistryName     string // only unarchiverepowf.
	StartRegistryVid ulid.I // only unarchiverepowf (optional).
	RepoId           uuid.I // only unarchiverepowf.
	StartRepoVid     ulid.I // only unarchiverepowf (optional).
	RepoGlobalPath   string // unarchiverepowf and workflow indexes.
	RepoArchiveURL   string // only unarchiverepowf.
	TarttTarPath     string // only unarchiverepowf.
	AuthorName       string // only unarchiverepowf.
	AuthorEmail      string // only unarchiverepowf.
	WorkflowId       uuid.I // only workflow indexes.
	WorkflowEventId  ulid.I // only workflow indexes.
}

func (EvUnarchiveRepoStarted) WorkflowEvent() {}

func (ev *EvUnarchiveRepoStarted) validateWorkflow() error {
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
	if ev.RepoArchiveURL == "" {
		return errors.New("empty RepoArchiveURL")
	}
	if ev.TarttTarPath == "" {
		return errors.New("empty TarttTarPath")
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

func (ev *EvUnarchiveRepoStarted) validateIndex() error {
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
	if ev.RepoArchiveURL != "" {
		return errors.New("non-empty RepoArchiveURL")
	}
	if ev.TarttTarPath != "" {
		return errors.New("non-empty TarttTarPath")
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

func (ev *EvUnarchiveRepoStarted) validateCommon() error {
	return nil
}

func NewPbUnarchiveRepoStartedWorkflow(ev *EvUnarchiveRepoStarted) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	evpb := pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED,
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
		FsoArchiveRepoInfo: &pb.FsoArchiveRepoInfo{
			ArchiveUrl: ev.RepoArchiveURL,
		},
		TarttTarInfo: &pb.TarttTarInfo{
			Path: ev.TarttTarPath,
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

func NewPbUnarchiveRepoStartedIndex(ev *EvUnarchiveRepoStarted) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.RepoGlobalPath,
		},
	}
}

func fromPbUnarchiveRepoStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED {
		panic("invalid event")
	}
	ev := &EvUnarchiveRepoStarted{}
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
	if inf := evpb.FsoArchiveRepoInfo; inf != nil {
		ev.RepoArchiveURL = inf.ArchiveUrl
	}
	if inf := evpb.TarttTarInfo; inf != nil {
		ev.TarttTarPath = inf.Path
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

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_STARTED` aka `EvUnarchiveRepoFilesStarted`.
// See unarchive-repo workflow aka unarchiverepowf.
type EvUnarchiveRepoFilesStarted struct {
	AclPolicy *pb.RepoAclPolicy
}

func (EvUnarchiveRepoFilesStarted) WorkflowEvent() {}

func NewPbUnarchiveRepoFilesStarted(aclPolicy *pb.RepoAclPolicy) pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_STARTED,
		RepoAclPolicy: aclPolicy,
	}
	return evpb
}

func fromPbUnarchiveRepoFilesStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_STARTED {
		panic("invalid event")
	}
	ev := &EvUnarchiveRepoFilesStarted{
		AclPolicy: evpb.RepoAclPolicy,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_STARTED` aka
// `EvUnarchiveRepoTarttStarted`.  See unarchive-repo workflow aka unarchiverepowf.
type EvUnarchiveRepoTarttStarted struct {
	WorkingDir string
}

func (EvUnarchiveRepoTarttStarted) WorkflowEvent() {}

func NewPbUnarchiveRepoTarttStarted(wd string) pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_STARTED,
		WorkingDir: wd,
	}
	return evpb
}

func fromPbUnarchiveRepoTarttStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_STARTED {
		panic("invalid event")
	}
	ev := &EvUnarchiveRepoTarttStarted{
		WorkingDir: evpb.WorkingDir,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_COMPLETED` aka
// `EvUnarchiveRepoTarttCompleted`.  See unarchive-repo workflow aka unarchiverepowf.
type EvUnarchiveRepoTarttCompleted struct {
	StatusCode    int32
	StatusMessage string
}

func (EvUnarchiveRepoTarttCompleted) WorkflowEvent() {}

func NewPbUnarchiveRepoTarttCompletedOk() pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
	return evpb
}

func NewPbUnarchiveRepoTarttCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbUnarchiveRepoTarttCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_COMPLETED {
		panic("invalid event")
	}
	ev := &EvUnarchiveRepoTarttCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMPLETED` aka `EvUnarchiveRepoFilesCompleted`.
// See unarchive-repo workflow aka unarchiverepowf.
type EvUnarchiveRepoFilesCompleted struct {
	StatusCode    int32
	StatusMessage string
}

func (EvUnarchiveRepoFilesCompleted) WorkflowEvent() {}

func NewPbUnarchiveRepoFilesCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbUnarchiveRepoFilesCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbUnarchiveRepoFilesCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMPLETED {
		panic("invalid event")
	}
	ev := &EvUnarchiveRepoFilesCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMMITTED` aka
// `EvUnarchiveRepoFilesCommitted`.
type EvUnarchiveRepoFilesCommitted struct{}

func (EvUnarchiveRepoFilesCommitted) WorkflowEvent() {}

func NewPbUnarchiveRepoFilesCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMMITTED,
	}
}

func fromPbUnarchiveRepoFilesCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvUnarchiveRepoFilesCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` aka `EvUnarchiveRepoCompleted`.
// See unarchive-repo workflow aka unarchiverepowf.
type EvUnarchiveRepoCompleted struct {
	StatusCode      int32  // only in unarchiverepowf, repos, and registry.
	StatusMessage   string // only in unarchiverepowf, repos, and registry.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvUnarchiveRepoCompleted) WorkflowEvent() {}

func NewPbUnarchiveRepoCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbUnarchiveRepoCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func NewPbUnarchiveRepoCompletedIdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbUnarchiveRepoCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED {
		panic("invalid event")
	}
	ev := &EvUnarchiveRepoCompleted{
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

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_GC_COMPLETED` aka
// `EvUnarchiveRepoGcCompleted`.
type EvUnarchiveRepoGcCompleted struct{}

func (EvUnarchiveRepoGcCompleted) WorkflowEvent() {}

func NewPbUnarchiveRepoGcCompleted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_GC_COMPLETED,
	}
}

func fromPbUnarchiveRepoGcCompleted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvUnarchiveRepoGcCompleted{}, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMMITTED` aka
// `EvUnarchiveRepoCommitted`.
type EvUnarchiveRepoCommitted struct{}

func (EvUnarchiveRepoCommitted) WorkflowEvent() {}

func NewPbUnarchiveRepoCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMMITTED,
	}
}

func fromPbUnarchiveRepoCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvUnarchiveRepoCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_DELETED` aka `EvUnarchiveRepoDeleted`.
// See split-root workflow aka splitrootwf.
type EvUnarchiveRepoDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvUnarchiveRepoDeleted) WorkflowEvent() {}

func NewPbUnarchiveRepoDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_UNARCHIVE_REPO_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbUnarchiveRepoDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvUnarchiveRepoDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
