import { Mongo } from 'meteor/mongo';
import { Match, check } from 'meteor/check';
import {
  CollNameHomeLinks,
  KeyId,
  KeyPath,
  KeyRoute,
  PubNameHome,
} from './fso-home.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';

class FsoHomeLinkC {
  constructor(doc) { this.d = doc; }
  id() { return this.d[KeyId]; }
  route() { return this.d[KeyRoute]; }
  path() { return this.d[KeyPath]; }
}

function createHomeLinksCollection({ namespace }) {
  const name = makeCollName(namespace, CollNameHomeLinks);
  return new Mongo.Collection(name, {
    transform: doc => new FsoHomeLinkC(doc),
  });
}

function createCollections({ namespace }) {
  // Client-only collections.
  return {
    homeLinks: createHomeLinksCollection({ namespace }),
  };
}

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If we wanted to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function createSubscribeFuncs({ namespace, subscriber }) {
  return {
    subscribeHome() {
      const pubName = makePubName(namespace, PubNameHome);
      return subscriber.subscribe(pubName);
    },
  };
}

function createFsoHomeModuleClient({
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
  createFsoHomeModuleClient,
};
