import { nogthrow } from 'meteor/nog-error-2';

// XXX Error specs are defined here, because`nog-error-2` currently does not
// contain common error specs, different from package `nog-error`, which
// contained common error specs.  We should later reconsider whether to move
// error specs to a common package.  Depending on the decision, we would then
// either move the specs or remove this comment.

const ERR_ACCESS_DENY = {
  errorCode: 'ERR_ACCESS_DENY',
  statusCode: 404,
  sanitized: 'full',
  reason: 'Access denied by policy.',
};

const ERR_ACCESS_DEFAULT_DENY = {
  errorCode: 'ERR_ACCESS_DEFAULT_DENY',
  statusCode: 404,
  sanitized: 'full',
  reason: 'Access denied without policy.',
};

export {
  nogthrow,
  ERR_ACCESS_DENY,
  ERR_ACCESS_DEFAULT_DENY,
};
