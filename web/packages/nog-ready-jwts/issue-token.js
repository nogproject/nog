import { check, Match } from 'meteor/check';

import {
  ERR_UNKNOWN_READY_JWT,
  nogthrow,
} from './errors.js';
import {
  AA_FSO_ISSUE_READY_JWT,
  AA_JWT_DELETE_TOKEN,
} from './actions.js';
import {
  matchSysAbspath,
} from './match.js';

function createTokenIssuer({
  checkAccess,
  findOneReadyJwtByPath,
  tokenProvider,
}) {
  const issuer = {
    issueToken(euid, opts) {
      check(opts, {
        path: matchSysAbspath,
        name: Match.Maybe(String),
      });
      const { path } = opts;

      checkAccess(euid, AA_FSO_ISSUE_READY_JWT, { path });

      // Check that an authorized `euid` is already a user object, because we
      // want to avoid a user lookup below, although we could add a lookup to
      // support UIDs.
      check(euid, Match.ObjectIncluding({
        _id: Match.Optional(String),
        username: String,
      }));
      const user = euid;

      return issuer.issueTokenSudo(user, opts);
    },

    issueTokenSudo(user, opts) {
      const { path, name } = opts;
      const spec = findOneReadyJwtByPath(path);
      if (!spec) {
        nogthrow(ERR_UNKNOWN_READY_JWT);
      }

      const tokenOpts = {
        expiresIn: spec.expiresIn(),
        subuser: spec.subuser(),
        aud: ['nogapp'],
        scopes: spec.scopes(),
      };

      if (name) {
        tokenOpts.name = name;
      }

      const {
        token, jti, expirationTime,
      } = tokenProvider.fsoSysTokenDetailed(user, tokenOpts);

      return {
        token, jti, expirationTime,
      };
    },

    // The Meteor method `deleteUserToken()` only depends on
    // `tokenProvider.deleteUserToken()`, which is a function of the server
    // package `nog-jwts-2` and does not depend on JWT configurations.  Thus,
    // the method does not need to be implemented in `nog-ready-jwts`.  We
    // implement it here anyway, because it is related to manipulating tokens,
    // and `nog-ready-jwts` is currently the only way to create user-visible
    // tokens.  We can move the method later if we need it independently of
    // `nog-ready-jwts`.
    deleteUserToken(euid, opts) {
      check(opts, {
        jti: String,
        userId: String,
      });

      checkAccess(euid, AA_JWT_DELETE_TOKEN, opts);

      const { userId, jti } = opts;
      tokenProvider.deleteUserToken(userId, { jti });
    },
  };

  return issuer;
}

export {
  createTokenIssuer,
};
