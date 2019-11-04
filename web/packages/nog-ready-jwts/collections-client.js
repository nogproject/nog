import { Mongo } from 'meteor/mongo';
import {
  CollNameReadyJwts,
  KeyDescription,
  KeyId,
  KeyPath,
  KeyTitle,
  makeCollName,
} from './collections.js';

class ReadyJwtC {
  constructor(doc) {
    const d = { ...doc };
    this.d = d;
  }

  id() { return this.d[KeyId]; }

  title() { return this.d[KeyTitle]; }

  description() { return this.d[KeyDescription]; }

  path() { return this.d[KeyPath]; }
}

function createCollectionsClient({ namespace }) {
  const readyJwtsName = makeCollName(namespace, CollNameReadyJwts);
  const readyJwts = new Mongo.Collection(readyJwtsName, {
    transform: doc => new ReadyJwtC(doc),
  });

  return {
    readyJwts,
  };
}

export {
  createCollectionsClient,
};
