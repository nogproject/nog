// Tartt state is cached in two collections:
//
//  - `CollNameTarttHeads`: the current `master-tartt` head info for each repo.
//    The doc id is the Meteor id that is also used for the main repos
//    collection.
//  - `CollNameRepoTars`: the tars as returned by gRPC `Tartt.ListTars()`.
//
// `publishTartt()` `./tartt-server.js` immediately returns MongoDB cursors
// that publish the cached state for the subscribed repo.  It also polls in the
// background for changes while the publication is active.  A poll first calls
// `Tartt.Head()` to determine if the cached state is up to date.  If so, it
// stops.  Otherwise it calls `Tartt.ListTars()`, upserts the response tars
// into `CollNameRepoTars`, removes outdated tars whose commit does not match
// the current head.  Finally, it updates `CollNameTarttHeads` to indicate to
// the next poll that the state is up to date.

const PubNameTartt = 'tartt';

const CollNameRepoTars = 'repoTars';
const CollNameTarttHeads = 'tarttHeads';

const KeyId = '_id';
const KeyRepoId = 'rid';
const KeyPath = 'pth';
const KeyTarType = 'tty';
const KeyTime = 'tim';
const KeyTarttCommit = 'tco';
const KeyAuthor = 'aut';
const KeyCommitter = 'com';

const TarType = {
  Full: 'Full',
  Patch: 'Patch',
};

export {
  CollNameRepoTars,
  CollNameTarttHeads,
  KeyAuthor,
  KeyCommitter,
  KeyId,
  KeyPath,
  KeyRepoId,
  KeyTarType,
  KeyTarttCommit,
  KeyTime,
  PubNameTartt,
  TarType,
};
