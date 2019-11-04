package pbevents

import (
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

type RegistryEvent interface {
	RegistryEvent()
}

func FromPbValidate(evpb pb.RegistryEvent) (ev RegistryEvent, err error) {
	switch evpb.Event {
	case pb.RegistryEvent_EV_FSO_REGISTRY_ADDED:
		return fromPbRegistryAdded(evpb)

	case pb.RegistryEvent_EV_EPHEMERAL_WORKFLOWS_ENABLED:
		return fromPbEphemeralWorkflowsEnabled(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_ACL_POLICY_UPDATED:
		return fromPbRepoAclPolicyUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_ROOT_ADDED:
		return fromPbRootAdded(evpb)

	case pb.RegistryEvent_EV_FSO_ROOT_REMOVED:
		return fromPbRootRemoved(evpb)

	case pb.RegistryEvent_EV_FSO_ROOT_UPDATED:
		return fromPbRootRemoved(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_NAMING_UPDATED:
		return fromPbRepoNamingUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_NAMING_CONFIG_UPDATED:
		return fromPbRepoNamingConfigUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_INIT_POLICY_UPDATED:
		return fromPbRepoInitPolicyUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED:
		return fromPbRootArchiveRecipientsUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED:
		return fromPbRootShadowBackupRecipientsUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_ACCEPTED:
		return fromPbRepoAccepted(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED:
		return fromPbRepoMoveAccepted(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_MOVED:
		return fromPbRepoMoved(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		return fromPbRepoAdded(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
		return fromPbRepoReinitAccepted(evpb)

	case pb.RegistryEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		return fromPbShadowRepoMoveStarted(evpb)

	case pb.RegistryEvent_EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
		return fromPbRepoEnableGitlabAccepted(evpb)

	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED:
		return fromPbSplitRootEnabled(evpb)

	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_PARAMS_UPDATED:
		return fromPbSplitRootParamsUpdated(evpb)

	case pb.RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED:
		return fromPbSplitRootDisabled(evpb)

	case pb.RegistryEvent_EV_FSO_PATH_FLAG_SET:
		return fromPbPathFlagSet(evpb)

	case pb.RegistryEvent_EV_FSO_PATH_FLAG_UNSET:
		return fromPbPathFlagUnset(evpb)

	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_STARTED_2:
		return fromPbFreezeRepoStarted2(evpb)

	case pb.RegistryEvent_EV_FSO_FREEZE_REPO_COMPLETED_2:
		return fromPbFreezeRepoCompleted2(evpb)

	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_STARTED_2:
		return fromPbUnfreezeRepoStarted2(evpb)

	case pb.RegistryEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2:
		return fromPbUnfreezeRepoCompleted2(evpb)

	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED:
		return fromPbArchiveRepoStarted(evpb)

	case pb.RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED:
		return fromPbArchiveRepoCompleted(evpb)

	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED:
		return fromPbUnarchiveRepoStarted(evpb)

	case pb.RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED:
		return fromPbUnarchiveRepoCompleted(evpb)

	default:
		return nil, ErrUnknownEventType
	}
}

func FromPbMust(evpb pb.RegistryEvent) RegistryEvent {
	ev, err := FromPbValidate(evpb)
	if err != nil {
		panic(err)
	}
	return ev
}
