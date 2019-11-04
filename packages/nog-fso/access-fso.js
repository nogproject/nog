import pathToRegexp from 'path-to-regexp';
import { Match, check } from 'meteor/check';
import {
  ERR_LOGIC,
  ERR_PARAM_INVALID,
  nogthrow,
} from './errors.js';
import { AllFsoActions } from './all-fso-actions.js'

// `Rule` enumerates the available permission rules.
const Rule = {
  // `AllowInsecureEverything` enables full, unchecked fso access for
  // individual users.  It should only be used during the preview phase.
  AllowInsecureEverything: 'AllowInsecureEverything',

  // `AllowPrincipalsNames` allows a list of `principals` to perform a
  // list of `actions` on a set of `names`.  The principal must be of type
  // `username:` or `ldapgroup:`.  We add more types as needed.
  AllowPrincipalsNames: 'AllowPrincipalsNames',

  // `AllowPrincipalsPathPrefix` allows a list of `principals` to perform a
  // list of `actions` on paths that start with `pathPrefix`, which must end
  // with a slash.  The principal must be of type `username:` or `ldapgroup:`.
  // We add more types as needed.  `AllowPrincipalsPathEqualOrPrefix` is often
  // more useful than `AllowPrincipalsPathPrefix`.
  AllowPrincipalsPathPrefix: 'AllowPrincipalsPathPrefix',

  // `AllowPrincipalsPathEqualOrPrefix` allows a list of `principals` to
  // perform a list of `actions` on paths that equal `path` or start with
  // `path` followed by a slash.  The principal must be of type `username:` or
  // `ldapgroup:`.  We add more types as needed.
  AllowPrincipalsPathEqualOrPrefix: 'AllowPrincipalsPathEqualOrPrefix',

  // `AllowPrincipalsPathPattern` allows a list of `principal` to perform a
  // list of `actions` on paths that match `pathPattern`.  The parameters of
  // `pathPattern` are ignored; it is sufficient that the path matches.
  AllowPrincipalsPathPattern: 'AllowPrincipalsPathPattern',

  // `AllowLdapGroupFromPath` uses a `pathPattern` that must have a named
  // parameter `:group` to extract an LDAP group name from the path and grant
  // access to an `ldapgroup:<group>` principal to perform a list of `actions`.
  // The rule has an optional parameter `ldapPrefix`, which will be prepended
  // to the matched parameter like `<ldapPrefix><group>` before comparing with
  // the LDAP group.
  AllowLdapGroupFromPath: 'AllowLdapGroupFromPath',

  // `AllowLdapGroup2FromPath` uses a `pathPattern` that must have two named
  // parameters `:group1` and `:group2` to extract LDAP group names from the
  // path and grant access to users that are in both LDAP groups to perform a
  // list of `actions`.  The effect implementation checks that a
  // `ldapgroup:<group1>` principal has `<group2>` in `opts.user.ldapgroups`.
  // The rule has to optional parameters `ldapPrefix1` and `ldapPrefix2`.  If
  // present, the matched pattern parameters will be prefixed like
  // `<ldapPrefixN><groupN>` before comparing with the LDAP groups.
  AllowLdapGroups2FromPath: 'AllowLdapGroups2FromPath',
};

const matchAction = Match.Where((x) => {
  check(x, String);
  return x.startsWith('fso/') || x.startsWith('bc/') || x.startsWith('sys/');
});

const matchPrincipal = Match.Where((x) => {
  check(x, String);
  return !!x.match(/^(username|ldapgroup):[^:]+$/);
});

// `matchLDHQualified()` loosely matches qualified DNS names that use the LDH
// (letter, digit, hyphen) rule without trailing dot.  A stricter check would
// reject leading and trailing dashes.
const matchLDHQualified = Match.Where((x) => {
  check(x, String);
  return !!x.match(/^[a-z0-9-]+([.][a-z0-9-]+)*$/);
});

const matchAbspath = Match.Where((x) => {
  check(x, String);
  return x.startsWith('/');
});

const matchPathPrefix = Match.Where((x) => {
  check(x, String);
  return x.startsWith('/') && x.endsWith('/');
});

const matchPathPattern = Match.Where((x) => {
  check(x, String);
  return x.startsWith('/');
});

const matchAllowInsecureEverything = Match.Where((x) => {
  check(x, {
    rule: String,
    usernames: [String],
  });
  return x.rule === Rule.AllowInsecureEverything;
});

const matchAllowPrincipalsNames = Match.Where((x) => {
  check(x, {
    rule: String,
    names: [matchLDHQualified],
    principals: [matchPrincipal],
    actions: [matchAction],
  });
  return x.rule === Rule.AllowPrincipalsNames;
});

