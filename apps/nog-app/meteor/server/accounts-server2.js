import { HTTP } from 'meteor/http';
import { NogError } from 'meteor/nog-error';
import { OAuthEncryption } from 'meteor/oauth-encryption';
import { Meteor } from 'meteor/meteor';
import { Accounts } from 'meteor/accounts-base';
import { createLdapClient } from 'meteor/nog-ldap';
import { mailToAdminsUserAddedSignInService } from './email.js';
const {
  ERR_CREATE_ACCOUNT_EMAIL,
  ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED,
  ERR_CREATE_ACCOUNT_USERNAME,
  nogthrow,
} = NogError;

// `gitlabGetEmail({url, token})` returns the email address for the GitLab user
// who owns `token`.
//
// See <https://docs.gitlab.com/ce/api/users.html#user>,
// <https://docs.gitlab.com/ce/api/oauth2.html#gitlab-as-an-oauth2-provider>,
// <https://docs.gitlab.com/ce/api/oauth2.html#access-gitlab-api-with-access-token>.
function gitlabGetEmail({ url, token }) {
  const headers = { Authorization: `Bearer ${token}` };
  const userApi = `${url}/api/v4/user`;
  const rsp = HTTP.get(userApi, { headers });
  if (!rsp.data || !rsp.data.email) {
    nogthrow(ERR_CREATE_ACCOUNT_EMAIL, {
      reason: 'Failed to retrieve GitLab account email.',
    });
  }
  return rsp.data.email;
}

// Cf. `pinEncryptedFieldsToUser()` in `accounts-base/accounts_server.js`.
function repinEncryptedFieldsToUser(serviceData, { oldUserId, newUserId }) {
  const dup = { ...serviceData };
  for (const [k, v] of Object.entries(dup)) {
    if (OAuthEncryption.isSealed(v)) {
      const cleartext = OAuthEncryption.open(v, oldUserId);
      dup[k] = OAuthEncryption.seal(cleartext, newUserId);
    }
  }
  return dup;
}

function compileWellknownAccounts(settings) {
  const byService = new Map();
  ['github', 'gitimp', 'gitzib'].forEach((k) => {
    byService.set(k, new Map());
  });

  settings.forEach((s) => {
    const { username, accountType, aka } = s;
    for (const [srv, name] of Object.entries(aka)) {
      byService.get(srv).set(name, {
        username, accountType,
      });
    }
  });

  return {
    byService,
  };
}

const wellknownAccounts = (() => {
  let settings = Meteor.settings.wellknownAccounts;
  if (settings === 'dev') {
    settings = [
      {
        username: 'sprohaska', accountType: 'password',
        aka: { github: 'sprohaska' },
      },
    ];
  }
  return compileWellknownAccounts(settings);
})();

