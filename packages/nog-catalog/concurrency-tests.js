/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { Random } from 'meteor/random';
import { expect } from 'chai';

import { createMongoDocLock } from './concurrency.js';


describe('createMongoDocLock()', function () {
  let coll;
  let docId;

  before(function () {
    coll = new Mongo.Collection(`testing_catalog_concurrency_${Random.id()}`);
  });

  beforeEach(function () {
    docId = coll.insert({});
  });

  it('Bob cannot lock when Alice holds the lock.', function () {
    const op = 'foo';
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op },
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op },
    });
    expect(alice.tryLock()).to.eql(true);
    expect(bob.tryLock()).to.eql(false);
  });

  it('Bob can lock after Alice unlocked.', function () {
    const op = 'foo';
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op },
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op },
    });
    expect(alice.tryLock()).to.eql(true);
    expect(bob.tryLock()).to.eql(false);
    expect(alice.unlock()).to.eql(true);
    expect(bob.tryLock()).to.eql(true);
  });

  it('Bob can lock when Alice\'s lock expired.', function () {
    const op = 'foo';
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op },
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op }, expireTimeoutS: 0,
    });
    expect(alice.tryLock()).to.eql(true);
    expect(bob.tryLock()).to.eql(true);
  });

  it('Alice can renew a lock, but only if Bob did not expire it.' +
  '', function () {
    const op = 'foo';
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op }, renewIntervalS: 0,
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op }, expireTimeoutS: 0,
    });
    expect(alice.tryLock()).to.eql(true);
    expect(alice.tryRenew()).to.eql(true);
    expect(bob.tryLock()).to.eql(true);
    expect(alice.tryRenew()).to.eql(false);
  });

  it('Different core locks are independent.', function () {
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op: 'foo' },
      renewIntervalS: 0, expireTimeoutS: 0,
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op: 'bar' },
      renewIntervalS: 0, expireTimeoutS: 0,
    });
    expect(alice.tryLock()).to.eql(true);
    expect(bob.tryLock()).to.eql(true);
    expect(alice.tryRenew()).to.eql(true);
    expect(bob.tryRenew()).to.eql(true);
    expect(alice.unlock()).to.eql(true);
    expect(bob.unlock()).to.eql(true);
  });

  it('Bob can lock when Alice\'s lock expired after 1s.', function () {
    const op = 'foo';
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op },
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op }, expireTimeoutS: 1,
    });
    expect(alice.tryLock()).to.eql(true);
    expect(bob.tryLock()).to.eql(false);
    Meteor._sleepForMs(1100);
    expect(bob.tryLock()).to.eql(true);
  });

  it('Bob cannot lock when Alice renewed the lock after 1s.', function () {
    const op = 'foo';
    const alice = createMongoDocLock({
      collection: coll, docId, core: { op }, renewIntervalS: 0,
    });
    const bob = createMongoDocLock({
      collection: coll, docId, core: { op }, expireTimeoutS: 1,
    });
    expect(alice.tryLock()).to.eql(true);
    expect(bob.tryLock()).to.eql(false);
    Meteor._sleepForMs(1100);
    expect(alice.tryRenew()).to.eql(true);
    expect(bob.tryLock()).to.eql(false);
  });
});
