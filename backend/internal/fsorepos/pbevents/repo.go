package pbevents

import (
	"bytes"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `RepoEvent_EV_FSO_REPO_INIT_STARTED` aka `EvRepoInitStarted` is the event of
// a repo aggregate.
type EvRepoInitStarted struct {
	pb.FsoRepoInitInfo
}

func (EvRepoInitStarted) RepoEvent() {}

func NewRepoInitStarted(inf *pb.FsoRepoInitInfo) pb.RepoEvent {
	return pb.RepoEvent{
		Event:           pb.RepoEvent_EV_FSO_REPO_INIT_STARTED,
		FsoRepoInitInfo: inf,
	}
}

func fromPbRepoInitStarted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_REPO_INIT_STARTED {
		panic("invalid event")
	}
	return &EvRepoInitStarted{
		FsoRepoInitInfo: *evpb.FsoRepoInitInfo,
	}, nil
}

// `RepoEvent_EV_FSO_SHADOW_REPO_CREATED` aka `EvShadowRepoCreated` is posted
// after the shadow has been created.  It contains details about the repo
// location.
type EvShadowRepoCreated struct {
	pb.FsoShadowRepoInfo
}

func (EvShadowRepoCreated) RepoEvent() {}

func NewShadowRepoCreated(shadowPath string) pb.RepoEvent {
	return pb.RepoEvent{
		Event: pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED,
		FsoShadowRepoInfo: &pb.FsoShadowRepoInfo{
			ShadowPath: shadowPath,
		},
	}
}

func fromPbShadowRepoCreated(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED {
		panic("invalid event")
	}
	return &EvShadowRepoCreated{
		FsoShadowRepoInfo: *evpb.FsoShadowRepoInfo,
	}, nil
}

// `RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED` aka `EvShadowRepoMoveStarted`
// starts a workflow that changes the shadow location.
//
// The workflow ends with:
//
//  - If committed, `RepoEvent_EV_FSO_SHADOW_REPO_MOVED` aka
//    `EvShadowRepoMoved`.
//  - There is currently no way to abort the workflow.
//
type EvShadowRepoMoveStarted struct {
	WorkflowId    uuid.I
	NewShadowPath string
}

func (EvShadowRepoMoveStarted) RepoEvent() {}

func NewShadowRepoMoveStarted(
	workflowId uuid.I, newShadowPath string,
) pb.RepoEvent {
	return pb.RepoEvent{
		Event:      pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED,
		WorkflowId: workflowId[:],
		FsoShadowRepoInfo: &pb.FsoShadowRepoInfo{
			NewShadowPath: newShadowPath,
		},
	}
}

func fromPbShadowRepoMoveStarted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED {
		panic("invalid event")
	}
	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	return &EvShadowRepoMoveStarted{
		WorkflowId:    id,
		NewShadowPath: evpb.FsoShadowRepoInfo.NewShadowPath,
	}, nil
}

// `RepoEvent_EV_FSO_SHADOW_REPO_MOVED` aka `EvShadowRepoMoved` completes a
// workflow that started with `RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED` aka
// `EvShadowRepoMoveStarted`.
type EvShadowRepoMoved struct {
	WorkflowId uuid.I

	// `WorkflowEventId` may be `ulid.Nil` for legacy events that were not
	// replicated from a workflow event.
	WorkflowEventId ulid.I

	NewShadowPath string
}

func (EvShadowRepoMoved) RepoEvent() {}

func NewShadowRepoMoved(
	workflowId uuid.I, workflowEventId ulid.I, newShadowPath string,
) pb.RepoEvent {
	return pb.RepoEvent{
		Event:           pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED,
		WorkflowId:      workflowId[:],
		WorkflowEventId: workflowEventId[:],
		FsoShadowRepoInfo: &pb.FsoShadowRepoInfo{
			NewShadowPath: newShadowPath,
		},
	}
}

func fromPbShadowRepoMoved(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED {
		panic("invalid event")
	}

	id, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}

	// Legacy events may lack `WorkflowEventId`.
	workflowEventId := ulid.Nil
	if evpb.WorkflowEventId != nil {
		i, err := ulid.ParseBytes(evpb.WorkflowEventId)
		if err != nil {
			return nil, err
		}
		workflowEventId = i
	}

	return &EvShadowRepoMoved{
		WorkflowId:      id,
		WorkflowEventId: workflowEventId,
		NewShadowPath:   evpb.FsoShadowRepoInfo.NewShadowPath,
	}, nil
}

