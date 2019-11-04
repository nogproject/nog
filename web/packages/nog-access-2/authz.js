import { check } from 'meteor/check';
import { matchStatement } from './statements.js';
import {
  isFunction, isObject, isRegExp, isString,
} from './underscore.js';
import {
  nogthrow,
  ERR_ACCESS_DENY,
  ERR_ACCESS_DEFAULT_DENY,
} from './errors.js';
import { checkUserPluginScopesV2 } from './scopes-v2.js';

function matchPrincipal(p, s) {
  if (isString(s.principal)) {
    return (p === s.principal);
  }
  if (isRegExp(s.principal)) {
    return !!p.match(s.principal);
  }
  return false;
}

function checkAccessPrincipalsStatements(
  principals, statements, action, opts,
) {
  let denied = false;
  let allowed = false;
  const denyReasons = [];
  const denyStatements = [];
  for (const principal of principals) {
    for (const s of statements) {
      if (matchPrincipal(principal, s) && (s.action === action)) {
        let { effect } = s;
        let reason = null;
        if (isFunction(effect)) {
          effect = effect({ principal, ...opts });
          if (isObject(effect)) {
            ({ effect, reason } = effect);
          }
        }

        switch (effect) {
          case 'allow':
            allowed = true;
            break;
          case 'deny':
            denied = true;
            if (reason) {
              denyReasons.push(reason);
            }
            denyStatements.push({
              principal,
              action,
              statement: {
                principal: s.principal,
                action: s.action,
                effect,
                reason,
              },
              opts,
            });
            break;
          case 'ignore':
            break;
          default:
            console.error(`Invalid policy effect '${effect}'.`);
            break;
        }
      }
    }
  }

  if (denied) {
    let reason;
    if (denyReasons.length > 0) {
      reason = `Access denied: ${denyReasons.join(' ')}`;
    } else {
      reason = 'Access denied by policy.';
    }
    nogthrow(ERR_ACCESS_DENY, { reason, denyStatements });
  }

  if (allowed) {
    return;
  }

  nogthrow(ERR_ACCESS_DEFAULT_DENY);
}

const checkUserPluginsDefault = [
  // Do not use `checkUserPluginScopes`; see `./scopes-legacy.js`,
  checkUserPluginScopesV2,
];

function principalPluginRoles(principals, user) {
  const { roles } = user;
  if (roles) {
    for (const r of roles) {
      principals.push(`role:${r}`);
    }
  } else {
    principals.push('guests');
  }
}

function principalPluginUsername(principals, user) {
  principals.push(`username:${user.username}`);
}

function principalPluginUserid(principals, user) {
  principals.push(`userid:${user._id}`);
}

function principalPluginLdapgroups(principals, user) {
  for (const srv of Object.values(user.services)) {
    const { ldapgroups } = srv;
    if (ldapgroups) {
      for (const g of ldapgroups) {
        principals.push(`ldapgroup:${g}`);
      }
    }
  }
}

const principalPluginsDefault = [
  principalPluginRoles,
  principalPluginUsername,
  principalPluginUserid,
  principalPluginLdapgroups,
];

function createAuthorizer({
  users,
  checkUserPlugins,
  principalPlugins,
}) {
  return {
    statements: [],

    addStatement(s) {
      check(s, matchStatement);
      this.statements.push(s);
    },

    addStatements(ss) {
      check(ss, [matchStatement]);
      this.statements = this.statements.concat(ss);
    },

    checkAccess(euid, action, opts) {
      let user = null;
      if (isObject(euid)) {
        user = euid;
      } else if (isString(euid)) {
        user = users.findOne({ _id: euid });
      }

      const optsWithUser = { ...opts };
      const principals = [];
      if (user) {
        if (checkUserPlugins) {
          for (const plug of checkUserPlugins) {
            // Plugin may throws to deny access.
            plug(user, action, opts);
          }
        }
        optsWithUser.user = user;
        for (const plug of principalPlugins) {
          // Plugin may push principals.
          plug(principals, user);
        }
      } else {
        principals.push('anonymous');
      }

      this.checkAccessPrincipals(principals, action, optsWithUser);
    },

    checkAccessPrincipals(principals, action, optsWithUser) {
      checkAccessPrincipalsStatements(
        principals, this.statements, action, optsWithUser,
      );
    },

    testAccess(euid, action, opts) {
      try {
        this.checkAccess(euid, action, opts);
        return true;
      } catch (err) {
        return false;
      }
    },
  };
}

export {
  checkUserPluginScopesV2,
  checkUserPluginsDefault,
  principalPluginRoles,
  principalPluginUsername,
  principalPluginUserid,
  principalPluginLdapgroups,
  principalPluginsDefault,
  createAuthorizer,
};
