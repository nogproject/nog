import { Mongo } from 'meteor/mongo';
import { check, Match } from 'meteor/check';
import {
  CollNameListingErrors,
  CollNameListingNodes,
  KeyId,
  KeyListingErrorMessage,
  KeyListingErrorSeverity,
  KeyPath,
  PubNameListing,
  Severity,
} from './fso-list.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

class FsoListNodeC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  path() { return this.d[KeyPath]; }
  isDir() { return this.path().endsWith('/'); }
  isRepo() { return !this.isDir(); }
}

function createListingNodesCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameListingNodes);
  return new Mongo.Collection(name, {
    transform: doc => new FsoListNodeC(doc),
  });
}

class FsoListingErrorC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  message() { return this.d[KeyListingErrorMessage]; }
  severity() { return this.d[KeyListingErrorSeverity]; }
  isWarning() { return this.severity() === Severity.Warning; }
}

function createListingErrorsCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameListingErrors);
  return new Mongo.Collection(name, {
    transform: doc => new FsoListingErrorC(doc),
  });
}

function createCollections({ namespace }) {
  // Client-only collections.
  return {
    listingNodes: createListingNodesCollection({ namespace }),
    listingErrors: createListingErrorsCollection({ namespace }),
  };
}

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If we wanted to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function createSubscribeFuncs({ namespace, subscriber }) {
  return {
    subscribeListing(opts) {
      check(opts, {
        path: String,
        recursive: Boolean,
        nonce: Match.Optional(String),
      });
      const pubName = makePubName(namespace, PubNameListing);
      return subscriber.subscribe(pubName, opts);
    },
  };
}

function createFsoListModuleClient({
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
  createFsoListModuleClient,
};
