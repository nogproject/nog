/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { nogthrow } from 'meteor/nog-error-2';
import { ERR_FAKE } from './testlib.js';

import './common-tests.js';

describe('nog-error-2 client', function () {
  it('throws "client" in error details', function () {
    function fn() {
      nogthrow(ERR_FAKE);
    }
    expect(fn).to.throw().with.property('details').that.matches(/client /);
  });
});
