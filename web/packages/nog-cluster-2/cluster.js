/*

The implementation has been ported from `nog-cluster/nog-cluster-server.coffee`
without major refactoring, because there are no unit tests.

`cluster.IdPartition` together with `cluster.registerHeartbeat()` implements a
basic work distribution scheme for a cluster of application instances.

Each server chooses a random id `self` during startup and registers itself into
a TTL collection `nogcluster.members` with a regular `heartbeat()`.

Other parts of the application can instantiate `IdPartition`.  `cluster` will
try to acquire the responsibility for id partition parts and regularly renew
them using the TTL collection `leases`.  It calls `onacquire` when it acquired
a part and `onrelease` when it released a part.  The callbacks can start and
stop background maintenance tasks for the part, such as updating the search
index.

The current scheme assigns each part to a single app instance.  Each instance
computes the number of parts that it wants to acquired based on the number of
available parts and the number of cluster members.  The scheme uses
overallocation: several app instances compete for the parts.  It could later be
extended to allow multiple instances to acquire the same part to implement
redundant processing for failure scenarios.  The allocation scheme keeps
allocations fixed unless the number of cluster members changes substantially,
so that a large amount of startup work in `onacquire` should be acceptable,
since it will be amortized over time.

The background tasks must not assume that they have exclusive responsibility
for a part.  They also should not make assumption about when `onacquire` and
`onrelease` are called.  `onacquire` should probably do a full up-to-date check
or schedule a full up-to-date check at regular intervals to ensure eventual
consistency.

Many parameters of the allocation scheme are hard-coded.  We will incrementally
expose more parameters in the settings if needed to handle deployment and
testing.

`optSingleInstanceMode` controls whether an instance immediately takes
responsibility for all updates, which is recommended for testing but not for
production deployments with multiple app instances.

`IdPartition({ max })` controls the number of partitions for individual tasks,
such as `updateKinds` and `searchIndex`; see other source files.

The first cluster heartbeat runs after `firstHeartbeat_ms`, which should be
small to quickly get a fully functional app for testing.

Regular heartbeats run at intervals of `heartbeat_s`. `ttl_s` controls the
lease time in MongoDB.  `ttl_s` should be a few times longer than `heartbeat_s`
to ensure stable lease assignment.

*/

import { Meteor } from 'meteor/meteor';
import { Random } from 'meteor/random';
import { Mongo } from 'meteor/mongo';

const config = {
  ttl_s: 30,
  heartbeat_s: 10,
  firstHeartbeat_ms: 100,
};

const idAlphabet = (() => {
  const numbers = '0123456789';
  const lower = 'abcdefghijklmnopqrstuvwxyz';
  const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  return numbers + upper + lower;
})();

function idPartitions(max) {
  const max2 = Math.min(idAlphabet.length, max);
  const partSize = Math.ceil(idAlphabet.length / max2);
  const parts = [];
  const part = {};
  for (let i = 0; i < idAlphabet.length; i += partSize) {
    const x = idAlphabet[i];
    if (part.begin != null) {
      part.sel = { $gte: part.begin, $lt: x };
      part.selHuman = `\`${part.begin}\` <= id < \`${x}\``;
      parts.push(Object.assign({}, part));
    }
    part.begin = x;
  }
  part.sel = { $gte: part.begin };
  part.selHuman = `\`${part.begin}\` <= id`;
  parts.push(part);
  return parts;
}

// See <http://stackoverflow.com/q/1985260>.
function rotate(arr, n) {
  const end = arr.splice(n); // Remove n.. from end,
  arr.unshift(...end); // and insert at start.
  return arr;
}

function makeCollName(namespace, basename) {
  return `${namespace.coll}.${basename}`;
}

