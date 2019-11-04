import { Meteor } from 'meteor/meteor';
import { check } from 'meteor/check';

import { createCollectionsServer } from './collections-server.js';
import { defMethodCalls } from './methods.js';
import { createTokenIssuer } from './issue-token.js';
import { publish } from './pub.js';
import { matchReadyJwt } from './match.js';

function createReadyJwtsModuleServer({
  namespace, publisher, findOneUserById,
  checkAccess, testAccess,
  tokenProvider,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(publisher.publish, Function);
  check(findOneUserById, Function);
  check(testAccess, Function);
  check(checkAccess, Function);
  check(tokenProvider.fsoSysTokenDetailed, Function);

  const {
    readyJwts,
    upsertReadyJwt,
    findOneReadyJwtByPath,
  } = createCollectionsServer({ namespace });

  publish({
    namespace, publisher, findOneUserById, testAccess,
    readyJwts,
    users: Meteor.users,
  });

  const {
    issueToken,
    issueTokenSudo,
    deleteUserToken,
  } = createTokenIssuer({
    checkAccess,
    findOneReadyJwtByPath,
    tokenProvider,
  });

  const module = {
    issueToken,
    issueTokenSudo,
    deleteUserToken,
    readyJwts,
    upsertReadyJwt,
    findOneReadyJwtByPath,
  };
  // Register Meteor methods without assigning `callX()` functions to `module`.
  // Server code should call the real functions, not via a Meteor method.
  defMethodCalls(module, { namespace });
  return module;
}

export {
  createReadyJwtsModuleServer,
  matchReadyJwt,
};
