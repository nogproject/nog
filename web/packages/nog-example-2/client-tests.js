/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { Meteor } from 'meteor/meteor';

import { clientSum, createExampleModuleClient } from 'meteor/nog-example-2';

import { fakeOne, fakeTwo } from './testlib.js';
import { describeCommonTests } from './common-tests.js';

describe('nog-example-2 client', function () {
  it('runs on client', function () {
    expect(Meteor.isClient).to.equal(true);
  });

  describeCommonTests();

  it('has clientSum()', function () {
    expect(clientSum(fakeOne, fakeTwo)).to.equal('client: 3');
  });

  describe('createExampleModuleClient()', function () {
    it('supports clientName injection', function () {
      const clientName = 'fooClient';
      const NogExample = createExampleModuleClient({ clientName });
      expect(NogExample.clientSum(2, 3)).to.equal(`client ${clientName}: 5`);
    });
  });
});
