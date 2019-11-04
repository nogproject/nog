import { OAuth } from 'meteor/oauth';

import { nogthrow, ERR_EMAIL_EXISTS } from './errors.js';
import { gitlabGetEmailAddress } from './gitlab.js';
import { uniqueUsername } from './username.js';

// `uniqueEmailAddressZedat()` tries aliases to construct a ZEDAT email address
// that is not yet used according to `isKnownEmail(addr)`.
//
// See ZEDAT aliases <https://www.zedat.fu-berlin.de/Aliasse>.
function uniqueEmailAddressZedat({
  isKnownEmail,
  preferredAddress,
  zedatUsername,
  subUsernames,
}) {
  const candidates = [
    preferredAddress,
    `${zedatUsername}@zedat.fu-berlin.de`,
  ].concat(
    subUsernames.map(p => `${p}@${zedatUsername}.dialup.fu-berlin.de`),
  );
  for (const addr of candidates) {
    if (!isKnownEmail(addr)) {
      return addr;
    }
  }
  nogthrow(ERR_EMAIL_EXISTS, {
    reason: 'All ZEDAT email aliases already used.',
  });
  return null;
}

// `createUserFuncGitimp()` is like `createUserFuncGitzib()` but for `git.imp`
// accounts; differences:
//
//  - `service` is typically `gitimp`.
//  - `domain` is typically `fu`.
//
function createUserFuncGitimp({
  isKnownUsername, isKnownEmail,
  service, gitlabUrl, domain, subUsernames,
}) {
  return function createUserGitimp(opts, partialUser) {
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
    address = uniqueEmailAddressZedat({
      isKnownEmail,
      preferredAddress: address,
      zedatUsername: srv.username,
      subUsernames,
    });
    user.emails = [{ address }];

    return user;
  };
}

export {
  createUserFuncGitimp,
};
