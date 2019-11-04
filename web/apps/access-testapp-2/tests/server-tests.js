/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { Meteor } from 'meteor/meteor';

function describeTests() {
  describe('meteor test', function () {
    it('runs on server.', function () {
      expect(Meteor.isServer).to.equal(true);
    });
  });
}

function describeFullAppTests() {
  describe('meteor test --full-app', function () {
    it('runs on server.', function () {
      expect(Meteor.isServer).to.equal(true);
    });
  });
}

describe('server', function () {
  it('runs in test mode', function () {
    expect(Meteor.isTest || Meteor.isAppTest).to.equal(true);
  });

  if (Meteor.isTest) {
    describeTests();
  }
  if (Meteor.isAppTest) {
    describeFullAppTests();
  }
});
