// - `AA_*` are nog-access actions.
// - `Key*` are MongoDB field names.
// - `PubName*` are publication basenames.
// - `CollName*` are collection basenames.

// `AA_FSO_DISCOVER` controls basic access to try to discover roots below a
// prefix.  Individual roots additionally require `AA_FSO_DISCOVER_ROOT`.
//
// Enabling additional paths requires `AA_FSO_ENABLE_DISCOVERY_PATH`: users can
// be allowed to enable paths separate from the general root config.
const AA_FSO_DISCOVER = 'fso/discover';
const AA_FSO_DISCOVER_ROOT = 'fso/discover-root';
const AA_FSO_ENABLE_DISCOVERY_PATH = 'fso/enable-discovery-path';

// `PubNameRoots` is the basename of the FSO roots Meteor publication.
const PubNameRoots = 'roots';
// `CollNameRoots` is the basename of the client-side FSO roots Meteor
// collection.
const CollNameRoots = 'roots';

// `PubNameUntracked` is the basename of the FSO repo candidates Meteor
// publication.
const PubNameUntracked = 'untracked';
// `CollNameUntracked` is the basename of the client-side Meteor collection
// that contains untracked paths.
const CollNameUntracked = 'untracked';

// `CollNameDiscoveryErrors` is the basename of the client-side Meteor
// collection to publish discovery-related errors to the UI.
const CollNameDiscoveryErrors = 'discoveryErrors';

// `KeyId` is the Mongo id.
const KeyId = '_id';

// `KeyGlobalRootPath` is a global path of an FSO root, which can also be used
// as its id.
const KeyGlobalRootPath = 'grp';

// `KeyRegistryName` is a registry name, which is also its id in FSO GRPCs.
// The registry has a different id in MongoDB; see `KeyRegistryId`.
const KeyRegistryName = 'rn';

// `KeyDiscoveryErrorMessage` is a discovery-related error message for the UI.
const KeyDiscoveryErrorMessage = 'err';

// `KeyUntrackedGlobalPath` is the global path of a potential FSO repo that is
// currently untracked.
const KeyUntrackedGlobalPath = 'ugp';

// `KeyUntrackedStatus` is the status of an untracked path.  `UntrackedStatus`
// enumerates the status codes.
const KeyUntrackedStatus = 'ust';
const UntrackedStatus = {
  Candidate: 'Candidate',
  Ignored: 'Ignored',
};

export {
  AA_FSO_DISCOVER,
  AA_FSO_DISCOVER_ROOT,
  AA_FSO_ENABLE_DISCOVERY_PATH,
  CollNameDiscoveryErrors,
  CollNameRoots,
  CollNameUntracked,
  KeyDiscoveryErrorMessage,
  KeyGlobalRootPath,
  KeyId,
  KeyRegistryName,
  KeyUntrackedGlobalPath,
  KeyUntrackedStatus,
  PubNameRoots,
  PubNameUntracked,
  UntrackedStatus,
};
