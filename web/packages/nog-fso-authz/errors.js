import { nogthrow } from 'meteor/nog-error-2';
import { ERR_LOGIC } from 'meteor/nog-error-codes';

const ERR_PARAM_INVALID = {
  errorCode: 'ERR_PARAM_INVALID',
  statusCode: 422,
  sanitized: 'full',
  reason: 'A parameter was semantically invalid.',
};

export {
  ERR_LOGIC,
  ERR_PARAM_INVALID,
  nogthrow,
};
