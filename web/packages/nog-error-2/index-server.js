// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';

import assert from 'assert';
import { createErrorModule } from './throw.js';

// `platform` allows late binding a logging collection by calling
// `setErrorLoggingCollection()` during app initialization.  If a logging
// collection is set, `nogthrow()` will store new errors before throwing them.
//
// The logging collection is primarily supported for backward compatibility
// with `nog-error`.  New applications should not use
// `setErrorLoggingCollection()`.  They should instead log only unhandled
// errors where appropriate, for example before returning them to a client.
const platform = {
  where: 'server',
  errorLog: null,
};

const NogError = createErrorModule({ platform });
const { createError, nogthrow } = NogError;

function setErrorLoggingCollection(coll) {
  assert.ok(
    platform.errorLog === null,
    'setErrorLoggingCollection() must be called only once.',
  );
  platform.errorLog = coll;
}

export {
  createErrorModule,
  setErrorLoggingCollection,
  createError,
  nogthrow,
};