// `RepoEvent_EV_FSO_REPO_MOVE_STARTED` aka `EvRepoMoveStarted` is part of the
// move-repo workflow.  See package `moverepowf` for details.
type EvRepoMoveStarted struct {
	RegistryEventId ulid.I
	WorkflowId      uuid.I
	OldGlobalPath   string
	OldFileHost     string
	OldHostPath     string
	OldShadowPath   string
	NewGlobalPath   string
	NewFileHost     string
	NewHostPath     string
}

func (EvRepoMoveStarted) RepoEvent() {}

func NewPbRepoMoveStarted(ev *EvRepoMoveStarted) pb.RepoEvent {
	return pb.RepoEvent{
		Event:           pb.RepoEvent_EV_FSO_REPO_MOVE_STARTED,
		RegistryEventId: ev.RegistryEventId[:],
		WorkflowId:      ev.WorkflowId[:],
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

func fromPbRepoMoveStarted(evpb pb.RepoEvent) (RepoEvent, error) {
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	registryEventId, err := ulid.ParseBytes(evpb.RegistryEventId)
	if err != nil {
		return nil, &ParseError{What: "registry event ID", Err: err}
	}
	return &EvRepoMoveStarted{
		WorkflowId:      workflowId,
		RegistryEventId: registryEventId,
		OldGlobalPath:   evpb.FsoRepoInitInfo.GlobalPath,
		OldFileHost:     evpb.FsoRepoInitInfo.FileHost,
		OldHostPath:     evpb.FsoRepoInitInfo.HostPath,
		OldShadowPath:   evpb.FsoShadowRepoInfo.ShadowPath,
		NewGlobalPath:   evpb.NewFsoRepoInitInfo.GlobalPath,
		NewFileHost:     evpb.NewFsoRepoInitInfo.FileHost,
		NewHostPath:     evpb.NewFsoRepoInitInfo.HostPath,
	}, nil
}

// `RepoEvent_EV_FSO_REPO_MOVED` aka `EvRepoMoved` is part of the move-repo
// workflow.  See package `moverepowf` for details.
type EvRepoMoved struct {
	WorkflowId      uuid.I
	WorkflowEventId ulid.I
	GlobalPath      string
	FileHost        string
	HostPath        string
	ShadowPath      string
}

func (EvRepoMoved) RepoEvent() {}

func NewPbRepoMoved(ev *EvRepoMoved) pb.RepoEvent {
	return pb.RepoEvent{
		Event:           pb.RepoEvent_EV_FSO_REPO_MOVED,
		WorkflowId:      ev.WorkflowId[:],
		WorkflowEventId: ev.WorkflowEventId[:],
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

func fromPbRepoMoved(evpb pb.RepoEvent) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_REPO_MOVED {
		panic("invalid event")
	}
	workflowId, err := uuid.FromBytes(evpb.WorkflowId)
	if err != nil {
		return nil, &ParseError{What: "workflow ID", Err: err}
	}
	workflowEventId, err := ulid.ParseBytes(evpb.WorkflowEventId)
	if err != nil {
		return nil, &ParseError{What: "workflow event ID", Err: err}
	}
	return &EvRepoMoved{
		WorkflowId:      workflowId,
		WorkflowEventId: workflowEventId,
		GlobalPath:      evpb.FsoRepoInitInfo.GlobalPath,
		FileHost:        evpb.FsoRepoInitInfo.FileHost,
		HostPath:        evpb.FsoRepoInitInfo.HostPath,
		ShadowPath:      evpb.FsoShadowRepoInfo.ShadowPath,
	}, nil
}

// `RepoEvent_EV_FSO_TARTT_REPO_CREATED` aka `EvTarttRepoCreated` confirms that
// a tartt repo has been created.  It contains the tartt location as a
// `tartt://` URL.
//
// `RepoEvent_EV_FSO_TARTT_REPO_CREATED` is currently the only event related to
// tartt repo initialization.  Its name is chosen such that a corresponding
// event could be added in the future that would indicate the start of the
// tartt repo creation process:
//
//  - future registry event `tartt repo init started`, which would reserve the
//    URL;
//  - repos event `RepoEvent_EV_FSO_TARTT_REPO_CREATED` would then indicate
//    completion of the inititialization process.
//
// Their relation would be similar to:
//
//  - registry event `RepoEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED`;
//  - repos event `RepoEvent_EV_FSO_GIT_REPO_CREATED`.
//
type EvTarttRepoCreated struct {
	pb.FsoArchiveRepoInfo
}

func (EvTarttRepoCreated) RepoEvent() {}

func NewTarttRepoCreated(tarttURL string) pb.RepoEvent {
	return pb.RepoEvent{
		Event: pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED,
		FsoArchiveRepoInfo: &pb.FsoArchiveRepoInfo{
			ArchiveUrl: tarttURL,
		},
	}
}

func fromPbTarttRepoCreated(evpb pb.RepoEvent) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED {
		panic("invalid event")
	}
	return &EvTarttRepoCreated{
		FsoArchiveRepoInfo: *evpb.FsoArchiveRepoInfo,
	}, nil
}

// `RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED` aka
// `EvShadowBackupRepoCreated` confirms that a backup directory has been
// created.  It contains the location as a `nogfsobak://` URL.
//
// See `EvTarttRepoCreated` for discussion about potential future two-phase
// initialization.
type EvShadowBackupRepoCreated struct {
	pb.FsoShadowBackupRepoInfo
}

func (EvShadowBackupRepoCreated) RepoEvent() {}

func NewShadowBackupRepoCreated(shadowBackupURL string) pb.RepoEvent {
	return pb.RepoEvent{
		Event: pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED,
		FsoShadowBackupRepoInfo: &pb.FsoShadowBackupRepoInfo{
			ShadowBackupUrl: shadowBackupURL,
		},
	}
}

func fromPbShadowBackupRepoCreated(evpb pb.RepoEvent) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED {
		panic("invalid event")
	}
	return &EvShadowBackupRepoCreated{
		FsoShadowBackupRepoInfo: *evpb.FsoShadowBackupRepoInfo,
	}, nil
}

// `RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED` aka `EvShadowBackupRepoMoved`
// changes the location of the backup directory.  It contains the location as a
// `nogfsobak://` URL.
type EvShadowBackupRepoMoved struct {
	pb.FsoShadowBackupRepoInfo
}

func (EvShadowBackupRepoMoved) RepoEvent() {}

func NewShadowBackupRepoMoved(shadowBackupURL string) pb.RepoEvent {
	return pb.RepoEvent{
		Event: pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED,
		FsoShadowBackupRepoInfo: &pb.FsoShadowBackupRepoInfo{
			ShadowBackupUrl: shadowBackupURL,
		},
	}
}

func fromPbShadowBackupRepoMoved(evpb pb.RepoEvent) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED {
		panic("invalid event")
	}
	return &EvShadowBackupRepoMoved{
		FsoShadowBackupRepoInfo: *evpb.FsoShadowBackupRepoInfo,
	}, nil
}

