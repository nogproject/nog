import { check, Match } from 'meteor/check';
import { Mongo } from 'meteor/mongo';
import {
  CollNameDiscoveryErrors,
  CollNameRoots,
  CollNameUntracked,
  KeyDiscoveryErrorMessage,
  KeyGlobalRootPath,
  KeyId,
  KeyRegistryName,
  KeyUntrackedGlobalPath,
  KeyUntrackedStatus,
  PubNameRoots,
  PubNameUntracked,
  UntrackedStatus,
} from './fso-discovery.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

class FsoDiscoveryRootC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  globalRootPath() { return this.d[KeyGlobalRootPath]; }
  registryName() { return this.d[KeyRegistryName]; }
}

function createRootsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameRoots);
  return new Mongo.Collection(name, {
    transform: doc => new FsoDiscoveryRootC(doc),
  });
}

class FsoUntrackedC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  globalPath() { return this.d[KeyUntrackedGlobalPath]; }
  registryName() { return this.d[KeyRegistryName]; }
  status() { return this.d[KeyUntrackedStatus]; }
  isCandidate() { return this.status() === UntrackedStatus.Candidate; }
  isIgnored() { return this.status() === UntrackedStatus.Ignored; }
}

function createUntrackedCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameUntracked);
  return new Mongo.Collection(name, {
    transform: doc => new FsoUntrackedC(doc),
  });
}

class FsoDiscoveryErrorC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  message() { return this.d[KeyDiscoveryErrorMessage]; }
}

function createDiscoveryErrorsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameDiscoveryErrors);
  return new Mongo.Collection(name, {
    transform: doc => new FsoDiscoveryErrorC(doc),
  });
}

function createCollections({ namespace }) {
  // These are all client-only collections.
  return {
    roots: createRootsCollection({ namespace }),
    untracked: createUntrackedCollection({ namespace }),
    discoveryErrors: createDiscoveryErrorsCollection({ namespace }),
  };
}

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If we wanted to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function createSubscribeFuncs({ namespace, subscriber }) {
  return {
    subscribeRoots(opts) {
      check(opts, { prefix: String });
      const pubName = makePubName(namespace, PubNameRoots);
      return subscriber.subscribe(pubName, opts);
    },
    subscribeUntracked(opts) {
      check(opts, {
        registry: String,
        globalRoot: String,
        nonce: Match.Maybe(String),
      });
      const pubName = makePubName(namespace, PubNameUntracked);
      return subscriber.subscribe(pubName, opts);
    },
  };
}

function createFsoDiscoverModuleClient({
  namespace, testAccess, subscriber,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(subscriber, Match.ObjectIncluding({ subscribe: Function }));

  const module = {
    ...createCollections({ namespace }),
    ...createSubscribeFuncs({ namespace, subscriber }),
  };
  return module;
}

export {
  createFsoDiscoverModuleClient,
};
