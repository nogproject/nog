import {
  PubNameReadyJwts,
  PubNameUserTokens,
  makePubName,
} from './pubsub.js';

// The `subscribeX()` functions are bound to a fixed subscriber, usually
// `Meteor` or a testing mock.  If the need arose to support Blaze template
// `this.subscribe(...)`, we would add `subscribeXSubscriber()` functions.
function defSubscribes({ namespace, subscriber }) {
  return {
    subscribeReadyJwts() {
      return subscriber.subscribe(makePubName(namespace, PubNameReadyJwts));
    },
    subscribeUserTokens() {
      return subscriber.subscribe(makePubName(namespace, PubNameUserTokens));
    },
  };
}

export {
  defSubscribes,
};