function handleWellknownAccounts(akaUser) {
  const {
    password, github, gitimp, gitzib,
  } = akaUser.services;

  if (password) {
    if (akaUser.username.startsWith('bz')) {
      nogthrow(ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED, {
        reason: 'Usernames "bz..." must sign in with Gitzib.',
      });
    }
    // Handle other password usernames as usual.
    return;
  }

  // Keep in sync with `./accounts-server.coffee`.
  let akaService;
  let akaUsername;
  let akaServiceData;
  if (github) {
    akaService = 'github';
    akaUsername = github.username;
    akaServiceData = github;
  } else if (gitimp) {
    akaService = 'gitimp';
    akaUsername = gitimp.username;
    akaServiceData = gitimp;
  } else if (gitzib) {
    akaService = 'gitzib';
    akaUsername = gitzib.username;
    akaServiceData = gitzib;
  } else {
    nogthrow(ERR_CREATE_ACCOUNT_USERNAME, {
      reason: 'Failed to determine username for account from unknown service.',
    });
  }

  const real = wellknownAccounts.byService.get(akaService).get(akaUsername);

  // If the real account is unknown, perform general sanity checks and return
  // to handling user as usual.
  if (!real) {
    if (akaUsername.startsWith('bz') && akaService !== 'gitzib') {
      nogthrow(ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED, {
        reason: 'Usernames "bz..." must sign in with Gitzib.',
      });
    }
    return;
  }

  // If the real account is known, check that it exists and if so, add the
  // login service to the existing account.
  const { username, accountType } = real;
  const user = Meteor.users.findOne({ username, accountType });
  if (!user) {
    nogthrow(ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED, {
      reason: (
        `Sign in as user "${username}" with ${accountType} first ` +
        `to activate your primary sign-in method.`
      ),
    });
  }

  const $set = {
    [`services.${akaService}`]: repinEncryptedFieldsToUser(akaServiceData, {
      oldUserId: akaUser._id,
      newUserId: user._id,
    }),
  };
  const nUp = Meteor.users.update(user._id, { $set });
  if (nUp !== 1) {
    nogthrow(ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED, {
      reason: 'Failed to add sign-in method to existing account.',
    });
  }

  mailToAdminsUserAddedSignInService({
    user,
    service: akaService,
  });

  // Success is reported as an exception to abort adding the new account.  The
  // user must repeat in order to sign in to the original account.
  nogthrow(ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED, {
    reason: (
      `${akaService} has been added as a new sign-in method ` +
      `to your existing account "${username}". ` +
      'Repeat the sign-in in order to use the new method.'
    ),
  });
}

const ldapSettings = new Map(
  Meteor.settings.ldap.map(s => [s.service, s]),
);

function updateUserFromLdap({ user }) {
  const $set = {};
  let autoRegisterUser = false;

  Object.entries(user.services).forEach(([k, srv]) => {
    const ldapCfg = ldapSettings.get(k);
    if (!ldapCfg) {
      return;
    }

    const ldap = createLdapClient(ldapCfg);
    if (!ldap.connected) {
      return;
    }

    let { username } = srv;
    // Handle legacy service data that lacks `username`: if `accountType`
    // equals the service name, use the main username, and also set it on the
    // service data.
    if (!username && k === user.accountType) {
      ({ username } = user);
      $set[`services.${k}.username`] = username;
    }
    if (!username) {
      return;
    }

    try {
      const foundUsers = ldap.searchUser(username);
      if (foundUsers.length === 0) {
        console.error(
          `[app] Missing user \`${username}\` in LDAP \`${ldap.url}\`.`,
        );
        return;
      } else if (foundUsers.length > 1) {
        console.error(
          `[app] Ambiguous LDAP lookup user \`${username}\` ` +
          `in LDAP \`${ldap.url}\`.`,
        );
        return;
      }
      const [ldapUser] = foundUsers;

      const grps = ldap.searchGroups(username);
      const primaryGrp = ldap.resolveGid(ldapUser.gidNumber);
      if (!grps.includes(primaryGrp)) {
        grps.push(primaryGrp);
      }
      $set[`services.${k}.ldapgroups`] = grps;

      const { autoRegisterGroups = [] } = ldapCfg;
      for (const autoG of autoRegisterGroups) {
        if (grps.includes(autoG)) {
          autoRegisterUser = true;
          return;
        }
      }
    } catch (err) {
      console.error(
        `[app] LDAP lookup user \`${username}\` in \`${ldap.url}\` failed.`,
        'err', err,
      );
    }
  });

  if (Object.keys($set).length === 0) {
    return;
  }

  // Set `ldapgroups` on services, and remove toplevel `ldapgroups` in order to
  // migrate to the new per-service scheme.
  const mod = {
    $set,
    $unset: { ldapgroups: '' },
  };
  if (autoRegisterUser) {
    mod.$addToSet = { roles: 'users' };
  }
  Meteor.users.update(user._id, mod);
}

Accounts.onLogin(updateUserFromLdap);

export {
  gitlabGetEmail,
  handleWellknownAccounts,
};
