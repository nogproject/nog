import { _ } from 'meteor/underscore';
import { check, Match } from 'meteor/check';
import {
  matchExpiresIn,
  matchOneKnownAudience,
  matchSan,
  matchSubuserName,
} from './fso-jwt.js';
import {
  ERR_ACCESS_DENY,
  ERR_LOGIC,
  ERR_PARAM_MALFORMED,
  nogthrow,
} from './errors.js';
import {
  KeyFsoId,
  KeyName,
} from './collections.js';

const AA_FSO_ISSUE_SYS_TOKEN = 'fso/issue-sys-token';
const AA_FSO_ISSUE_USER_TOKEN = 'fso/issue-user-token';

// eslint-disable-next-line no-unused-vars
const matchSimpleScope = Match.Where((x) => {
  check(x, {
    action: String,
    path: Match.Optional(String),
    name: Match.Optional(String),
  });
  if (!(x.path || x.name)) {
    throw new Match.Error(
      'require at least one of `path` or `name`.',
    );
  }
  return true;
});

const matchStringNonEmpty = Match.Where((x) => {
  check(x, String);
  if (x.length === 0) {
    throw new Match.Error('require non-empty string');
  }
  return true;
});

// An `XorScope` is a restricted scope representation:
//
//  - either `action` or `actions` but not both;
//  - only one of `name`, `names`, `path`, `paths`;
//  - all strings must be non-empty.
//
// The restrictions should avoid confusion.
const matchXorScope = Match.Where((x) => {
  check(x, {
    action: Match.Optional(matchStringNonEmpty),
    actions: Match.Optional([matchStringNonEmpty]),
    name: Match.Optional(matchStringNonEmpty),
    names: Match.Optional([matchStringNonEmpty]),
    path: Match.Optional(matchStringNonEmpty),
    paths: Match.Optional([matchStringNonEmpty]),
  });
  if (_.size(_.pick(x, 'action', 'actions')) !== 1) {
    throw new Match.Error('require either `action` or `actions`');
  }
  if (_.size(_.pick(x, 'name', 'names', 'path', 'paths')) !== 1) {
    throw new Match.Error(
      'require exactly one of `name`, `names`, `path`, or `paths`',
    );
  }
  return true;
});

function normalizeXorScope(scope) {
  const actions = scope.actions || [scope.action];
  if (_.has(scope, 'name')) {
    return { actions, names: [scope.name] };
  }
  if (_.has(scope, 'names')) {
    return { actions, names: scope.names };
  }
  if (_.has(scope, 'path')) {
    return { actions, paths: [scope.path] };
  }
  if (_.has(scope, 'paths')) {
    return { actions, paths: scope.paths };
  }
  nogthrow(ERR_LOGIC, { reason: 'Invalid XorScope.' });
  return null;
}

const rgxUuid = (
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
);

const matchUuid = Match.Where((x) => {
  check(x, String);
  return !!x.match(rgxUuid);
});

const matchPathNameRepoIdScope = Match.Where((x) => {
  check(x, {
    action: String,
    path: Match.Optional(String),
    name: Match.Optional(String),
    repoId: Match.Optional(matchUuid),
  });
  if (!(x.path || x.name || x.repoId)) {
    throw new Match.Error(
      'require at least one of `path`, `name`, or `repoId`.',
    );
  }
  if (x.path && x.repoId) {
    throw new Match.Error(
      '`path` and `repoId` cannot be used together.',
    );
  }
  return true;
});

// XXX Maybe refactor to avoid duplication with `nog-access/scopes-v2.js`.

function globMatches(g, fixed) {
  if (g === '*') {
    return true;
  }

  if (g.endsWith('*')) {
    const prefix = g.substr(0, g.length - 1);
    return fixed.startsWith(prefix);
  }

  return fixed === g;
}

function globIncludes(globs, fixed) {
  if (!globs) {
    return false;
  }
  for (const g of globs) {
    if (globMatches(g, fixed)) {
      return true;
    }
  }
  return false;
}

function isSubScope(eff, req) {
  const { action, path, name } = req;

  if (!globIncludes(eff.actions, action)) {
    return false;
  }

  if (path && !globIncludes(eff.paths, path)) {
    return false;
  }

  if (name && !globIncludes(eff.names, name)) {
    return false;
  }

  return true;
}

function isInEffectiveScopes(eff, req) {
  for (const e of eff) {
    if (isSubScope(e, req)) {
      return true;
    }
  }
  return false;
}

function isWildcardScope(scope) {
  const { action, path, name } = scope;
  return action.endsWith('*') ||
    (path && path.endsWith('*')) ||
    (name && name.endsWith('*'));
}

function parseUuid(id) {
  const hex = id.replace(/-/g, '');
  return Buffer.from(hex, 'hex');
}

