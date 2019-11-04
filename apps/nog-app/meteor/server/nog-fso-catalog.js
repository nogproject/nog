import { _ } from 'meteor/underscore';
import { Meteor } from 'meteor/meteor';
import { NogError } from 'meteor/nog-error';
const {
  ERR_CONFLICT,
} = NogError;

// `maxQueueSize` is the maximum number of pending repos that require an
// update.  Further events will be dropped.
const maxQueueSize = 1000;
// `batchSize` is the number of `selectRepos` that are passed to
// `updateCatalogFso()` at once.
const batchSize = 100;
// `throttleWaitS` is the rate limit that is effective when repo changes arrive
// at a higher rate.
const throttleWaitS = 5;

// `retryMinMs` is the minimum duration until retry.  A random value up to
// `retryJitterMs` is added.
const retryMinMs = 5000;
const retryJitterMs = 3000;

function loginfo(msg, ...args) { console.log(`[app] ${msg}`, ...args); }
function logwarn(msg, ...args) { console.log(`[app] ${msg}`, ...args); }
function logerr(msg, ...args) { console.error(`[app] ${msg}`, ...args); }

// XXX The updater should perhaps be moved to `packages/nog-catalog-fso`.

function createFsoCatalogUpdater({
  catalogs, updateCatalogFso,
}) {
  // `queue` contains repo IDs that are applied to catalogs in the cluster
  // parts that this updater is responsible for.
  const queue = [];

  // `queueLocal` contains repo IDs from local events.  They are applied to all
  // catalogs, independently of cluster parts.
  const queueLocal = [];

  // `parts` are `NogCluster` parts for which this updater is responsible.
  // Key: `part.begin`, value: `part`.
  const parts = new Map();

  // `isScheduled` indicates whether a call to `updateCatalogs()` has been
  // scheduled.  Calls can be scheduled as a retry `setTimeout()` or from an
  // event via `updateCatalogsThrottled()`.
  let isScheduled = false;

  // `partsMongoQuery()` returns a MongoDB query to find catalogs in cluster
  // parts for which this updater is responsible.
  function partsMongoQuery() {
    if (parts.size === 0) {
      return null;
    }
    return {
      $or: Array.from(parts.values()).map(p => ({ _id: p.sel })),
      fsoPath: { $exists: true },
    };
  }

  // `updateCatalogs()` processes a batch of repo IDs.  It removes the batch at
  // the end of the while loop to support retry without requeuing.  Removal can
  // be safely deferred, because this function is the only place that removes
  // from the queues.
  //
  // The `selectRepos` entries should perhaps be deduplicated.  Deduplication
  // cannot be simply achieved with `Set`, however, because the entries are
  // `Buffer` objects.
  //
  // As long as `queueLocal` is only used for manual update retriggers, events
  // should be rare.  It seems sufficient to simply process them as a separate
  // batch without optimization.
  function updateCatalogs() {
    isScheduled = false;

    // `updateOneCatalog()` sets `retry` on concurrency errors to tell the
    // while loop to stop.
    let retry = false;

    function updateOneCatalog({ fsoPath, updateUser }, repos) {
      // Call `updateCatalogFso()` as the user that last updated the catalog.
      // This ensures that the background update runs with permissions of a
      // real user that controls the catalog.  A catalog owner must
      // explicitly update the catalog once to set the update user and enable
      // background updates.
      const euidUpdate = updateUser ? (
        Meteor.users.findOne(updateUser)
      ) : (
        null
      );
      if (!euidUpdate) {
        logwarn(
          'Skipped background catalog update without update user.',
          'catalog', fsoPath,
        );
        return;
      }

      loginfo(
        'Started background catalog update.',
        'catalog', fsoPath,
        'euid', euidUpdate._id,
        'username', euidUpdate.username,
      );
      try {
        updateCatalogFso(euidUpdate, {
          repoPath: fsoPath,
          selectRepos: repos,
        });
      } catch (err) {
        // Retry concurrency conflicts.  But ignore other errors to ensure
        // that repo-specific errors repos do not block the queue.
        if (err.errorCode === ERR_CONFLICT.errorCode) {
          loginfo(
            'Concurrent catalog update.',
            'catalog', fsoPath,
          );
          retry = true;
          return;
        }
        logerr(
          'Failed to update catalog; dropped repo ids.',
          'catalog', fsoPath,
          'err', err,
        );
      }
    }

    while (queue.length > 0 || queueLocal.length > 0) {
      const selectRepos = queue.slice(0, batchSize);
      const selectReposLocal = queueLocal.slice(0, batchSize);

      const fields = { fsoPath: 1, updateUser: 1 };
      const selParts = partsMongoQuery();
      const selLocal = { fsoPath: { $exists: true } };

      if (selectReposLocal.length > 0) {
        loginfo(
          'Started processing local catalog update batch.',
          'n', selectReposLocal.length,
        );
        catalogs.find(selLocal, { fields }).forEach((cat) => {
          updateOneCatalog(cat, selectReposLocal);
        });
      }

      if (selParts && selectRepos.length > 0) {
        loginfo(
          'Started processing catalog update batch.',
          'n', selectRepos.length,
        );
        catalogs.find(selParts, { fields }).forEach((cat) => {
          updateOneCatalog(cat, selectRepos);
        });
      }

      if (retry) {
        if (!isScheduled) {
          isScheduled = true;
          const retryInMs = Math.round(
            retryMinMs + (Math.random() * retryJitterMs),
          );
          Meteor.setTimeout(updateCatalogs, retryInMs);
          loginfo(
            'Scheduled retry update catalog.',
            'retryInMs', retryInMs,
          );
        }
        return;
      }

      queue.splice(0, batchSize);
      queueLocal.splice(0, batchSize);
    }
  }

  const updateCatalogsThrottled = _.throttle(
    Meteor.bindEnvironment(updateCatalogs), throttleWaitS * 1000,
  );

  function handleGitRefUpdate(ev) {
    const { repoId, ref } = ev;

    if (ref !== 'refs/heads/master-meta') {
      return;
    }

    if (queue.length >= maxQueueSize) {
      loginfo(
        'Ignored repo change, catalog update queue full.',
        'repoId', repoId,
      );
      return;
    }

    queue.push(repoId);
    if (!isScheduled) {
      // Set `isScheduled` first, since throttled may immediately call the
      // wrapped function.
      isScheduled = true;
      updateCatalogsThrottled();
    }
  }

  return {
    broadcastSubscription: null,

    update({ repoFsoId }) {
      if (queueLocal.length >= maxQueueSize) {
        loginfo(
          'Ignored `update()`, local catalog update queue full.',
          'repoId', repoFsoId,
        );
        return;
      }
      queueLocal.push(repoFsoId);
      if (!isScheduled) {
        // Set `isScheduled` first, since throttled may immediately call the
        // wrapped function.
        isScheduled = true;
        updateCatalogsThrottled();
      }
    },

    startWatchBroadcast(broadcast) {
      const s = broadcast.subscribeGitRefUpdatedAll(handleGitRefUpdate);
      this.broadcastSubscription = s;
    },

    stopWatchBroadcast(broadcast) {
      if (this.broadcastSubscription) {
        broadcast.unsubscribe(this.broadcastSubscription);
        this.broadcastSubscription = null;
      }
    },

    startPart(part) {
      parts.set(part.begin, part);

      let paths = 'unknown';
      try {
        const sel = {
          _id: part.sel,
          fsoPath: { $exists: true },
        };
        const fields = { fsoPath: 1 };
        paths = catalogs.find(sel, { fields }).map(d => d.fsoPath);
      } catch (err) {
        // Ignore error, since the `paths` are only for information.
      }

      loginfo(
        'Took responsibility for updating fso catalogs.',
        'part', part.selHuman,
        'catalogs', paths,
      );
    },

    stopPart(part) {
      parts.delete(part.begin);
      loginfo(
        'Dropped responsibility for updating fso catalogs.',
        'part', part.selHuman,
      );
    },
  };
}

export {
  createFsoCatalogUpdater,
};
