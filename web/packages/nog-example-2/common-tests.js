/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { sum } from 'meteor/nog-example-2';

import { fakeOne, fakeTwo } from './testlib.js';

function describeCommonTests() {
  describe('common tests', function () {
    it('has sum()', function () {
      expect(sum(fakeOne, fakeTwo)).to.equal(3);
    });
  });
}

export {
  describeCommonTests,
};
