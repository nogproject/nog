// @flow

import { _ } from 'meteor/underscore';
import { Meteor } from 'meteor/meteor';
import { Random } from 'meteor/random';

import { NogError } from 'meteor/nog-error';
const {
  ERR_SYNCHRO_SNAPSHOT_INVALID,
  ERR_LOGIC,
  ERR_SYNCHRO_APPLY_FAILED,
  nogthrow,
} = NogError;

import { synchroCommitReposDiffStream } from './nog-sync-diff.coffee';

const NULL_SHA1 = '0000000000000000000000000000000000000000';


function isDuplicateMongoIdError(err) {
  return err.code === 11000;
}


type applySnapshotDiffAnonC$opts = {
  syncStore: {
    contentStore: {
      repos: any,
      deletedRepos: any,
    },
    getCommitSudo: (opts: { sha: string }) => {},
    getTreeSudo: (opts: { sha: string }) => {},
  },
  aCommitSha: string,
  bCommitSha: string,
};

function applySnapshotDiffAnonC(
  euid: any,
  { syncStore, aCommitSha, bCommitSha }: applySnapshotDiffAnonC$opts
) {
  const log = console.log;

  const { contentStore } = syncStore;

  function onadded({ b }) {
    // Based on code copied from `NogContent.Store.createRepo()`.

    // XXX Users are not yet part of the snapshot.  When they become part of
    // the snapshot, they should have been added at this point.  A missing user
    // should then be considered a logic error.
    //
    // For now, simply map unknown users to `ownerId: unknown`.

    const nog = b.meta.nog;
    const repoFullName = `${nog.owner}/${nog.name}`;
    const ownerDoc = Meteor.users.findOne(
      { username: nog.owner },
      { fields: { _id: 1, username: 1 } }
    );
    let ownerId;
    if (ownerDoc != null) {
      ownerId = ownerDoc._id;
    } else {
      console.error(
        `[sync] Warning: missing user for repo owner ${nog.owner}`
      );
      ownerId = 'unknown';
    }

    const master = nog.refs['branches/master'];
    const cmasters = nog.conflicts['branches/master'];
    let refs;
    let conflicts;
    if (master != null && cmasters == null) {
      refs = {
        'branches/master': master,
      };
      conflicts = {};
    } else if (master == null && cmasters != null) {
      refs = {
        'branches/master': NULL_SHA1,
      };
      conflicts = {
        'branches/master': cmasters,
      };
    } else {
      nogthrow(ERR_SYNCHRO_SNAPSHOT_INVALID);
    }

    try {
      contentStore.repos.insert({
        name: nog.name,
        owner: nog.owner,
        ownerId,
        refs, conflicts,
      });
    } catch (err) {
      if (!isDuplicateMongoIdError(err)) {
        throw err;
      }

      if (master != null) {
        // Synchro snapshot contains a single, conflict-free sha.
        //
        // Check if the conflict with the local master is trivial.  If so,
        // complete successfully.  Otherwise store the conflict.

        const trivialResolution = contentStore.repos.findOne({
          owner: nog.owner,
          name: nog.name,
          'refs.branches/master': master,
        });
        if (trivialResolution != null) {
          log(`[sync] AT ${repoFullName}`);
          return;
        }

        const n = contentStore.repos.update(
          { owner: nog.owner, name: nog.name },
          {
            $set: { 'conflicts.branches/master': [master] },
            $currentDate: { mtime: true },
          },
        );
        if (n === 1) {
          log(`[sync] AC ${repoFullName}`);
          return;
        }

        nogthrow(ERR_SYNCHRO_APPLY_FAILED, {
          reason: `Failed AC ${repoFullName}`,
        });

        return;
      } else if (cmasters != null) {
        // Synchro snapshot contains a conflict.
        //
        // Try to remove the local master from the conflict set before storing
        // it.

        const localMaster = contentStore.repos.findOne({
          owner: nog.owner,
          name: nog.name,
        }).refs['branches/master'];

        const n = contentStore.repos.update(
          { owner: nog.owner, name: nog.name },
          {
            $set: {
              'conflicts.branches/master': _.without(cmasters, localMaster),
            },
            $currentDate: { mtime: true },
          },
        );
        if (n === 1) {
          log(`[sync] AC ${repoFullName}`);
          return;
        }

        nogthrow(ERR_SYNCHRO_APPLY_FAILED, {
          reason: `Failed to AC ${repoFullName}`,
        });

        return;
      }

      nogthrow(ERR_LOGIC);
    }
    log(`[sync] A ${repoFullName}`);
  }

  function ondeleted({ a }) {
    const nog = a.meta.nog;
    const arefs = nog.refs;
    const repoFullName = `${nog.owner}/${nog.name}`;

    // Only the old master master is checked.  The repo is deleted even if
    // there are conflicts recorded in the repo.  The assumption is that a
    // conflict resolution would have modified master.

    const oldMaster = (
      arefs['branches/master'] ||
      'unknown'  // XXX Unclear whether this can happen.
    );

    const sel = {
      owner: nog.owner,
      name: nog.name,
      'refs.branches/master': oldMaster,
    };

    // If the repo is not found, assume it is an old master mismatch and, in a
    // best effort, set the conflicts to NULL_SHA1 to indicates that it has
    // been deleted elsewhere.  If the update that sets NULL_SHA1 fails, assume
    // that the repo has already been deleted by a previous apply that has been
    // interrupted.

    const repo = contentStore.repos.findOne(sel);
    if (repo == null) {
      contentStore.repos.update(
        { owner: nog.owner, name: nog.name },
        {
          $set: { 'conflicts.branches/master': [NULL_SHA1] },
          $currentDate: { mtime: true },
        },
      );
      return;
    }

    // Store a duplicate in deletedRepos, handling double delete gracefully to
    // support restarting interrupted applies or other complex conflict
    // situations, which could cause a race condition between the `findOne()`
    // above and the `remove()` below.

    try {
      contentStore.deletedRepos.insert(repo);
    } catch (err) {
      if (!isDuplicateMongoIdError(err)) {
        throw err;
      }
      repo._id = `${repo._id}-${Random.id()}`;
      contentStore.deletedRepos.insert(repo);
    }

    const n = contentStore.repos.remove(sel);
    contentStore.deletedRepos.update(
      repo._id, { $currentDate: { mtime: true } },
    );
    if (n === 0) {
      log(`[sync] Failed to D ${repoFullName}`);
    } else if (n === 1) {
      log(`[sync] D ${repoFullName}`);
    } else {
      nogthrow(ERR_LOGIC, {
        reason: 'Unexpectedly removed more than one repo.',
      });
    }
  }

  function onmodified({ a, b }) {
    const arefs = a.meta.nog.refs;
    const aconflicts = a.meta.nog.conflicts;
    const brefs = b.meta.nog.refs;
    const bconflicts = b.meta.nog.conflicts;
    const nog = b.meta.nog;
    const repoFullName = `${nog.owner}/${nog.name}`;

    const master = brefs['branches/master'];
    const cmasters = bconflicts['branches/master'];

    if (cmasters != null) {
      // The b master has conflicts.  Store them w/o the local master.
      const sel = { owner: nog.owner, name: nog.name };
      const repo = contentStore.repos.findOne(sel);
      const localMaster = repo.refs['branches/master'];
      const n = contentStore.repos.update(
        sel,
        {
          $set: {
            'conflicts.branches/master': _.without(cmasters, localMaster),
          },
          $currentDate: { mtime: true },
        },
      );
      if (n === 0) {
        // The repo might have been concurrently deleted.
        log(`[sync] MD ${repoFullName}`);
      } else if (n === 1) {
        log(`[sync] MC ${repoFullName}`);
      } else {
        nogthrow(ERR_LOGIC);
      }

      return;
    }

    // Conflicting B master has been handled. From here on, single
    // conflict-free B master, i.e. master != null, cmaster == null.

    // Update only if the local master matches the A master, accepting both
    // conflicting or non-concflicting.  Also accept if the local master
    // already points to the B master; maybe after an interrupted apply.

    const oldMasters = (
      aconflicts['branches/master'] || [arefs['branches/master']]
    );
    const nm = contentStore.repos.update(
      {
        owner: nog.owner,
        name: nog.name,
        'refs.branches/master': { $in: [master, ...oldMasters] },
      },
      {
        $set: { 'refs.branches/master': master },
        $unset: { 'conflicts.branches/master': '' },
        $currentDate: { mtime: true },
      }
    );
    if (nm === 1) {
      log(`[sync] M ${repoFullName}`);
      return;
    }

    // If update failed, assume that the reason was an A master mismatch.
    // Store the B master as conflicting.

    const nc = contentStore.repos.update(
      { owner: nog.owner, name: nog.name },
      {
        $set: { 'conflicts.branches/master': [master] },
        $currentDate: { mtime: true },
      },
    );
    if (nc === 0) {
      // Repo might have been deleted concurrently.
      log(`[sync] M? ${repoFullName}`);
    } else if (nc === 1) {
      log(`[sync] ML ${repoFullName}`);
      return;
    } else {
      nogthrow(ERR_LOGIC);
    }

    return;
  }

  synchroCommitReposDiffStream({
    aSha: aCommitSha,
    bSha: bCommitSha,
    ondeleted, onadded, onmodified,
    store: {
      getCommit: (sha) => syncStore.getCommitSudo({ sha }),
      getTree: (sha) => syncStore.getTreeSudo({ sha }),
    },
  });
}

export { applySnapshotDiffAnonC };
