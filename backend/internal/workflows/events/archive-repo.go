package events

import (
	"errors"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED` aka `EvArchiveRepoStarted`.
// See archive-repo workflow aka archiverepowf.
type EvArchiveRepoStarted struct {
	RegistryId       uuid.I // only archiverepowf.
	RegistryName     string // only archiverepowf.
	StartRegistryVid ulid.I // only archiverepowf (optional).
	RepoId           uuid.I // only archiverepowf.
	StartRepoVid     ulid.I // only archiverepowf (optional).
	RepoGlobalPath   string // archiverepowf and workflow indexes.
	AuthorName       string // only archiverepowf.
	AuthorEmail      string // only archiverepowf.
	WorkflowId       uuid.I // only workflow indexes.
	WorkflowEventId  ulid.I // only workflow indexes.
}

func (EvArchiveRepoStarted) WorkflowEvent() {}

func (ev *EvArchiveRepoStarted) validateWorkflow() error {
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

func (ev *EvArchiveRepoStarted) validateIndex() error {
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

func (ev *EvArchiveRepoStarted) validateCommon() error {
	return nil
}

func NewPbArchiveRepoStartedWorkflow(ev *EvArchiveRepoStarted) pb.WorkflowEvent {
	if err := ev.validateWorkflow(); err != nil {
		panic(err)
	}
	evpb := pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED,
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

func NewPbArchiveRepoStartedIndex(ev *EvArchiveRepoStarted) pb.WorkflowEvent {
	if err := ev.validateIndex(); err != nil {
		panic(err)
	}
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GlobalPath: ev.RepoGlobalPath,
		},
	}
}

func fromPbArchiveRepoStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED {
		panic("invalid event")
	}
	ev := &EvArchiveRepoStarted{}
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

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_STARTED` aka `EvArchiveRepoFilesStarted`.
// See archive-repo workflow aka archiverepowf.
type EvArchiveRepoFilesStarted struct {
	AclPolicy *pb.RepoAclPolicy
}

func (EvArchiveRepoFilesStarted) WorkflowEvent() {}

func NewPbArchiveRepoFilesStarted(aclPolicy *pb.RepoAclPolicy) pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_STARTED,
		RepoAclPolicy: aclPolicy,
	}
	return evpb
}

func fromPbArchiveRepoFilesStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_STARTED {
		panic("invalid event")
	}
	ev := &EvArchiveRepoFilesStarted{
		AclPolicy: evpb.RepoAclPolicy,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_TARTT_COMPLETED` aka
// `EvArchiveRepoTarttCompleted`.  See archive-repo workflow aka archiverepowf.
type EvArchiveRepoTarttCompleted struct {
	TarPath string
}

func (EvArchiveRepoTarttCompleted) WorkflowEvent() {}

func NewPbArchiveRepoTarttCompleted(tarPath string) pb.WorkflowEvent {
	if tarPath == "" {
		panic("empty tar path")
	}
	evpb := pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_TARTT_COMPLETED,
		TarttTarInfo: &pb.TarttTarInfo{
			Path: tarPath,
		},
	}
	return evpb
}

func fromPbArchiveRepoTarttCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_TARTT_COMPLETED {
		panic("invalid event")
	}
	ev := &EvArchiveRepoTarttCompleted{}
	if inf := evpb.TarttTarInfo; inf != nil {
		ev.TarPath = inf.Path
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_SWAP_STARTED` aka
// `EvArchiveRepoSwapStarted`.  See archive-repo workflow aka archiverepowf.
type EvArchiveRepoSwapStarted struct {
	WorkingDir string
}

func (EvArchiveRepoSwapStarted) WorkflowEvent() {}

func NewPbArchiveRepoSwapStarted(wd string) pb.WorkflowEvent {
	evpb := pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_SWAP_STARTED,
		WorkingDir: wd,
	}
	return evpb
}

