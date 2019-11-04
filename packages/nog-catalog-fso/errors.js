import { NogError } from 'meteor/nog-error';
const {
  nogthrow,
  ERR_NOT_OF_KIND,
} = NogError;

const ERR_FSO_CATALOG = {
  errorCode: 'ERR_FSO_CATALOG',
  statusCode: 500,
  sanitized: null,
  reason: 'Generic fso catalog error',
};

export {
  ERR_FSO_CATALOG,
  ERR_NOT_OF_KIND,
  nogthrow,
};
