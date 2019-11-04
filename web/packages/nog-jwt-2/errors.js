import { nogthrow } from 'meteor/nog-error-2';
import { ERR_LOGIC } from 'meteor/nog-error-codes';

const ERR_FSO = {
  errorCode: 'ERR_FSO',
  statusCode: 500,
  sanitized: null,
  reason: 'Generic fso error',
};

const ERR_ACCESS_DENY = {
  errorCode: 'ERR_ACCESS_DENY',
  statusCode: 404,
  sanitized: 'full',
  reason: 'Access denied by policy.',
};

// Malformed as in XML structure validation: the format of a value is invalid.
const ERR_PARAM_MALFORMED = {
  errorCode: 'ERR_PARAM_MALFORMED',
  statusCode: 422,
  sanitized: 'full',
  reason: 'A parameter was malformed.',
};

const ERR_MISSING_AUTH_HEADER = {
  errorCode: 'ERR_MISSING_AUTH_HEADER',
  statusCode: 401,
  sanitized: 'full',
  reason: 'Missing authorization header.',
};

const ERR_MALFORMED_AUTH_HEADER = {
  errorCode: 'ERR_MALFORMED_AUTH_HEADER',
  statusCode: 401,
  sanitized: 'full',
  reason: 'Malformed authorization header.',
};

const ERR_INVALID_JWT = {
  errorCode: 'ERR_INVALID_JWT',
  statusCode: 403,
  sanitized: 'full',
  reason: 'Invalid authorization header JWT.',
};

const ERR_AUTH_UNKNOWN_JWT = {
  errorCode: 'ERR_UNKNOWN_JWT',
  statusCode: 403,
  sanitized: 'full',
  reason: 'Unknown JWT.',
};

const ERR_AUTH_UNKNOWN_USER = {
  errorCode: 'ERR_AUTH_UNKNOWN_USER',
  statusCode: 404,
  sanitized: 'full',
  reason: 'Unknown user.',
};

export {
  ERR_ACCESS_DENY,
  ERR_AUTH_UNKNOWN_JWT,
  ERR_AUTH_UNKNOWN_USER,
  ERR_FSO,
  ERR_INVALID_JWT,
  ERR_LOGIC,
  ERR_MALFORMED_AUTH_HEADER,
  ERR_MISSING_AUTH_HEADER,
  ERR_PARAM_MALFORMED,
  nogthrow,
};
