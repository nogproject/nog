function makeCollName(namespace, basename) {
  return `${namespace.coll}.${basename}`;
}

// `Key*` are MongoDB field names.

// `KeyId` is the Mongo document ID.
const KeyId = '_id';

// `KeyFsoId` is an FSO UUID.
const KeyFsoId = 'fid';

// `KeyVid` is the event sourcing journal version ID.
const KeyVid = 'vid';

// `KeyName` is the registry name.
const KeyName = 'n';

// `KeyPath` is the repo path.
const KeyPath = 'p';

// `KeyRegistryId` is the Mongo ID pointer from repo to registry.
const KeyRegistryId = 'rgid';

export {
  makeCollName,
  KeyId,
  KeyFsoId,
  KeyVid,
  KeyName,
  KeyPath,
  KeyRegistryId,
};
