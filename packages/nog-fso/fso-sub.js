import { check } from 'meteor/check';
import { makePubName } from './fso-pubsub.js';

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If the need arose to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function createSubscribeFuncs({ namespace, subscriber }) {
  return {
    subscribeRepo({ repoName }) {
      check(repoName, String);
      return subscriber.subscribe(makePubName(namespace, 'repo'), {
        repoName,
      });
    },
  };
}

export {
  createSubscribeFuncs,
};
