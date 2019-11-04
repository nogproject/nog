import { Mongo } from 'meteor/mongo';
import { check } from 'meteor/check';

import {
  CollNameReadyJwts,
  KeyDescription,
  KeyExpiresIn,
  KeyId,
  KeyPath,
  KeyScopes,
  KeySubuser,
  KeyTitle,
  makeCollName,
} from './collections.js';
import {
  matchReadyJwt,
} from './match.js';

class ReadyJwt {
  constructor(doc) {
    const d = { ...doc };
    this.d = d;
  }

  id() { return this.d[KeyId]; }

  title() { return this.d[KeyTitle]; }

  description() { return this.d[KeyDescription]; }

  path() { return this.d[KeyPath]; }

  subuser() { return this.d[KeySubuser]; }

  expiresIn() { return this.d[KeyExpiresIn]; }

  scopes() { return this.d[KeyScopes]; }
}

function createCollectionsServer({ namespace }) {
  const readyJwtsName = makeCollName(namespace, CollNameReadyJwts);
  const readyJwts = new Mongo.Collection(readyJwtsName, {
    transform: doc => new ReadyJwt(doc),
  });
  readyJwts.rawCollection().createIndex({ [KeyPath]: 1 }, { unique: true });

  function upsertReadyJwt(spec) {
    check(spec, matchReadyJwt);
    const {
      title,
      description,
      path,
      subuser,
      expiresIn,
      scopes,
    } = spec;
    readyJwts.upsert({ [KeyPath]: path }, {
      $set: {
        [KeyTitle]: title,
        [KeyDescription]: description,
        [KeyPath]: path,
        [KeySubuser]: subuser,
        [KeyExpiresIn]: expiresIn,
        [KeyScopes]: scopes,
      },
    });
  }

  function findOneReadyJwtByPath(path) {
    check(path, String);
    return readyJwts.findOne({ [KeyPath]: path });
  }

  return {
    readyJwts,
    upsertReadyJwt,
    findOneReadyJwtByPath,
  };
}

export {
  createCollectionsServer,
};
