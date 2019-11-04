/* eslint-disable no-underscore-dangle */
/* eslint-disable camelcase */

import { Meteor } from 'meteor/meteor';
import { MongoInternals } from 'meteor/mongo';


// The copy order achieves weak consistency at the destination during
// collection copy.  Repos, for example, appear after the commits and the users
// have been copied.  It is only a weak consistency, since repos might be
// modified at the source after copying the commits.  Eventual consistency will
// be achieved when the oplog tailer copies the new commits.
//
// `nogcontent.repo_sets` is needed to maintain repo membership information for
// entries that are not yet reachable from a ref.  A client expects to be able
// to use recently inserted objects and trees to create a new commit.  When we
// switch from one db to another after entries have been added but before they
// have been linked to a ref, `repo_sets` is the only collection that has the
// current repo membership information.
//
// `content_index*` is included to keep the initial startup time of an app that
// uses the destination database low.

// `meteor_accounts_loginServiceConfiguration` is excluded.  The app is
// expected to re-insert the OAuth config from the settings.

// `uploads` is excluded.  Clients are expected to retry operations after the
// planned maintenance window that is necessary to switch dbs.

const nogCollections = [
  'migrations',
  'blobs',
  'objects',
  'trees',
  'commits',
  'nogcontent.repo_sets',
  'shares',
  'users',
  'roles',
  'content_index',
  'content_index_state',
  'repos',
  'nogexec.jobs.jobs',
  'trash.repos',
];


function splitFirst(s, c) {
  const [head, ...tail] = s.split(c);
  return [head, tail.join(c)];
}


function getOplogTailTs({ oplogurl }) {
  const opdb = new MongoInternals.RemoteCollectionDriver(oplogurl);
  const oplog = opdb.open('oplog.rs');
  const last = oplog.findOne({ }, { sort: { $natural: -1 } });
  return last.ts;
}


// `tailForever()` does not handle rollbacks during replica set failover.  As a
// consequence, the destination may contain spurious changes after a failover.
// As a safety measure, `tailForever()` refuses to start if it does not find
// the stored state `ts` in the oplog.  It also reports failover on the oplog
// source.  These safety measures seem good enough for supervised one time-sync
// admin tasks, but maybe not be safe enough for long-term unattended
// background replication.
//
// Alternatives:
//
// - Implement replication with Mongo Connector, which correctly handles
//   rollback; see Mongo Connector wiki 'Writing Your Own DocManager', method
//   `search()`, <https://goo.gl/DV1UZY>.
//
// - Implement rollback during failover similar to Mongo Connector.  Their
//   strategy is to check whether a tailable cursor is empty in order to detect
//   a rollback situation.  If they detect rollback, the cursor init procedure
//   searches back from the stored state ts to the last ts in the oplog and
//   performs rollback of the ops in the destination.  For details, grep 'def
//   rollback' in repo `github.com/mongodb-labs/mongo-connector`.
//
// - Reliably detect a rollback situation and report an error so that a human
//   can resolve the situation.  The check whether the stored state ts is still
//   in the oplog should protect against rollback during cursor initialization.
//   XXX We need to clarify whether there are situations in which the driver
//   automatically reconnects and reinitializes the cursor without notice.

function tailForever(
  { dsturl, oplogurl, dbns, afterTs, collections, saveState }
) {
  const dstdb = new MongoInternals.RemoteCollectionDriver(dsturl);
  const opdb = new MongoInternals.RemoteCollectionDriver(oplogurl);
  const oplog = opdb.open('oplog.rs');
  const cursor = oplog.find(
    {
      ts: { $gt: afterTs },
      ns: { $in: collections.map((c) => `${dbns}.${c}`) },
    },
    {
      // Meteor automatically sets `awaitdata` and related; see Meteor source
      // `mongo_driver.js`, <https://goo.gl/zGqGLh>.
      tailable: true,
    },
  );

  // Use an internal API to get notified of MongoDB failover on the oplog as a
  // warning that rollback might have happened.

  opdb.mongo._onFailover(() => {
    console.log('mongo failover of source oplog to', opdb.mongo._primary);
  });

  // Confirm that the oplog still contains `afterTs` to protect against loosing
  // ops when falling too far behind or spurious ops in a rollback situation.
  //
  // Use a separate find instead of `$gte` in the tailing find above, since the
  // tailing find uses further query restrictions, so that the doc that has
  // `afterTs` may not be part of the result set.  The tailing find cannot use
  // `$or` to ensure that `afterTs` would be in the result set, since the
  // Meteor implementation requires a toplevel `ts` to detect that the magic
  // Mongo replay oplog option should be used to find the cursor start
  // efficiently.

  // XXX `findOne()` takes 15 seconds on a Compose production db with 100k
  // entries to find `afterTs`.  The find probably involves a table scan, which
  // could perhaps be avoided by using a magic oplog tailing flag.

  console.log('Verifying that oplog contains saved timestamp state.');
  if (oplog.findOne({ ts: afterTs }) == null) {
    throw new Error(
      'The start tailing timestamp is not in the oplog.  ' +
      'It may indicate that the oplog has been truncated, ' +
      'or a rollback may have happened.  ' +
      'You should analyze the situation and consider forcing a full sync.'
    );
  }
  console.log('Found saved timestamp; good.');

  // Use the internal `tail()` API, because we want to track `ts` to restart
  // tailing.  But the Meteor MongoDB driver deletes `ts` before delivering the
  // doc; see `delete doc.ts` in Meteor source `mongo_driver.js`
  // <https://goo.gl/5ZpAJS>.  As a consequence, the following does not work:
  //
  // ```
  // cursor.observe({ added(op) { /* `ts` is missing. */ } });
  // ```

  // Inserts are executed as upserts that replace existing docs to avoid
  // duplicate id errors, assuming that the doc might have been inserted
  // earlier by an oplog replay and already has a state that corresponds to an
  // oplog entry that we will reach later; the eventually consistent state will
  // be achieved by simply continuing to apply the oplog.  Mongo Connector uses
  // essentially the same approach; see Mongo Connector wiki page 'System
  // Overview'
  // <https://github.com/mongodb-labs/mongo-connector/wiki/System%20Overview>.

  cursor._mongo.tail(cursor._cursorDescription, (doc) => {
    const collName = splitFirst(doc.ns, '.')[1];
    if (doc.op === 'u') {
      console.log('update', collName, doc.o2._id);
      dstdb.open(collName).update(doc.o2, doc.o);
    } else if (doc.op === 'i') {
      console.log('upsert', collName, doc.o._id);
      dstdb.open(collName).upsert({ _id: doc.o._id }, doc.o);  // replaces doc.
    } else if (doc.op === 'd') {
      console.log('remove', collName, doc.o._id);
      dstdb.open(collName).remove(doc.o);
    } else {
      console.log('ignoring op', doc);
    }
    saveState({ afterTs: doc.ts });
  });
}


