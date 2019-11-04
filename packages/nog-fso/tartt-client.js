import { Mongo } from 'meteor/mongo';
import { check, Match } from 'meteor/check';
import {
  CollNameRepoTars,
  CollNameTarttHeads,
  KeyAuthor,
  KeyCommitter,
  KeyId,
  KeyTarType,
  KeyTime,
  PubNameTartt,
} from './tartt.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

class TarttHeadC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  author() {
    const { name: n, email: e } = this.d[KeyAuthor];
    return `${n} <${e}>`;
  }
  authorDate() {
    return this.d[KeyAuthor].date;
  }
  committer() {
    const { name: n, email: e } = this.d[KeyCommitter];
    return `${n} <${e}>`;
  }
  committerDate() {
    return this.d[KeyCommitter].date;
  }
}

function createTarttHeadsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameTarttHeads);
  return new Mongo.Collection(name, {
    transform: doc => new TarttHeadC(doc),
  });
}

class RepoTarC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  time() { return this.d[KeyTime]; }
  tarType() { return this.d[KeyTarType]; }
}

function createRepoTarsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameRepoTars);
  return new Mongo.Collection(name, {
    transform: doc => new RepoTarC(doc),
  });
}

function createCollections({ namespace }) {
  return {
    tarttHeads: createTarttHeadsCollection({ namespace }),
    repoTars: createRepoTarsCollection({ namespace }),
  };
}

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If we wanted to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function createSubscribeFuncs({ namespace, subscriber }) {
  return {
    subscribeTartt(opts) {
      check(opts, {
        path: String,
      });
      const pubName = makePubName(namespace, PubNameTartt);
      return subscriber.subscribe(pubName, opts);
    },
  };
}

function createTarttModuleClient({
  namespace, testAccess, subscriber,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(subscriber, Match.ObjectIncluding({ subscribe: Function }));

  return {
    ...createCollections({ namespace }),
    ...createSubscribeFuncs({ namespace, subscriber }),
  };
}

export {
  createTarttModuleClient,
};
