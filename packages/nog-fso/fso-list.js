// - `AA_*` are nog-access actions.
// - `Key*` are MongoDB field names.
// - `PubName*` are publication basenames.
// - `CollName*` are collection basenames.

// `AA_FSO_LIST_REPOS` controls listing of a single level.  For example, with
// opts `{ path: /example/foo/ }`, it would list dirs and repos as if path was
// a directory on a filesystem.
const AA_FSO_LIST_REPOS = 'fso/list-repos';

// `AA_FSO_LIST_REPOS_RECURSIVE` controls listing, similar to
// `AA_FSO_LIST_REPOS`.  But it permits recursive listing of repos below a
// path.
const AA_FSO_LIST_REPOS_RECURSIVE = 'fso/list-repos-recursive';

const PubNameListing = 'listing';

const CollNameListingNodes = 'listing';
const CollNameListingErrors = 'listingErrors';

const KeyId = '_id';
const KeyPath = 'pth';

const KeyListingErrorMessage = 'err';
const KeyListingErrorSeverity = 'sev';
const Severity = {
  Warning: 'Warning',
};

export {
  AA_FSO_LIST_REPOS,
  AA_FSO_LIST_REPOS_RECURSIVE,
  CollNameListingErrors,
  CollNameListingNodes,
  KeyId,
  KeyListingErrorMessage,
  KeyListingErrorSeverity,
  KeyPath,
  PubNameListing,
  Severity,
};
