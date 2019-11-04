import forge from 'node-forge';
import fs from 'fs';
import jwt from 'jsonwebtoken';
import { Random } from 'meteor/random';
import { check, Match } from 'meteor/check';
import { nogthrow, ERR_FSO } from './errors.js';
import { ActionCode } from './action-code.js';

// `nowBackdated()` returns a Unix timestamp that is suitable for JWT `iat`.
// It returns the time a few seconds ago to allow for a bit of clock drift if
// using the token right away.
function nowBackdated() {
  return Math.floor(Date.now() / 1000) - 5;
}

// Restrict subuser to protect agains unreasonable tokens:
//
//  - minimum and maximum length;
//  - limited character set.
//
const matchSubuserName = Match.Where((x) => {
  check(x, String);
  return (x.length > 2) && (x.length < 60) && !!x.match(/^[a-z0-9.+-]+$/);
});

const matchExpiresIn = Match.Where((x) => {
  check(x, Number);
  if (Math.floor(x) !== x) {
    throw new Match.Error('not an integer');
  }
  if (x < 1) {
    throw new Match.Error('not positive');
  }
  if (x > 63 * 24 * 60 * 60) {
    throw new Match.Error('duration longer than 2m');
  }
  return true;
});

const matchOneKnownAudience = Match.Where((x) => {
  check(x, String);
  return x === 'nogapp' || x === 'fso';
});

// Restrict SAN to loose DNS label of limited length to protect against
// unreasonable tokens.
const matchOneSan = Match.Where((x) => {
  check(x, String);
  return (x.length < 60) && !!x.match(/^DNS:[a-zA-Z0-9.-]+$/);
});

const matchSan = [matchOneSan];

// Restrict audience to lowercase alphanum of limited length to protect against
// unreasonable tokens.
const matchOneAudience = Match.Where((x) => {
  check(x, String);
  return (x.length < 30) && !!x.match(/^[a-z0-9]+$/);
});

const matchScope = Match.Where((x) => {
  check(x, {
    action: Match.Optional(String),
    actions: Match.Optional([String]),
    path: Match.Optional(String),
    paths: Match.Optional([String]),
    name: Match.Optional(String),
    names: Match.Optional([String]),
  });
  if (!(x.action || x.actions)) {
    throw new Match.Error('require at least one of `action` or `actions`.');
  }
  if (!(x.path || x.paths || x.name || x.names)) {
    throw new Match.Error(
      'require at least one of `path`, `paths`, `name`, or `names`.',
    );
  }
  return true;
});

function compileAction(action) {
  const c = ActionCode[action];
  if (!c) {
    nogthrow(ERR_FSO, { reason: `Invalid JWT scope action \`${action}\`.` });
  }
  return c;
}

// `compileScope()` converts the long representation of a scope into the JWT
// representation.
//
// The fields of the long representation:
//
//  - `action`, `actions`: `ActionCode` that has a JWT presentation.  Use
//    `action` for a single value and `actions` for a list.  At least one of
//    the two must be present.  If both a present, they are concatenated.
//  - `path`, `paths`: Path as used in access checks.
//  - `name`, `names`: Name as used in access checks.
//
// At least one of `path`, `paths`, `name`, or `names` must be present.
//
// The fields of the JWT representation:
//
//  - `aa`: List of access actions.
//  - `p`: List of paths.
//  - `n`: List of names.
//
// The access check tests for each field individually whether it is in the
// scope.  The scope, therefore, permits operations that are in the product
// sets `actions x paths` or `actions x names`, assuming the access check uses
// either a path or a name but not both.
function compileScope(scope) {
  const aa = [];
  if (scope.action) {
    aa.push(compileAction(scope.action));
  }
  if (scope.actions) {
    for (const action of scope.actions) {
      aa.push(compileAction(action));
    }
  }

  const p = [];
  if (scope.path) {
    p.push(scope.path);
  }
  if (scope.paths) {
    for (const path of scope.paths) {
      p.push(path);
    }
  }

  const n = [];
  if (scope.name) {
    n.push(scope.name);
  }
  if (scope.names) {
    for (const name of scope.names) {
      n.push(name);
    }
  }

  const sc = { aa };
  if (p.length) {
    sc.p = p;
  }
  if (n.length) {
    sc.n = n;
  }
  return sc;
}

function compileScopes(opts) {
  const scopes = [];
  if (opts.scope) {
    scopes.push(compileScope(opts.scope));
  }
  if (opts.scopes) {
    for (const sc of opts.scopes) {
      scopes.push(compileScope(sc));
    }
  }
  return scopes;
}

// Use lookahead `(?=`, so that each split part includes the BEGIN line.
const beginRgx = /(?=-----BEGIN )/;

function readCert(path) {
  const pem = fs.readFileSync(path).toString();
  const parts = pem.split(beginRgx);
  if (parts.length !== 2) {
    nogthrow(ERR_FSO, { reason: 'Invalid combined cert-key PEM.' });
  }
  const [certPem, keyPem] = parts;
  // Parse key to validate PEM.
  forge.pki.privateKeyFromPem(keyPem);
  // Convert cert to DER base64, not base64url, see
  // <https://tools.ietf.org/html/rfc7515#section-4.1.6>.
  const cert = forge.pki.certificateFromPem(certPem);
  const asn = forge.pki.certificateToAsn1(cert);
  const der = forge.asn1.toDer(asn).getBytes();
  const x5c = forge.util.encode64(der);
  return { keyPem, x5c };
}

