import { NogError } from 'meteor/nog-error';
const {
  ERR_ACCESS_DENY,
  ERR_LOGIC,
  ERR_PARAM_INVALID,
  nogthrow,
} = NogError;

const ERR_FSO = {
  errorCode: 'ERR_FSO',
  statusCode: 500,
  sanitized: null,
  reason: 'Generic fso error',
};

const ERR_FSO_CLIENT = {
  errorCode: 'ERR_FSO_CLIENT',
  statusCode: 500,
  sanitized: 'full',
  reason: 'Generic fso error',
};

export {
  ERR_ACCESS_DENY,
  ERR_FSO,
  ERR_FSO_CLIENT,
  ERR_LOGIC,
  ERR_PARAM_INVALID,
  nogthrow,
};
