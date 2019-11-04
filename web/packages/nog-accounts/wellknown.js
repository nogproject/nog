import { Match, check } from 'meteor/check';
import { OAuthEncryption } from 'meteor/oauth-encryption';
import {
  ERR_MERGE_WELLKNOWN_ACCOUNT,
  ERR_UPDATED_WELLKNOWN_ACCOUNT,
  nogthrow,
} from './errors.js';

const matchDev = Match.Where((x) => {
  check(x, String);
  return x === 'dev';
});

function createWellknownAccountsSetting({
  serviceNames,
}) {
  check(serviceNames, [String]);

  const matchAka = {};
  serviceNames.forEach((srv) => {
    matchAka[srv] = { username: String };
  });

  const matchWellknownAccount = Match.Where((x) => {
    check(x, matchAka);
    return Object.keys(x).length >= 2;
  });

  const matchWellknownAccounts = [matchWellknownAccount];

  const matchDevWellknownAccounts = Match.OneOf(
    matchDev, matchWellknownAccounts,
  );

  return {
    key: 'wellknownAccounts',
    val: [],
    help: `
\`wellknownAccounts\` is a list of account aliases.  If an alias to an existing
account is detected during account creation, the new login service will be
added to the existing account.  Example:

    Meteor.settings.wellknownAccounts: [
      {
        gitimp: { username: 'alice' },
        gitzib: { username: 'alovelace' },
      },
    ]

The supported services are: ${serviceNames.join(', ')}.

Use the special value \`dev\` to use the dev settings.
`,
    match: matchDevWellknownAccounts,
  };
}

// Like `pinEncryptedFieldsToUser()` in `accounts-base/accounts_server.js`.
function repinEncryptedFieldsToUser(service, { oldUserId, newUserId }) {
  const dup = { ...service };
  for (const [k, v] of Object.entries(dup)) {
    if (OAuthEncryption.isSealed(v)) {
      const cleartext = OAuthEncryption.open(v, oldUserId);
      dup[k] = OAuthEncryption.seal(cleartext, newUserId);
    }
  }
  return dup;
}

function createWellknownAccountsHandler({ users, serviceNames, settings }) {
  const byService = new Map();
  serviceNames.forEach((k) => {
    byService.set(k, new Map());
  });
  settings.forEach((aka) => {
    for (const [k, srv] of Object.entries(aka)) {
      byService.get(k).set(srv.username, aka);
    }
  });

  function handleWellknownAccounts(partialUser) {
    let serviceName = null;
    let serviceUsername = null;
    let service = null;
    for (const n of serviceNames) {
      const s = partialUser.services[n];
      if (s) {
        serviceName = n;
        serviceUsername = s.username;
        service = s;
        break;
      }
    }
    if (!serviceName) {
      return;
    }

    const aka = byService.get(serviceName).get(serviceUsername);
    if (!aka) {
      return;
    }

    const sel = { $or: [] };
    for (const [k, srv] of Object.entries(aka)) {
      sel.$or.push({ [`services.${k}.username`]: srv.username });
    }

    const user = users.findOne(sel);
    if (!user) {
      return;
    }

    const $set = {
      [`services.${serviceName}`]: repinEncryptedFieldsToUser(service, {
        oldUserId: partialUser._id,
        newUserId: user._id,
      }),
    };
    const nUp = users.update(user._id, { $set });
    if (nUp !== 1) {
      nogthrow(ERR_MERGE_WELLKNOWN_ACCOUNT, {
        reason: 'Failed to add login service to existing user.',
      });
    }

    // Success is reported as an exception to abort adding the new account.
    // The user must repeat in order to sign in to the original account.
    nogthrow(ERR_UPDATED_WELLKNOWN_ACCOUNT, {
      reason: (
        `${serviceName} has been added as a new login service `
        + `to your existing account "${user.username}".  `
        + 'Sign in again to use the new login service.'
      ),
    });
  }

  return handleWellknownAccounts;
}

export {
  createWellknownAccountsHandler,
  createWellknownAccountsSetting,
};
