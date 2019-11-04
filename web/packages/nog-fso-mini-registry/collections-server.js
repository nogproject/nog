import { Mongo } from 'meteor/mongo';

import {
  makeCollName,
  KeyId,
  KeyFsoId,
  KeyVid,
  KeyName,
  KeyPath,
  KeyRegistryId,
} from './collections.js';

class FsoRegistry {
  constructor(doc) {
    const d = { ...doc };
    if (d[KeyVid]) {
      d[KeyVid] = Buffer.from(d[KeyVid]);
    }
    this.d = d;
  }

  id() { return this.d[KeyId]; }

  vid() { return this.d[KeyVid]; }

  name() { return this.d[KeyName]; }
}

class FsoRepo {
  constructor(doc) {
    const d = { ...doc };
    if (d[KeyVid]) {
      d[KeyVid] = Buffer.from(d[KeyVid]);
    }
    if (d[KeyFsoId]) {
      d[KeyFsoId] = Buffer.from(d[KeyFsoId]);
    }
    this.d = d;
  }

  id() { return this.d[KeyId]; }

  fsoId() { return this.d[KeyFsoId]; }

  path() { return this.d[KeyPath]; }

  registryId() { return this.d[KeyRegistryId]; }
}

function createCollectionsServer({ namespace }) {
  const regN = makeCollName(namespace, 'registries');
  const registries = new Mongo.Collection(regN, {
    transform: doc => new FsoRegistry(doc),
  });
  registries.rawCollection().createIndex({ [KeyName]: 1 }, { unique: true });

  const repos = new Mongo.Collection(makeCollName(namespace, 'repos'), {
    transform: doc => new FsoRepo(doc),
  });
  repos.rawCollection().createIndex({ [KeyPath]: 1 }, { unique: true });
  repos.rawCollection().createIndex({ [KeyFsoId]: 1 }, { unique: true });

  return {
    registries,
    repos,
  };
}

export {
  createCollectionsServer,
};