// `RepoEvent_EV_FSO_GIT_REPO_CREATED` aka `EvGitRepoCreated` is posted if a
// Git repo at a Git hosting service is created.
type EvGitRepoCreated struct {
	pb.FsoGitRepoInfo
}

func (EvGitRepoCreated) RepoEvent() {}

func NewGitRepoCreated(inf *pb.FsoGitRepoInfo) pb.RepoEvent {
	return pb.RepoEvent{
		Event:          pb.RepoEvent_EV_FSO_GIT_REPO_CREATED,
		FsoGitRepoInfo: inf,
	}
}

func fromPbGitRepoCreated(evpb pb.RepoEvent) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_GIT_REPO_CREATED {
		panic("invalid event")
	}
	return &EvGitRepoCreated{
		FsoGitRepoInfo: *evpb.FsoGitRepoInfo,
	}, nil
}

// `RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED` aka `EvEnableGitlabAccepted`.
type EvEnableGitlabAccepted struct {
	GitlabHost string
	GitlabPath string
}

func (EvEnableGitlabAccepted) RepoEvent() {}

func NewEnableGitlabAccepted(host, path string) pb.RepoEvent {
	return pb.RepoEvent{
		Event: pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED,
		FsoRepoInitInfo: &pb.FsoRepoInitInfo{
			GitlabHost: host,
			GitlabPath: path,
		},
	}
}

