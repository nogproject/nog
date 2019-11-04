// `createMongoDocLock({ collection, docId, core })` creates a `lock` that will
// be maintained in the document `docId` in `collection`.  The lock is managed
// an array element `{ ...core, lockId, ts }` in the field `locks`.  Locks with
// the same `core` are mutually exclusive.  Locks with a different `core` can
// be acquired concurrently.
//
// A lock is acquired with `lock.tryLock()` and released with `lock.unlock()`.
// The caller must repeatedly call `lock.tryRenew()` during longer operations
// to protect agains forced expiry of stale locks.  `tryRenew()` is cheap and
// can be called frequently.
//
// The timing details are controlled by the args `renewIntervalS` (default 10
// seconds) and `expireTimeoutS` (default 30 seconds).  `tryRenew()` will
// update MongoDB only every `renewIntervalS` seconds.  `tryLock()` will expire
// locks that have not been renewed for `expireTimeoutS` seconds.
//
// The optional arg `logPrefix` enables logging of stale lock expiry.  Log
// entries will use the specified prefix.

import { Random } from 'meteor/random';


function createMongoDocLock({
  collection, docId, core, renewIntervalS = 10, expireTimeoutS = 30,
  logPrefix = null,
}) {
  return {
    collection, docId, core, renewIntervalS, expireTimeoutS, logPrefix,

    lockId: Random.id(),
    nextRenewalMs: 0,

    tryLock() {
      this.nextRenewalMs = Date.now() + (1000 * this.renewIntervalS);

      const coreLock = { ...this.core };

      const trySetLock = () => {
        const fullLock = { ...this.core, lockId: this.lockId, ts: new Date() };
        return !!this.collection.update(
          { _id: this.docId, locks: { $not: { $elemMatch: coreLock } } },
          { $push: { locks: fullLock } },
        );
      };

      if (trySetLock()) {
        return true;
      }

      const tryExpire = () => {
        const cutoff = new Date(Date.now() - (1000 * this.expireTimeoutS));
        const selExpired = { ...this.core, ts: { $lte: cutoff } };
        return !!this.collection.update(
          { _id: this.docId, locks: { $elemMatch: selExpired } },
          { $pull: { locks: coreLock } },
        );
      };

      if (!tryExpire()) {
        return false;
      }

      if (this.logPrefix) {
        console.log(
          `${logPrefix} expired stale lock ${JSON.stringify(coreLock)}.`,
        );
      }

      return trySetLock();
    },

    unlock() {
      const lock = { ...this.core, lockId: this.lockId };
      return !!this.collection.update(
        { _id: this.docId, locks: { $elemMatch: lock } },
        { $pull: { locks: lock } },
      );
    },

    tryRenew() {
      const now = Date.now();
      if (now < this.nextRenewalMs) {
        return true;
      }

      this.nextRenewalMs = now + (1000 * this.renewIntervalS);

      const lock = { ...this.core, lockId: this.lockId };
      return !!this.collection.update(
        { _id: this.docId, locks: { $elemMatch: lock } },
        { $currentDate: { 'locks.$.ts': true } },
      );
    },
  };
}


export { createMongoDocLock };
