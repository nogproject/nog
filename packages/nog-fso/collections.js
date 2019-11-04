function makeCollName(namespace, basename) {
  return `${namespace.coll}.${basename}`;
}

// `Key*` are MongoDB field names.  More in `./fso-discovery.js`.

// `KeyId` is the Mongo id.
const KeyId = '_id';

// `KeyFsoId` is an FSO UUID.
const KeyFsoId = 'fid';

// `KeyVid` is the event sourcing journal version id.
const KeyVid = 'vid';

// `KeyName` is the repo or registry name.
const KeyName = 'n';

// `KeyRegistryId` is a Mongo id pointer from repo to registry.
const KeyRegistryId = 'rgid';

// `KeyGitlabHost` is a host like `git.zib.de`.
const KeyGitlabHost = 'glh';

// `KeyGitlabPath` is a GitLab path to the repo, like `exfso/_._example_._foo`.
const KeyGitlabPath = 'glp';

// `KeyGitlabProjectId` is a GitLab project id.  It is stored as a string,
// although GitLab uses an integer.
const KeyGitlabProjectId = 'glid';

// `KeyGitlabUrl` is a GitLab UI project URL that can be used as an href in the
// UI.  Example: `https://git.example.org/exfs/repo/tree/master-stat`.
const KeyGitlabUrl = 'glur';

// `KeyFilesSummary` is a summary of the GitLab repo files.
const KeyFilesSummary = 'fss';

// `KeyReadme` is the readme status and text of a repo.
const KeyReadme = 'rme';

// `KeyStatRequested` is the time of the last stat request through the UI.
const KeyStatRequested = 'stt';

// `KeyStatStatus` is an object `{ nNew, nModified, nDeleted, ... }` that
// contains the number of `StatStatus()` paths with the respective status.
// The additional `...` fields are:
//
//  - `ts`: the time when the stat status operation was started.
//  - `changes` and `changesIsComplete`: a limited list of `{ path, status }`.
//    `status = ? | M | D` for new, modified, and deleted.  `changesIsComplete`
//    indicates whether the list is complete.
//
const KeyStatStatus = 'sts';

// `KeyRefreshContentRequested` is the time of the last content request through
// the UI.
const KeyRefreshContentRequested = 'cot';

// `KeyErrorMessage` is a per-repo error message for users.
const KeyErrorMessage = 'err';

// DEPRECATED: `KeyMeta` was a list of generic `{k, v}` metadata.  The fields
// will be replaced by `KeyMetadata` during publishing.
const KeyMeta = 'm';
// `KeyMetadata` is an object to store metadata (`kvs`) and the update status
// (`isUpdating`).
const KeyMetadata = 'md';

// `KeyMetaCommitId` is the Git commit ID of the metadata.
const KeyMetaCommitId = 'mco';

// `KeyGitNogHost` is the GRPC `nogfso.GitNog` service host.
const KeyGitNogHost = 'gnh';

// `KeyGitNogCommit` is the GitNog head commit information.
const KeyGitNogCommit = 'gnco';

export {
  KeyErrorMessage,
  KeyFilesSummary,
  KeyFsoId,
  KeyGitNogCommit,
  KeyGitNogHost,
  KeyGitlabHost,
  KeyGitlabPath,
  KeyGitlabProjectId,
  KeyGitlabUrl,
  KeyId,
  KeyMeta,
  KeyMetadata,
  KeyMetaCommitId,
  KeyName,
  KeyReadme,
  KeyRefreshContentRequested,
  KeyRegistryId,
  KeyStatRequested,
  KeyStatStatus,
  KeyVid,
  makeCollName,
};
