// XXX `checkUserPluginScopes()` should not be used.  It is copied from
// `nog-error` primarily as a reminder that it is used there with `nog-auth`
// for `NogExec` keys, i.e. `NOG_KEYID` and `NOG_SECRETKEY`; see
// `nog-app/.../nog-exec-server.coffee`.
//
// In case we reconsider, `checkUserPluginScopes()` should be tested thoroughly
// before using it.

import {
  nogthrow,
  ERR_ACCESS_DENY,
} from './errors.js';

function checkScopes(scopes, action, opts) {
  for (const s of scopes) {
    if (s.action === action) {
      for (const [k, v] of Object.entries(s.opts)) {
        if (opts[k] !== v) {
          nogthrow(ERR_ACCESS_DENY, {
            reason: 'Scoped key opts mismatch.',
          });
        }
      }
      return;
    }
  }
  nogthrow(ERR_ACCESS_DENY, {
    reason: 'Insufficient key scope.',
  });
}

function checkUserPluginScopes(user, action, opts) {
  const { scopes } = user;
  if (scopes) {
    checkScopes(scopes, action, opts);
  }
}

export {
  checkUserPluginScopes,
};
