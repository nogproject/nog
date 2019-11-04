// - `AA_*` are nog-access actions.
// - `Key*` are MongoDB field names.
// - `PubName*` are publication basenames.
// - `CollName*` are collection basenames.

// `AA_FSO_HOME` controls access to the home dashboard.  `path` is always `/`.
const AA_FSO_HOME = 'fso/home';

// `PubNameHome` publishes FSO home links for the current user.
const PubNameHome = 'home';

// `CollNameHomeLinks` is a client-side collection with the home links of the
// current user.
const CollNameHomeLinks = 'homeLinks';

// `KeyId` is a MongoDB key.
const KeyId = '_id';
// `KeyPath` is an FSO path.
const KeyPath = 'pth';
// `KeyRoute` is a home route.  It must be translated before it can be used
// with the router.
const KeyRoute = 'rt';

export {
  AA_FSO_HOME,
  CollNameHomeLinks,
  KeyId,
  KeyPath,
  KeyRoute,
  PubNameHome,
};