const matchAllowPrincipalsPathPrefix = Match.Where((x) => {
  check(x, {
    rule: String,
    pathPrefix: matchPathPrefix,
    principals: [matchPrincipal],
    actions: [matchAction],
  });
  return x.rule === Rule.AllowPrincipalsPathPrefix;
});

const matchAllowPrincipalsPathEqualOrPrefix = Match.Where((x) => {
  check(x, {
    rule: String,
    path: matchAbspath,
    principals: [matchPrincipal],
    actions: [matchAction],
  });
  return x.rule === Rule.AllowPrincipalsPathEqualOrPrefix;
});

const matchAllowPrincipalsPathPattern = Match.Where((x) => {
  check(x, {
    rule: String,
    pathPattern: matchPathPattern,
    principals: [matchPrincipal],
    actions: [matchAction],
  });
  return x.rule === Rule.AllowPrincipalsPathPattern;
});

const matchAllowLdapGroupFromPath = Match.Where((x) => {
  check(x, {
    rule: String,
    pathPattern: matchPathPattern,
    actions: [matchAction],
    ldapPrefix: Match.Optional(String),
  });
  return x.rule === Rule.AllowLdapGroupFromPath;
});

const matchAllowLdapGroups2FromPath = Match.Where((x) => {
  check(x, {
    rule: String,
    pathPattern: matchPathPattern,
    actions: [matchAction],
    ldapPrefix1: Match.Optional(String),
    ldapPrefix2: Match.Optional(String),
  });
  return x.rule === Rule.AllowLdapGroups2FromPath;
});

const matchPermission = Match.Where((x) => {
  check(x, Match.OneOf(
    matchAllowInsecureEverything,
    matchAllowPrincipalsNames,
    matchAllowPrincipalsPathPrefix,
    matchAllowPrincipalsPathEqualOrPrefix,
    matchAllowPrincipalsPathPattern,
    matchAllowLdapGroupFromPath,
    matchAllowLdapGroups2FromPath,
  ));
  return true;
});

const matchFsoPermissions = [matchPermission];

const Effect = {
  allow: 'allow',
  ignore: 'ignore',
};

// `compilePathPattern0(pattern)` takes an Express path pattern and returns a
// function `match(path)` that can be called to test whether a path matches.
function compilePathPattern0(pat) {
  const keys = [];
  const rgx = pathToRegexp(pat, keys);
  return path => rgx.exec(path);
}

// `compilePathPattern1(pattern, paramName)` takes an Express path `pattern`
// that must contain the parameter `paramName` and returns a function
// `match(path)` that can be called later to extract the parameter from `path`.
// `match()` returns `null` if the match fails.
function compilePathPattern1(pat, paramName) {
  const keys = [];
  const rgx = pathToRegexp(pat, keys);

  let paramIdx = 0;
  keys.forEach((k, i) => {
    if (k.name === paramName) {
      paramIdx = i + 1;
    }
  });
  if (!paramIdx) {
    nogthrow(ERR_PARAM_INVALID, {
      reason: `Missing param \`${paramName}\` in \`${pat}\`.`,
    });
  }

  return function matchPath(path) {
    const m = rgx.exec(path);
    if (!m) {
      return null;
    }

    return m[paramIdx];
  };
}

// `compilePathPattern2(pattern, param1, param2)` takes an Express path
// `pattern` that must contain the two names parameters `param1` and `param2`
// and returns a function `match(path)` that can be called later to extract the
// parameters from `path` as an array with two elements.  `match()` returns
// `null` if the match fails.
function compilePathPattern2(pat, param1, param2) {
  const keys = [];
  const rgx = pathToRegexp(pat, keys);

  let param1Idx = 0;
  keys.forEach((k, i) => {
    if (k.name === param1) {
      param1Idx = i + 1;
    }
  });
  if (!param1Idx) {
    nogthrow(ERR_PARAM_INVALID, {
      reason: `Missing param \`${param1}\` in \`${pat}\`.`,
    });
  }

  let param2Idx = 0;
  keys.forEach((k, i) => {
    if (k.name === param2) {
      param2Idx = i + 1;
    }
  });
  if (!param2Idx) {
    nogthrow(ERR_PARAM_INVALID, {
      reason: `Missing param \`${param2}\` in \`${pat}\`.`,
    });
  }

  return function matchPath(path) {
    const m = rgx.exec(path);
    if (!m) {
      return null;
    }

    return [m[param1Idx], m[param2Idx]];
  };
}

function ensureTrailingSlash(s) {
  if (s.endsWith('/')) {
    return s;
  }
  return `${s}/`;
}