// User token claims:
//
//  - sub: Meteor username.
//  - xcrd: LDAP username and groups.
//  - sc: List of scopes.  See `compileScope()` for details.
//
// Sys token claims:
//
//  - sub: `sys:<username>+<subuser>`, where `<username>` is a Meteor username
//    or a special name.  Special names start with `nog`.  See below for a
//    list.  Special names need not be present in the `users` collection.
//  - san: like X.509 Subject Alternative Name, but only `DNS:` values.
//  - sc: List of scopes.  See `compileScope()` for details.
//
// Special names:
//
//   -  `nogapp+nogfso`: sys call creds for module `NogFso`.  See
//      `../../apps/nog-app/meteor/server/nog-fso.js`.
//
// Tokens that are valid for more than `ephemeralLimitS` are stored on the
// corresponding doc in the `users` collection.  Issuing such tokens is denied
// for special names that are not present in `users`.
function createFsoTokenProvider(settings) {
  check(settings, {
    issuer: String,
    cert: String,
    users: Match.Any,
    domains: Match.Optional([{ service: String, jwtXcrd: String }]),
  });
  const { issuer, users, domains = [] } = settings;
  const { keyPem, x5c } = readCert(settings.cert);

  const jwtDomainsByService = new Map();
  domains.forEach(({ service, jwtXcrd }) => {
    jwtDomainsByService.set(service, jwtXcrd);
  });

  const ephemeralLimitS = 60 * 60;
  return {
    sign(payload, { userId, expiresIn }) {
      // JWT ID `jti` <https://tools.ietf.org/html/rfc7519#section-4.1.7>
      const jti = Random.id();

      // Backdate issued at `iat` a bit for compatibility with `jwt-go`, which
      // verifies that `iat` is before now, see
      // <https://github.com/dgrijalva/jwt-go/blob/master/map_claims.go#L70>.
      const iat = nowBackdated();
      const exp = iat + expiresIn;

      // Record longterm JWTs on user doc.  Bearer auth checks the user doc, so
      // that longterm JWTs can be revoked.
      if (expiresIn > ephemeralLimitS) {
        if (!userId) {
          nogthrow(ERR_FSO, {
            reason: 'Refusing to issue longterm JWT without user id.',
          });
        }

        // GC JWTs that expired more than 10d ago.
        const gcBefore = new Date();
        gcBefore.setDate(gcBefore.getDate() - 10);
        users.update(userId, {
          $pull: { 'services.nogfsoiam.jwts': { exp: { $lt: gcBefore } } },
        });

        const jwtInfo = {
          jti,
          iat: new Date(iat * 1000),
          exp: new Date(exp * 1000),
        };
        const n = users.update(userId, {
          $push: { 'services.nogfsoiam.jwts': jwtInfo },
        });
        if (n !== 1) {
          nogthrow(ERR_FSO, {
            reason: 'Failed to record longterm JWT on user doc.',
          });
        }
      }

      const token = jwt.sign(
        {
          ...payload, jti, iat, exp,
        },
        keyPem,
        {
          algorithm: 'RS256',
          issuer,
          header: { x5c },
        },
      );
      return token;
    },

    fsoToken(euid, opts) {
      check(euid, Match.ObjectIncluding({
        _id: Match.Optional(String),
        username: String,
      }));
      check(opts, {
        expiresIn: matchExpiresIn,
        scope: Match.Optional(matchScope),
        scopes: Match.Optional([matchScope]),
      });
      const { expiresIn } = opts;
      const payload = {
        sub: `${euid.username}`,
        aud: ['fso'],
      };

      // `xcrd` contains the LDAP username and groupnames by LDAP domain.
      const xcrd = [];
      Object.entries(euid.services).forEach(([service, v]) => {
        const jwtDomain = jwtDomainsByService.get(service);
        if (!jwtDomain) {
          return;
        }
        const { ldapgroups, username } = v;
        if (!username || !ldapgroups) {
          return;
        }
        xcrd.push({ d: jwtDomain, u: username, g: ldapgroups });
      });
      if (xcrd.length > 0) {
        payload.xcrd = xcrd;
      }

      const scopes = compileScopes(opts);
      if (scopes.length) {
        payload.sc = scopes;
      }
      return this.sign(payload, {
        userId: euid._id,
        expiresIn,
      });
    },

    fsoSysToken(euid, opts) {
      check(euid, Match.ObjectIncluding({
        _id: Match.Optional(String),
        username: String,
      }));
      check(opts, {
        expiresIn: matchExpiresIn,
        subuser: String,
        aud: Match.Optional([matchOneAudience]),
        san: Match.Optional(matchSan),
        scope: Match.Optional(matchScope),
        scopes: Match.Optional([matchScope]),
      });
      const {
        expiresIn, subuser, san,
        aud = ['fso'],
      } = opts;

      const payload = {
        sub: `sys:${euid.username}+${subuser}`,
        aud,
      };

      if (san) {
        payload.san = san;
      }

      const scopes = compileScopes(opts);
      if (scopes.length) {
        payload.sc = scopes;
      }

      return this.sign(payload, {
        userId: euid._id,
        expiresIn,
      });
    },

    // `...Func()` variants return bound functions that can be used without
    // `this`.  See `rpcAuthorization()`.
    fsoTokenFunc() {
      return this.fsoToken.bind(this);
    },
    fsoSysTokenFunc() {
      return this.fsoSysToken.bind(this);
    },
  };
}

export {
  createFsoTokenProvider,
  matchExpiresIn,
  matchOneKnownAudience,
  matchSan,
  matchScope,
  matchSubuserName,
};
