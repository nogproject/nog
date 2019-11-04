import { OAuth } from 'meteor/oauth';

import { nogthrow, ERR_EMAIL_EXISTS } from './errors.js';
import { gitlabGetEmailAddress } from './gitlab.js';
import { uniqueUsername } from './username.js';

const rgxZibEmailAddress = /^([a-z0-9+]+)@zib\.de$/;

// `uniqueEmailAddressZib()` tries aliases to construct a ZIB email address
// that is not yet used according to `isKnownEmail(addr)`.
function uniqueEmailAddressZib({
  isKnownEmail,
  preferredAddress,
  subUsernames,
}) {
  const candidates = [preferredAddress];
  const m = preferredAddress.match(rgxZibEmailAddress);
  if (m) {
    const name = m[1];
    subUsernames.forEach((sub) => {
      candidates.push(`${name}+${sub}@zib.de`);
    });
  }
  for (const addr of candidates) {
    if (!isKnownEmail(addr)) {
      return addr;
    }
  }
  nogthrow(ERR_EMAIL_EXISTS, {
    reason: 'All ZIB email aliases already used.',
  });
  return null;
}

// `createUserFuncGitzib()` returns a function that can be used to handle
// `Accounts.onCreateUser()`.  Params:
//
//  - `isKnownUsername(username)` controls if a username may be used.
//  - `isKnownEmail(username)` controls if an email may be used.
//  - `service` is the account service name, typically `gitzib`.
//  - `gitlabUrl` is the GitLab base URL for requesting the email address.
//  - `domain` is a suffix that may be used to construct unique usernames,
//    typically `zib`.
//  - `subUsernames` are names that may be added to the email address to make
//    it unique.
//
// The Meteor account system requires unique usernames and email addresses.
//
// Email addresses must be stored as an array of objects with field
// `address`; see `meteor/packages/accounts-base/accounts_server.js`.
function createUserFuncGitzib({
  isKnownUsername, isKnownEmail,
  service, gitlabUrl, domain, subUsernames,
}) {
  return function createUserGitzib(opts, partialUser) {
    const { services } = partialUser;
    if (!services) {
      return null;
    }
    const srv = services[service];
    if (!srv) {
      return null;
    }

    const user = { ...partialUser };

    user.username = uniqueUsername({
      isKnownUsername,
      domainUsername: srv.username,
      domain,
    });
    let address = gitlabGetEmailAddress({
      url: gitlabUrl,
      token: OAuth.openSecret(srv.accessToken, user._id),
    });
    address = uniqueEmailAddressZib({
      isKnownEmail,
      preferredAddress: address,
      subUsernames,
    });
    user.emails = [{ address }];

    return user;
  };
}

export {
  createUserFuncGitzib,
};