function createAuthApiActions({
  checkAccess, testAccess, repos, rpcTokenProvider,
}) {
  function resolveRepoId(scope) {
    if (!scope.repoId) {
      return scope;
    }

    const sel = { [KeyFsoId]: parseUuid(scope.repoId) };
    const fields = { [KeyName]: true };
    const repo = repos.findOne(sel, { fields });
    if (!repo) {
      nogthrow(ERR_ACCESS_DENY, { reason: 'Unknown repo.' });
    }

    const ret = {
      action: scope.action,
      path: repo.path(),
    };
    if (scope.name) {
      ret.name = scope.name;
    }
    return ret;
  }

  function postAuth(req) {
    const euid = req.auth.user;
    checkAccess(euid, AA_FSO_ISSUE_USER_TOKEN, { path: '/' });

    const opts = req.body;
    check(opts, {
      expiresIn: matchExpiresIn,
      scopes: [matchPathNameRepoIdScope],
    });
    const { expiresIn } = opts;
    const scopes = opts.scopes.map(resolveRepoId);

    const { scopesV2 } = euid;
    if (!scopesV2) {
      nogthrow(ERR_ACCESS_DENY, { reason: 'Missing scopes v2.' });
    }

    for (const scope of scopes) {
      if (isWildcardScope(scope)) {
        nogthrow(ERR_ACCESS_DENY, {
          reason: 'Wildcard scopes not allowed.',
        });
      }
      // `isInEffectiveScopes()` is a redundant check.  `testAccess()` also
      // checks `euid.scopesV2`.
      if (!isInEffectiveScopes(scopesV2, scope)) {
        nogthrow(ERR_ACCESS_DENY, {
          reason: 'Requested scopes are not included in the effective scopes.',
        });
      }
      if (!testAccess(euid, scope.action, _.pick(scope, 'path', 'name'))) {
        nogthrow(ERR_ACCESS_DENY, {
          reason: 'The effective user cannot use a requested scope.',
        });
      }
    }

    return {
      token: rpcTokenProvider.fsoToken(euid, { expiresIn, scopes }),
    };
  }

  // A request to `postSysAuth()` must use a scoped JWT.  `testAccess()` is
  // used below to check that the requested scopes are included in the scoped
  // JWT.  The returned JWT can, therefore, only have the same or a narrower
  // scope.
  //
  // Wildcard scopes are allowed.
  //
  // `subuser`, `aud`, `san`, and `expiresIn` are only restricted by the
  // parameter validation, but not by the scoped JWT.
  //
  // This combination allows an admin to issue a widely scoped token via the
  // GUI and then use it to issue more narrowly scoped tokens via the API.
  function postSysAuth(req) {
    const euid = req.auth.user;
    checkAccess(euid, AA_FSO_ISSUE_SYS_TOKEN, { path: '/' });
    if (!euid.scopesV2) {
      nogthrow(ERR_ACCESS_DENY, { reason: 'Missing euid scopes v2.' });
    }

    const opts = req.body;
    check(opts, {
      subuser: matchSubuserName,
      expiresIn: matchExpiresIn,
      aud: [matchOneKnownAudience],
      san: Match.Optional(matchSan),
      scopes: [matchXorScope],
    });
    if (opts.aud.length < 1) {
      nogthrow(ERR_PARAM_MALFORMED, { reason: '`aud` must not be empty.' });
    }
    if (opts.scopes.length < 1) {
      nogthrow(ERR_PARAM_MALFORMED, { reason: '`scopes` must not be empty.' });
    }

    const {
      subuser, expiresIn, aud, san,
    } = opts;
    const scopes = opts.scopes.map(normalizeXorScope);
    for (const scope of scopes) {
      for (const action of scope.actions) {
        for (const name of (scope.names || [])) {
          if (!testAccess(euid, action, { name })) {
            nogthrow(ERR_ACCESS_DENY, {
              reason: (
                'The effective user cannot use ' +
                `{ action: ${action}, name: ${name} }.`
              ),
            });
          }
        }
        for (const path of (scope.paths || [])) {
          if (!testAccess(euid, action, { path })) {
            nogthrow(ERR_ACCESS_DENY, {
              reason: (
                'The effective user cannot use ' +
                `{ action: ${action}, path: ${path} }.`
              ),
            });
          }
        }
      }
    }

    const tokOpts = {
      subuser, expiresIn, aud, scopes,
    };
    if (san) {
      tokOpts.san = san;
    }
    return {
      token: rpcTokenProvider.fsoSysToken(euid, tokOpts),
    };
  }

  return [
    {
      method: 'POST',
      path: '/auth',
      action: postAuth,
    },
    {
      method: 'POST',
      path: '/sysauth',
      action: postSysAuth,
    },
  ];
}

export {
  createAuthApiActions,
};