function compileFsoPermissions(perms) {
  check(perms, matchFsoPermissions);
  const statements = [];
  for (const p of perms) {
    switch (p.rule) {
      case Rule.AllowInsecureEverything:
        for (const u of p.usernames) {
          for (const action of AllFsoActions) {
            statements.push({
              principal: `username:${u}`,
              action,
              effect: 'allow',
            });
          }
        }
        break;

      case Rule.AllowPrincipalsNames: {
        const principals = new Set(p.principals);
        const names = new Set(p.names);
        for (const action of p.actions) {
          statements.push({
            principal: /^(username|ldapgroup):[^:]+$/,
            action,
            effect: (opts) => {
              if (!principals.has(opts.principal)) {
                return Effect.ignore;
              }
              if (names.has(opts.name)) {
                return Effect.allow;
              }
              return Effect.ignore;
            },
          });
        }
        break;
      }

      case Rule.AllowPrincipalsPathPrefix: {
        const principals = new Set(p.principals);
        for (const action of p.actions) {
          statements.push({
            principal: /^(username|ldapgroup):[^:]+$/,
            action,
            effect: (opts) => {
              if (!principals.has(opts.principal)) {
                return Effect.ignore;
              }
              if (opts.path.startsWith(p.pathPrefix)) {
                return Effect.allow;
              }
              return Effect.ignore;
            },
          });
        }
        break;
      }

      case Rule.AllowPrincipalsPathEqualOrPrefix: {
        const principals = new Set(p.principals);
        const pathPrefixSlash = ensureTrailingSlash(p.path);
        for (const action of p.actions) {
          statements.push({
            principal: /^(username|ldapgroup):[^:]+$/,
            action,
            effect: (opts) => {
              if (!principals.has(opts.principal)) {
                return Effect.ignore;
              }
              if (opts.path === p.path) {
                return Effect.allow;
              }
              if (opts.path.startsWith(pathPrefixSlash)) {
                return Effect.allow;
              }
              return Effect.ignore;
            },
          });
        }
        break;
      }

      case Rule.AllowPrincipalsPathPattern: {
        const matchPath = compilePathPattern0(p.pathPattern);
        const principals = new Set(p.principals);
        for (const action of p.actions) {
          statements.push({
            principal: /^(username|ldapgroup):[^:]+$/,
            action,
            effect: (opts) => {
              const m = matchPath(opts.path);
              if (m && principals.has(opts.principal)) {
                return Effect.allow;
              }
              return Effect.ignore;
            },
          });
        }
        break;
      }

      case Rule.AllowLdapGroupFromPath: {
        const matchPath = compilePathPattern1(p.pathPattern, 'group');
        const prefix = p.ldapPrefix || '';

        const effect = (opts) => {
          const m = matchPath(opts.path);
          if (!m) {
            return Effect.ignore;
          }
          const pathGroup = `${prefix}${m}`;
          const principalGroup = opts.principal.split(':')[1];
          if (pathGroup === principalGroup) {
            return Effect.allow;
          }
          return Effect.ignore;
        };

        for (const action of p.actions) {
          statements.push({
            principal: /^ldapgroup:[^:]+$/,
            action,
            effect,
          });
        }
        break;
      }

      case Rule.AllowLdapGroups2FromPath: {
        const matchPath = compilePathPattern2(
          p.pathPattern, 'group1', 'group2',
        );
        const prefix1 = p.ldapPrefix1 || '';
        const prefix2 = p.ldapPrefix2 || '';

        const effect = (opts) => {
          const { user } = opts;
          if (!user) {
            return Effect.ignore;
          }

          const allLdapgroups = [];
          Object.values(user.services).forEach(({ ldapgroups }) => {
            if (ldapgroups) {
              allLdapgroups.push(...ldapgroups);
            }
          });
          if (allLdapgroups.length === 0) {
            return Effect.ignore;
          }

          const m = matchPath(opts.path);
          if (!m) {
            return Effect.ignore;
          }

          // 1-based names with 0-based match array!
          const pathGroup1 = `${prefix1}${m[0]}`;
          const pathGroup2 = `${prefix2}${m[1]}`;
          const principalGroup = opts.principal.split(':')[1];
          const userIsInGroup1 = (pathGroup1 === principalGroup);
          const userIsInGroup2 = allLdapgroups.includes(pathGroup2);
          if (userIsInGroup1 && userIsInGroup2) {
            return Effect.allow;
          }
          return Effect.ignore;
        };

        for (const action of p.actions) {
          statements.push({
            principal: /^ldapgroup:[^:]+$/,
            action,
            effect,
          });
        }
        break;
      }

      default:
        // The `check()` at function entry rejects unknown rules.
        nogthrow(ERR_LOGIC, {
          reason: `Unexpected permission rule \`${p.rule}\`.`,
        });
    }
  }

  return statements;
}

export {
  compileFsoPermissions,
  matchFsoPermissions,
};
