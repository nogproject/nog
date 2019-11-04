// See also `../../backend/internal/fsoauthz/scopeauthz.go`.

import { NogError } from 'meteor/nog-error';
const {
  nogthrow,
  ERR_ACCESS_DENY,
} = NogError;

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

function scopeMatches(scope, action, opts) {
  if (!globIncludes(scope.actions, action)) {
    return false;
  }

  for (const [k, v] of Object.entries(opts)) {
    // Mapping to plural works for all scope V2 fields.  There are currently
    // only two: `paths` and `names`.  We may have to use a different mapping
    // if we add more fields.
    const plural = `${k}s`;
    if (!globIncludes(scope[plural], v)) {
      return false;
    }
  }

  return true;
}

function checkScopesV2(scopes, action, opts) {
  for (const sc of scopes) {
    if (scopeMatches(sc, action, opts)) {
      return;
    }
  }
  nogthrow(ERR_ACCESS_DENY, { reason: 'Insufficient scope v2.' });
}

export {
  checkScopesV2,
};
