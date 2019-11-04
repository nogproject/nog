import { nogthrow } from 'meteor/nog-error-2';

const ERR_FSO = {
  errorCode: 'ERR_FSO',
  statusCode: 500,
  sanitized: null,
  reason: 'Generic fso error',
};

export {
  ERR_FSO,
  nogthrow,
};
