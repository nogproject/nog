/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { Meteor } from 'meteor/meteor';

import { serverSum, createExampleModuleServer } from 'meteor/nog-example-2';

import { fakeOne, fakeTwo } from './testlib.js';
import { describeCommonTests } from './common-tests.js';

describe('nog-example-2 server', function () {
  it('runs on server', function () {
    expect(Meteor.isServer).to.equal(true);
  });

  describeCommonTests();

  it('has serverSum()', function () {
    expect(serverSum(fakeOne, fakeTwo)).to.equal('server: 3');
  });

  describe('createExampleModuleServer()', function () {
    it('supports serverName injection', function () {
      const serverName = 'fooServer';
      const NogExample = createExampleModuleServer({ serverName });
      expect(NogExample.serverSum(2, 3)).to.equal(`server ${serverName}: 5`);
    });
  });
});
