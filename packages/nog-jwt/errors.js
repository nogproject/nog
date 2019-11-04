import { NogError } from 'meteor/nog-error';
const {
  nogthrow,
} = NogError;

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
  ERR_AUTH_UNKNOWN_JWT,
  ERR_AUTH_UNKNOWN_USER,
  ERR_INVALID_JWT,
  ERR_MALFORMED_AUTH_HEADER,
  nogthrow,
};
