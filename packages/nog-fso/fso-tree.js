// - `AA_*` are nog-access actions.
// - `Key*` are MongoDB field names.
// - `PubName*` are publication basenames.
// - `CollName*` are collection basenames.

// `AA_FSO_READ_REPO_TREE` is the action that controls access to reading FSO
// repo trees, which includes stat information and per-path metadata.
const AA_FSO_READ_REPO_TREE = 'fso/read-repo-tree';

// `PubNameTree` is the basename of the publication that publishes an FSO tree
// listing for a repo.
const PubNameTree = 'tree';

// `PubNameTreePathContent` is the basename of the publication that publishes
// the content for an FSO tree path.
const PubNameTreePathContent = 'treePathContent';

// `CollNameFiles` is the basename of the client-side collection for FSO tree
// file information.
const CollNameFiles = 'files';

// `CollNameContent` is the basename of the client-side collection for FSO tree
// path content.
const CollNameContent = 'content';

// `CollNameTreeErrors` is the basename of the client-side Meteor collection to
// publish tree-related errors to the UI.
const CollNameTreeErrors = 'treeErrors';

// `KeyId` is the Mongo id.
const KeyId = '_id';

// `KeyRepoName` is a repo name.
const KeyRepoName = 'rpn';

// `KeyTreePath` is a path relative to the root of a tree.  `.` denotes the
// root.
const KeyTreePath = 'tp';

// `KeyStatInfo` is a GRPC `GitNogTree.ListStatTree()` file info object.
const KeyStatInfo = 'st';

// `KeyMeta` is a tree path metadata object.
const KeyMeta = 'meta';

// `KeyTreeErrorMessage` is a tree-related error message for the UI.
const KeyTreeErrorMessage = 'err';

// `KeyContent` is file content.
const KeyContent = 'c';

export {
  AA_FSO_READ_REPO_TREE,
  CollNameContent,
  CollNameFiles,
  CollNameTreeErrors,
  KeyContent,
  KeyId,
  KeyMeta,
  KeyRepoName,
  KeyStatInfo,
  KeyTreeErrorMessage,
  KeyTreePath,
  PubNameTree,
  PubNameTreePathContent,
};
