/* eslint-disable react/no-this-in-sfc */

import {
  CollNameReadyJwts,
  KeyId,
  KeyTitle,
  KeyDescription,
  KeyPath,
  makeCollName,
} from './collections.js';
import {
  makePubName,
  PubNameReadyJwts,
  PubNameUserTokens,
} from './pubsub.js';
import {
  AA_FSO_ISSUE_READY_JWT,
} from './actions.js';

// `publishReadyJwts()` publishes all ready JWTs to which the current user has
// access.
//
// The implementation assumes that there are only a few ready JWTs, so that a
// `testAccess()` call for each of them is not too expensive.
function publishReadyJwtsFunc({
  namespace, findOneUserById, testAccess,
  readyJwts,
}) {
  const readyJwtsName = makeCollName(namespace, CollNameReadyJwts);

  return function publishReadyJwts() {
    // Find user once to avoid find during each `testAccess()`.
    const euid = this.userId ? findOneUserById(this.userId) : null;
    if (!euid) {
      this.ready();
      return null;
    }

    readyJwts.find({}, {
      transform: null,
      fields: {
        [KeyId]: 1,
        [KeyTitle]: 1,
        [KeyDescription]: 1,
        [KeyPath]: 1,
      },
    }).forEach((d) => {
      if (testAccess(euid, AA_FSO_ISSUE_READY_JWT, { path: d[KeyPath] })) {
        this.added(readyJwtsName, d[KeyId], d);
      }
    });

    this.ready();
    return null;
  };
}

function publishUserTokensFunc({ users }) {
  return function publishUserTokens() {
    if (!this.userId) {
      this.ready();
      return null;
    }

    return users.find(
      { _id: this.userId }, { fields: { 'services.nogfsoiam.jwts': 1 } },
    );
  };
}

function publish({
  namespace, publisher, findOneUserById, testAccess,
  readyJwts, users,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }

  defPub(PubNameReadyJwts, publishReadyJwtsFunc({
    namespace, findOneUserById, testAccess,
    readyJwts,
  }));

  defPub(PubNameUserTokens, publishUserTokensFunc({ users }));
}

export {
  publish,
};