function createCluster({
  namespace, optSingleInstanceMode, optGlobalReadOnly,
}) {
  const cluster = {
    self: Random.id(),
    size: 0,
    watchers: [],
    registerHeartbeat(w) {
      return this.watchers.push(w);
    },
  };

  const members = new Mongo.Collection(makeCollName(namespace, 'members'));
  members.rawCollection().createIndex({
    heartbeat: 1,
  }, {
    expireAfterSeconds: config.ttl_s,
  });
  cluster.members = members;

  const leases = new Mongo.Collection(makeCollName(namespace, 'leases'));
  leases.rawCollection().createIndex({
    heartbeat: 1,
  }, {
    expireAfterSeconds: config.ttl_s,
  });
  cluster.leases = leases;

  // Don't wait for Mongo TTL to delete doc, but select by cutoff to calculate
  // cluster size to achieve a more predictable failover time.
  function heartbeat() {
    members.upsert({
      _id: cluster.self,
    }, {
      $currentDate: { heartbeat: true },
    });

    const cutoff = new Date();
    cutoff.setSeconds(cutoff.getSeconds() - config.ttl_s);
    const sel = { heartbeat: { $gt: cutoff } };
    cluster.size = members.find(sel).count();

    for (const w of cluster.watchers) {
      w.heartbeat();
    }
  }

  function foreverHeartbeat() {
    function nextHeartbeat() {
      try {
        // Handle all errors to ensure that the next call is scheduled even if
        // `heartbeat()` throws an unexpected error.
        heartbeat();
      } catch (err) {
        console.error(
          '[nog-cluster] Unexpected error in `heartbeat()`.', err.stack,
        );
      }
      Meteor.setTimeout(nextHeartbeat, config.heartbeat_s * 1000);
    }
    Meteor.setTimeout(nextHeartbeat, config.firstHeartbeat_ms);
  }

  if (optGlobalReadOnly) {
    console.log(
      '[nog-cluster] [GRO] Disabling cluster heartbeats in read-only mode.',
    );
  } else {
    foreverHeartbeat();
  }

  // `IdPartition` is stored on cluster, because its implementation refers to
  // `cluster`.
  cluster.IdPartition = class IdPartition {
    constructor({ name, max }) {
      this.name = name;
      this.overacquireFactor = 2;
      this.donthave = idPartitions(max);
      rotate(this.donthave, Math.floor(Math.random() * this.donthave.length));
      this.nParts = this.donthave.length;
      this.acquired = {};
      this.onacquire = () => {};
      this.onrelease = () => {};
    }

    nAcquired() {
      return Object.keys(this.acquired).length;
    }

    heartbeat() {
      const { name, nParts, overacquireFactor } = this;
      const wantMin = Math.min(
        nParts,
        Math.ceil(
          overacquireFactor * nParts
          / Math.max(1, cluster.size),
        ),
      );
      const wantMax = Math.min(nParts, 2 * wantMin);
      const nOld = this.nAcquired();
      if (this.nAcquired() < wantMin) {
        this.tryAcquire();
      } else if (this.nAcquired() > wantMax) {
        this.releaseOne();
      }
      this.confirm();
      if (this.nAcquired() !== nOld) {
        console.log(
          `[nog-cluster] Member ${cluster.self} now holds `
          + `${this.nAcquired()} of ${nParts} leases of \`${name}\`; `
          + `target: ${wantMin} to ${wantMax}.`);
      }
    }

    tryAcquire() {
      if (this.donthave.length === 0) {
        return;
      }

      const { self } = cluster;
      const part = this.donthave.shift();
      const id = `${this.name}.${part.begin}`;
      if (optSingleInstanceMode) {
        const n = leases.remove({
          _id: id,
          owner: { $ne: self },
        });
        if (n > 0) {
          console.log(
            `[nog-cluster] Force acquire lease \`${id}\` `
            + `in single instance mode.`,
          );
        }
      }
      try {
        leases.insert({
          _id: id,
          heartbeat: new Date(),
          owner: self,
        });
      } catch (err) {
        this.donthave.push(part);
        // Mongo code 11000 indicates duplicate _id.
        if (err.code !== 11000) {
          throw err;
        }
        console.log(
          `[nog-cluster] Member ${self} did not acquire lease \`${id}\`.`,
        );
        return;
      }
      console.log(`[nog-cluster] Member ${self} acquired lease \`${id}\`.`);
      this.acquired[part.begin] = part;
      this.onacquire(part);
    }

    releaseOne() {
      if (!(this.nAcquired() > 0)) {
        return;
      }

      const { self } = cluster;
      const part = Object.values(this.acquired)[0];
      const id = `${this.name}.${part.begin}`;
      leases.remove({
        _id: id,
        owner: self,
      });
      console.log(`[nog-cluster] Member ${self} released lease \`${id}\`.`);
      this.releasePart(part);
    }

    confirm() {
      const { self } = cluster;
      for (const part of Object.values(this.acquired)) {
        const id = `${this.name}.${part.begin}`;
        const n = leases.update({
          _id: id,
          owner: self,
        }, {
          $currentDate: { heartbeat: true },
        });
        if (n === 0) {
          console.log(`[nog-cluster] Member ${self} lost lease \`${id}\`.`);
          this.releasePart(part);
        }
      }
    }

    releasePart(part) {
      delete this.acquired[part.begin];
      this.donthave.push(part);
      this.onrelease(part);
    }
  };

  return cluster;
}

export {
  createCluster,
};
