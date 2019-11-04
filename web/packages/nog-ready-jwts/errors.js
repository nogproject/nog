import { nogthrow } from 'meteor/nog-error-2';

const ERR_UNKNOWN_READY_JWT = {
  errorCode: 'ERR_UNKNOWN_READY_JWT',
  statusCode: 404,
  sanitized: 'full',
  reason: 'Unknown JWT specification.',
};

export {
  ERR_UNKNOWN_READY_JWT,
  nogthrow,
};
