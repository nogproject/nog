/* global Assets */
import { Meteor } from 'meteor/meteor';
import protobuf from 'protobufjs';
protobuf.loadSync = Meteor.wrapAsync(protobuf.load, protobuf);
import grpc from 'grpc';

// Explicitly load as Protobufjs 6.  Decode enums as integers.
// See `grpc.LoadObject()`, <https://grpc.io/grpc/node/grpc.html>.
function addProto(pb, path) {
  const raw = protobuf.loadSync(Assets.absoluteFilePath(path));
  return Object.assign(pb, grpc.loadObject(raw, {
    protobufjsVersion: 6,
    binaryAsBase64: false,  // Use Buffer.
    enumsAsStrings: false,  // Use Number.
    longsAsStrings: true,  // Strings, not objects.
  }).nogfso);
}

const pb = {};
addProto(pb, 'proto/nogfsopb/gitnog.proto');
addProto(pb, 'proto/nogfsopb/gitnogtree.proto');
addProto(pb, 'proto/nogfsopb/registry.proto');
addProto(pb, 'proto/nogfsopb/repos.proto');
addProto(pb, 'proto/nogfsopb/stat.proto');
addProto(pb, 'proto/nogfsopb/broadcast.proto');
addProto(pb, 'proto/nogfsopb/discovery.proto');
addProto(pb, 'proto/nogfsopb/tartt.proto');

// Export full proto as `pb` and selected details by name.
const {
  GitNog,
  GitNogTree,
  Registry,
  RegistryEvent,
  RepoEvent,
  WorkflowEvent,
  Repos,
  Stat,
  Broadcast,
  BroadcastEvent,
  Discovery,
  Tartt,
  SubdirTracking,
  JobControl,
  PathStatus,
} = pb;

const {
  EV_FSO_REGISTRY_ADDED,
  EV_FSO_ROOT_ADDED,
  EV_FSO_ROOT_REMOVED,
  EV_FSO_ROOT_UPDATED,
  EV_FSO_REPO_ACCEPTED,
  EV_FSO_REPO_ADDED,
  EV_FSO_REPO_REINIT_ACCEPTED,
  EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED,
  EV_FSO_REPO_NAMING_UPDATED,
  EV_FSO_REPO_NAMING_CONFIG_UPDATED,
  EV_FSO_REPO_INIT_POLICY_UPDATED,
  EV_FSO_REPO_MOVE_ACCEPTED,
  EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED,
  EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED,
} = RegistryEvent.Type.values;

const {
  EV_FSO_REPO_INIT_STARTED,
  EV_FSO_SHADOW_REPO_CREATED,
  EV_FSO_GIT_REPO_CREATED,
  EV_FSO_GIT_TO_NOG_CLONED,
  EV_FSO_REPO_ERROR_SET,
  EV_FSO_REPO_ERROR_CLEARED,
  EV_FSO_ENABLE_GITLAB_ACCEPTED,
  EV_FSO_REPO_MOVE_STARTED,
  EV_FSO_REPO_MOVED,
  EV_FSO_ARCHIVE_RECIPIENTS_UPDATED,
  EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED,
} = RepoEvent.Type.values;

const {
  EV_FSO_REPO_MOVE_STA_RELEASED,
  EV_FSO_REPO_MOVE_APP_ACCEPTED,
  EV_FSO_REPO_MOVE_COMMITTED,
} = WorkflowEvent.Type.values;

const {
  EV_BC_FSO_MAIN_CHANGED,
  EV_BC_FSO_REGISTRY_CHANGED,
  EV_BC_FSO_REPO_CHANGED,
  EV_BC_FSO_GIT_REF_UPDATED,
} = BroadcastEvent.Type.values;

const {
  ST_ENTER_SUBDIRS,
  ST_BUNDLE_SUBDIRS,
  ST_IGNORE_SUBDIRS,
  ST_IGNORE_MOST,
} = SubdirTracking.values;

const {
  PS_NEW,
  PS_MODIFIED,
  PS_DELETED,
} = PathStatus.Status.values;

const {
  JC_WAIT,
  JC_BACKGROUND,
} = JobControl.values;

const {
  TAR_FULL,
  TAR_PATCH,
} = pb.TarInfo.TarType.values;

export {
  // Re-export `grpc`, since it is a bit tricky to import.
  grpc,

  pb,

  GitNog,
  GitNogTree,
  Registry,
  Repos,
  Stat,
  Broadcast,
  Discovery,
  Tartt,

  RegistryEvent,
  EV_FSO_REGISTRY_ADDED,
  EV_FSO_ROOT_ADDED,
  EV_FSO_ROOT_REMOVED,
  EV_FSO_ROOT_UPDATED,
  EV_FSO_REPO_ACCEPTED,
  EV_FSO_REPO_ADDED,
  EV_FSO_REPO_REINIT_ACCEPTED,
  EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED,
  EV_FSO_REPO_NAMING_UPDATED,
  EV_FSO_REPO_NAMING_CONFIG_UPDATED,
  EV_FSO_REPO_INIT_POLICY_UPDATED,
  EV_FSO_REPO_MOVE_ACCEPTED,
  EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED,
  EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED,

  RepoEvent,
  EV_FSO_REPO_INIT_STARTED,
  EV_FSO_SHADOW_REPO_CREATED,
  EV_FSO_GIT_REPO_CREATED,
  EV_FSO_GIT_TO_NOG_CLONED,
  EV_FSO_REPO_ERROR_SET,
  EV_FSO_REPO_ERROR_CLEARED,
  EV_FSO_ENABLE_GITLAB_ACCEPTED,
  EV_FSO_REPO_MOVE_STARTED,
  EV_FSO_REPO_MOVED,
  EV_FSO_ARCHIVE_RECIPIENTS_UPDATED,
  EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED,

  WorkflowEvent,
  EV_FSO_REPO_MOVE_STA_RELEASED,
  EV_FSO_REPO_MOVE_APP_ACCEPTED,
  EV_FSO_REPO_MOVE_COMMITTED,

  BroadcastEvent,
  EV_BC_FSO_MAIN_CHANGED,
  EV_BC_FSO_REGISTRY_CHANGED,
  EV_BC_FSO_REPO_CHANGED,
  EV_BC_FSO_GIT_REF_UPDATED,

  // enum SubdirTracking
  ST_ENTER_SUBDIRS,
  ST_BUNDLE_SUBDIRS,
  ST_IGNORE_SUBDIRS,
  ST_IGNORE_MOST,

  // enum JobControl
  JC_WAIT,
  JC_BACKGROUND,

  // enum PathStatus.Status
  PS_NEW,
  PS_MODIFIED,
  PS_DELETED,

  // enum TarInfo.TarType
  TAR_FULL,
  TAR_PATCH,
};
