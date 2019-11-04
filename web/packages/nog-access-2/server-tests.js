/* eslint-env mocha */
/* eslint-disable func-names */

import { describeAuthzTests } from './authz-tests.js';
import { describeStatementsTests } from './statements-tests.js';
import { describeScopesLegacyTests } from './scopes-legacy-tests.js';
import { describeScopesV2Tests } from './scopes-v2-tests.js';

describe('nog-access-2 server', function () {
  describeAuthzTests();
  describeStatementsTests();
  describeScopesLegacyTests();
  describeScopesV2Tests();
});