// `copyCollectionOneByOne()` and `copyCollectionBulk()` yield the same result.
// The one by one copy code is much smaller and easier to understand.  The bulk
// copy gives 10x better performance (see timings below).
//
// A large batch size does not increase performance much, but it requires more
// memory.  Memory could become an issue for collections that contain large
// documents.  Objects could, in principle, be large.  Therefore, do not simply
// use the larges possible batch size.
//
// Measured copy performance from Compose classic us-east-1 to eu-west-1:
//
// - one by one: 1.5k objects / minute.
// - batchSize 100: 15k objects / minute.
// - batchSize 1000: 16k objects / minute.

// eslint-disable-next-line no-unused-vars
function copyCollectionOneByOne({ srcdb, dstdb, collection }) {
  const src = srcdb.open(collection);
  const dst = dstdb.open(collection);
  src.find({}, { sort: { $natural: 1 } }).map((doc) => {
    // Upsert to avoid duplicate id errors if the doc already exists.
    dst.upsert({ _id: doc._id }, doc);  // replaces doc.
    console.log('upsert', collection, doc._id);
    return null;
  });
}

function copyCollectionBulk({ srcdb, dstdb, collection }) {
  const src = srcdb.open(collection);
  const dst = dstdb.open(collection);

  const batchSize = 200;  // See discussion in comment above.
  let nPending;
  let bulk = null;

  function ensureBulk() {
    if (bulk == null) {
      bulk = dst.rawCollection().initializeUnorderedBulkOp();
      nPending = 0;
    }
  }

  // `execute()` must be called only on non-empty bulks.
  function flush() {
    console.log('flush bulk', collection);
    const res = Meteor.wrapAsync(bulk.execute, bulk)();
    bulk = null;
    if (!res.ok) {
      throw new Error('Bulk execution failed.');
    }
  }

  function maybeFlush() {
    if (nPending >= batchSize) {
      flush();
    }
  }

  function finalFlush() {
    if (bulk != null) {
      flush();
    }
  }

  src.find({}, { sort: { $natural: 1 } }).map((doc) => {
    ensureBulk();
    // Upsert to avoid duplicate id errors if the doc already exists.
    bulk.find({ _id: doc._id }).upsert().replaceOne(doc);
    nPending += 1;
    console.log('upsert', collection, doc._id);
    maybeFlush();
    return null;
  });

  finalFlush();
}


function copyCollections({ srcurl, dsturl, collections }) {
  const srcdb = new MongoInternals.RemoteCollectionDriver(srcurl);
  const dstdb = new MongoInternals.RemoteCollectionDriver(dsturl);
  for (const collection of collections) {
    copyCollectionBulk({ srcdb, dstdb, collection });
  }
}


function syncNogDbForever(
  { src, dst, states, stateId, optForceFullCopy, waitBeforeCopy_s }
) {
  const { oplogurl, dbns } = src;
  const srcurl = src.url;
  const dsturl = dst.url;
  const collections = nogCollections;

  console.log(`Using stateId '${stateId}'.`);

  // Save the state after every op.  We could perhaps save only every nths
  // state to use fewer upserts.  Some oplog operations would be applied again
  // after a restart, but the eventual result would be identical.  Storing
  // every state seems safer, however, since rollback situations can be
  // detected that could be missed when storing only every nth state.
  function saveState({ afterTs }) {
    states.upsert(stateId, { afterTs });
  }

  let afterTs;
  const state = states.findOne(stateId);
  let fullCopy;
  if (state == null) {
    console.log('No stored state; performing full copy.');
    fullCopy = true;
  } else if (optForceFullCopy) {
    console.log('Forced full copy.');
    fullCopy = true;
  }
  if (fullCopy) {
    afterTs = getOplogTailTs({ oplogurl });

    console.log(
      'Captured oplog tail timestamp; ' +
      `waiting ${waitBeforeCopy_s}s before starting full copy; ` +
      'you may CTRL-C to break; no state has been stored so far.'
    );
    Meteor._sleepForMs(waitBeforeCopy_s * 1000);

    // We do not drop destination collections before copying, so that existing
    // docs are preserved unless they are directly modified by the sync.

    copyCollections({ srcurl, dsturl, collections });

    saveState({ afterTs });
  } else {
    console.log('Start tailing from stored state.');
    afterTs = state.afterTs;
  }

  console.log('Start tailing oplog.');
  tailForever({ dsturl, oplogurl, dbns, afterTs, collections, saveState });
}


export { syncNogDbForever };