func fromPbEnableGitlabAccepted(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED {
		panic("invalid event")
	}
	return &EvEnableGitlabAccepted{
		GitlabHost: evpb.FsoRepoInitInfo.GitlabHost,
		GitlabPath: evpb.FsoRepoInitInfo.GitlabPath,
	}, nil
}

// `RepoEvent_EV_FSO_REPO_ERROR_SET` aka `EvRepoErrorSet` sets an error string
// on the repo.
type EvRepoErrorSet struct {
	Message string
}

func (EvRepoErrorSet) RepoEvent() {}

func NewRepoErrorSet(msg string) pb.RepoEvent {
	return pb.RepoEvent{
		Event:               pb.RepoEvent_EV_FSO_REPO_ERROR_SET,
		FsoRepoErrorMessage: msg,
	}
}

func fromPbRepoErrorSet(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_REPO_ERROR_SET {
		panic("invalid event")
	}
	return &EvRepoErrorSet{
		Message: evpb.FsoRepoErrorMessage,
	}, nil
}

// `RepoEvent_EV_FSO_REPO_ERROR_CLEARED` aka `EvRepoErrorCleared` clears an
// repo error.
type EvRepoErrorCleared struct{}

func (EvRepoErrorCleared) RepoEvent() {}

func NewRepoErrorCleared() pb.RepoEvent {
	return pb.RepoEvent{
		Event: pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED,
	}
}

func fromPbRepoErrorCleared(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	if evpb.Event != pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED {
		panic("invalid event")
	}
	return &EvRepoErrorCleared{}, nil
}

// `RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED` aka
// `EvArchiveRecipientsUpdated` replaces the GPG key fingerprints to which
// tartt archive secrets are encrypted.
type EvArchiveRecipientsUpdated struct {
	Keys [][]byte
}

func (EvArchiveRecipientsUpdated) RepoEvent() {}

func NewArchiveRecipientsUpdated(keys [][]byte) pb.RepoEvent {
	for _, k := range keys {
		if len(k) != 20 {
			panic("invalid GPG key fingerprint")
		}
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if bytes.Equal(keys[i], keys[j]) {
				panic("duplicate GPG key fingerprints")
			}
		}
	}

	return pb.RepoEvent{
		Event:                 pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED,
		FsoGpgKeyFingerprints: keys,
	}
}

func fromPbArchiveRecipientsUpdated(evpb pb.RepoEvent) (RepoEvent, error) {
	ev := &EvArchiveRecipientsUpdated{
		Keys: evpb.FsoGpgKeyFingerprints,
	}
	for _, k := range ev.Keys {
		if len(k) != 20 {
			return nil, ErrMalformedGPGFingerprint
		}
	}
	return ev, nil
}

// `RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED` aka
// `EvShadowBackupRecipientsUpdated` replaces the GPG key fingerprints to which
// archive backups are encrypted.
type EvShadowBackupRecipientsUpdated struct {
	Keys [][]byte
}

func (EvShadowBackupRecipientsUpdated) RepoEvent() {}

func NewShadowBackupRecipientsUpdated(keys [][]byte) pb.RepoEvent {
	for _, k := range keys {
		if len(k) != 20 {
			panic("invalid GPG key fingerprint")
		}
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if bytes.Equal(keys[i], keys[j]) {
				panic("duplicate GPG key fingerprints")
			}
		}
	}

	return pb.RepoEvent{
		Event:                 pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED,
		FsoGpgKeyFingerprints: keys,
	}
}

func fromPbShadowBackupRecipientsUpdated(
	evpb pb.RepoEvent,
) (RepoEvent, error) {
	ev := &EvShadowBackupRecipientsUpdated{
		Keys: evpb.FsoGpgKeyFingerprints,
	}
	for _, k := range ev.Keys {
		if len(k) != 20 {
			return nil, ErrMalformedGPGFingerprint
		}
	}
	return ev, nil
}