func fromPbArchiveRepoSwapStarted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_SWAP_STARTED {
		panic("invalid event")
	}
	ev := &EvArchiveRepoSwapStarted{
		WorkingDir: evpb.WorkingDir,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED` aka `EvArchiveRepoFilesCompleted`.
// See archive-repo workflow aka archiverepowf.
type EvArchiveRepoFilesCompleted struct {
	StatusCode    int32
	StatusMessage string
}

func (EvArchiveRepoFilesCompleted) WorkflowEvent() {}

func NewPbArchiveRepoFilesCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbArchiveRepoFilesCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func fromPbArchiveRepoFilesCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED {
		panic("invalid event")
	}
	ev := &EvArchiveRepoFilesCompleted{
		StatusCode:    evpb.StatusCode,
		StatusMessage: evpb.StatusMessage,
	}
	return ev, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMMITTED` aka
// `EvArchiveRepoFilesCommitted`.
type EvArchiveRepoFilesCommitted struct{}

func (EvArchiveRepoFilesCommitted) WorkflowEvent() {}

func NewPbArchiveRepoFilesCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMMITTED,
	}
}

func fromPbArchiveRepoFilesCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvArchiveRepoFilesCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` aka `EvArchiveRepoCompleted`.
// See archive-repo workflow aka archiverepowf.
type EvArchiveRepoCompleted struct {
	StatusCode      int32  // only in archiverepowf, repos, and registry.
	StatusMessage   string // only in archiverepowf, repos, and registry.
	WorkflowId      uuid.I // only in workflow indexes.
	WorkflowEventId ulid.I // only in workflow indexes.
}

func (EvArchiveRepoCompleted) WorkflowEvent() {}

func NewPbArchiveRepoCompletedOk() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		StatusCode:    0,
		StatusMessage: "",
	}
}

func NewPbArchiveRepoCompletedError(code int32, message string) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:         pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		StatusCode:    code,
		StatusMessage: message,
	}
}

func NewPbArchiveRepoCompletedIdRef(id uuid.I, vid ulid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:           pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED,
		WorkflowId:      id[:],
		WorkflowEventId: vid[:],
	}
}

func fromPbArchiveRepoCompleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	if evpb.Event != pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED {
		panic("invalid event")
	}
	ev := &EvArchiveRepoCompleted{
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

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_GC_COMPLETED` aka
// `EvArchiveRepoGcCompleted`.
type EvArchiveRepoGcCompleted struct{}

func (EvArchiveRepoGcCompleted) WorkflowEvent() {}

func NewPbArchiveRepoGcCompleted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_GC_COMPLETED,
	}
}

func fromPbArchiveRepoGcCompleted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvArchiveRepoGcCompleted{}, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMMITTED` aka
// `EvArchiveRepoCommitted`.
type EvArchiveRepoCommitted struct{}

func (EvArchiveRepoCommitted) WorkflowEvent() {}

func NewPbArchiveRepoCommitted() pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event: pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMMITTED,
	}
}

func fromPbArchiveRepoCommitted(
	evpb *pb.WorkflowEvent,
) (WorkflowEvent, error) {
	return &EvArchiveRepoCommitted{}, nil
}

// `WorkflowEvent_EV_FSO_ARCHIVE_REPO_DELETED` aka `EvArchiveRepoDeleted`.
// See split-root workflow aka splitrootwf.
type EvArchiveRepoDeleted struct {
	WorkflowId uuid.I // only in workflow indexes.
}

func (EvArchiveRepoDeleted) WorkflowEvent() {}

func NewPbArchiveRepoDeleted(id uuid.I) pb.WorkflowEvent {
	return pb.WorkflowEvent{
		Event:      pb.WorkflowEvent_EV_FSO_ARCHIVE_REPO_DELETED,
		WorkflowId: id[:],
	}
}

func fromPbArchiveRepoDeleted(evpb *pb.WorkflowEvent) (WorkflowEvent, error) {
	ev := &EvArchiveRepoDeleted{}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, err
	}
	ev.WorkflowId = id
	return ev, nil
}
