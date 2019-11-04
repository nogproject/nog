// - `CollName*` are collection basenames.
// - `Key*` are MongoDB field names.

const CollNameReadyJwts = 'readyjwts';

// `KeyId` is the Mongo document ID.
const KeyId = '_id';

// `KeyTitle` is the name for the UI, like `BCPFS Admin (prod)`.
const KeyTitle = 't';

// `KeyDescription` is the description for the UI, like `A BCPFS Admin (prod)
// token provides full access to /bcp and /bsmol.`.
const KeyDescription = 'd';

// `KeyPath` is the ready JWT specification path, like `/sys/jwts/admin/foo`.
const KeyPath = 'p';

// `KeySubuser` is the subuser for the token subject.
const KeySubuser = 'u';

// `KeyExpiresIn` is the token validity period in seconds.
const KeyExpiresIn = 'e';

// `KeyScopes` is the list of scopes.
const KeyScopes = 'sc';

function makeCollName(namespace, basename) {
  return `${namespace.coll}.${basename}`;
}

export {
  CollNameReadyJwts,
  KeyDescription,
  KeyExpiresIn,
  KeyId,
  KeyPath,
  KeyScopes,
  KeySubuser,
  KeyTitle,
  makeCollName,
};
