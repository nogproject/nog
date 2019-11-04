package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

type RepoEvent interface {
	RepoEvent()
}

func FromPbValidate(evpb pb.RepoEvent) (ev RepoEvent, err error) {
	switch evpb.Event {
	case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
		return fromPbRepoInitStarted(evpb)

	case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
		return fromPbEnableGitlabAccepted(evpb)

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
		return fromPbShadowRepoCreated(evpb)

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return fromPbShadowRepoMoveStarted(evpb)

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED:
		return fromPbShadowRepoMoved(evpb)

	case pb.RepoEvent_EV_FSO_REPO_MOVE_STARTED:
		return fromPbRepoMoveStarted(evpb)

	case pb.RepoEvent_EV_FSO_REPO_MOVED:
		return fromPbRepoMoved(evpb)

	case pb.RepoEvent_EV_FSO_TARTT_REPO_CREATED:
		return fromPbTarttRepoCreated(evpb)

	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_CREATED:
		return fromPbShadowBackupRepoCreated(evpb)

	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_REPO_MOVED:
		return fromPbShadowBackupRepoMoved(evpb)

	case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
		return fromPbGitRepoCreated(evpb)

	case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
		return fromPbRepoErrorSet(evpb)

	case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
		return fromPbRepoErrorCleared(evpb)

	case pb.RepoEvent_EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
		return fromPbArchiveRecipientsUpdated(evpb)

	case pb.RepoEvent_EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		return fromPbShadowBackupRecipientsUpdated(evpb)

	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED:
		return fromPbFreezeRepoStarted(evpb)

	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED:
		return fromPbFreezeRepoCompleted(evpb)

	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return fromPbFreezeRepoStarted2(evpb)

	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return fromPbFreezeRepoCompleted2(evpb)

	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED:
		return fromPbUnfreezeRepoStarted(evpb)

	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED:
		return fromPbUnfreezeRepoCompleted(evpb)

	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return fromPbUnfreezeRepoStarted2(evpb)

	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return fromPbUnfreezeRepoCompleted2(evpb)

	case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return fromPbArchiveRepoStarted(evpb)

	case pb.RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return fromPbArchiveRepoCompleted(evpb)

	case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return fromPbUnarchiveRepoStarted(evpb)

	case pb.RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return fromPbUnarchiveRepoCompleted(evpb)

	default:
		return nil, ErrUnknownEventType
	}
}

func FromPbMust(evpb pb.RepoEvent) RepoEvent {
	ev, err := FromPbValidate(evpb)
	if err != nil {
		panic(err)
	}
	return ev
}
