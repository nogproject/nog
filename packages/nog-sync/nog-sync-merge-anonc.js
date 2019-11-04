import { _ } from 'meteor/underscore';
import { NogError } from 'meteor/nog-error';

const {
  ERR_REF_NOT_FOUND,
  ERR_LOGIC,
  ERR_SYNCHRO_CONTENT_MISSING,
  nogthrow,
} = NogError;

function isOfErr(err, spec) {
  return err.errorCode === spec.errorCode;
}

import { mergeBases } from './nog-sync-mergebases.js';
import { createStagingPrefixTree } from './nog-sync-merge-lib.coffee';
import { applySnapshotDiffAnonC } from './nog-sync-apply-anonc.js';

import {
  snapTreeShaForCommitSha,
  synchroCommitReposDiff3Stream,
} from './nog-sync-diff.coffee';


const NULL_SHA1 = '0000000000000000000000000000000000000000';


function isChanged(x) {
  return x != null;
}


function isDeleted(x) {
  return x === 'D';
}


function cloneLeaf(leaf) {
  const c = { ...leaf };
  c.meta = { ...c.meta };
  c.meta.nog = { ...c.meta.nog };
  return c;
}


// `mergeSnapshotsAnonC()` performs a proper merge of the two snapshots at
// `ourCommitSha` and `theirCommitSha` with an optional base `baseCommitSha`.
// It stores a conflict as an array that contains the alternative shas.

function mergeSnapshotsAnonC(
  euid,
  {
    syncStore, ownerName, synchroName,
    baseCommitSha, ourCommitSha, theirCommitSha,
    subject,
  }
) {
  const store = {
    getCommit(sha) {
      return syncStore.getCommitSudo({ sha });
    },
    getTree(sha) {
      return syncStore.getTreeSudo({ sha });
    },
  };

  const pfxTree = createStagingPrefixTree({
    store,
    rootSha: snapTreeShaForCommitSha(store, baseCommitSha),
  });

  function onchanged({ base, a, b }) {
    if (isDeleted(a) && isDeleted(b)) {
      pfxTree.del(base);
      return;
    }

    // D a M b -> conflict [NULL_SHA1, ...b].
    if (isDeleted(a) && isChanged(b)) {
      let cmasters = [NULL_SHA1].concat(
        b.meta.nog.conflicts['branches/master'] ||
        [b.meta.nog.refs['branches/master']]
      );
      cmasters.sort();
      cmasters = _.uniq(cmasters, /* isSorted: */ 1);

      const merged = cloneLeaf(b);
      merged.meta.nog.refs = {};
      merged.meta.nog.conflicts = {
        'branches/master': cmasters,
      };
      pfxTree.set(merged);

      return;
    }

    // M a D b -> conflict [NULL_SHA1, ...a].
    if (isChanged(a) && isDeleted(b)) {
      let cmasters = [NULL_SHA1].concat(
        a.meta.nog.conflicts['branches/master'] ||
        [a.meta.nog.refs['branches/master']]
      );
      cmasters.sort();
      cmasters = _.uniq(cmasters, /* isSorted: */ 1);

      const merged = cloneLeaf(a);
      merged.meta.nog.refs = {};
      merged.meta.nog.conflicts = {
        'branches/master': cmasters,
      };
      pfxTree.set(merged);

      return;
    }

    // M a M b -> conflict unless there is a trivial resolution.
    if (isChanged(a) && isChanged(b)) {
      let cmasters = [].concat(
        (
          a.meta.nog.conflicts['branches/master'] ||
          [a.meta.nog.refs['branches/master']]
        ),
        (
          b.meta.nog.conflicts['branches/master'] ||
          [b.meta.nog.refs['branches/master']]
        ),
      );
      cmasters.sort();
      cmasters = _.uniq(cmasters, /* isSorted: */ 1);

      const merged = cloneLeaf(a);
      if (cmasters.length === 1) {
        // Use trivial conflict resolution.
        merged.meta.nog.refs = {
          'branches/master': cmasters[0],
        };
        merged.meta.nog.conflicts = {};
      } else {
        merged.meta.nog.refs = {};
        merged.meta.nog.conflicts = {
          'branches/master': cmasters,
        };
      }
      pfxTree.set(merged);

      return;
    }

    if (isChanged(a)) {
      if (isDeleted(a)) {
        pfxTree.del(base);
      } else {
        pfxTree.set(a);
      }
      return;
    }

    if (isChanged(b)) {
      if (isDeleted(b)) {
        pfxTree.del(base);
      } else {
        pfxTree.set(b);
      }
      return;
    }

    nogthrow(ERR_LOGIC);
  }

  synchroCommitReposDiff3Stream({
    store, onchanged,
    baseSha: baseCommitSha,
    aSha: ourCommitSha,
    bSha: theirCommitSha,
  });

  const commit = store.getCommit(ourCommitSha);
  const root = { ...store.getTree(commit.tree) };
  delete root._id;
  delete root._idversion;  // eslint-disable-line no-underscore-dangle
  root.entries = [...root.entries];
  root.entries[0] = pfxTree.asNogTree();
  const rootSha = syncStore.createTree(
    euid, { ownerName, synchroName, content: root }
  );

  const merge = {
    subject,
    message: '',
    parents: [ourCommitSha, theirCommitSha],
    tree: rootSha,
  };
  const mergeSha = syncStore.createCommit(
    euid,
    { ownerName, synchroName, content: merge }
  );

  return mergeSha;
}


// `mergeRecursive()` uses Git's strategy to recursively merge multiple merge
// bases to create a virtual base in order to handle complex history like
// crisscross merges.

function mergeRecursive(
  euid,
  { syncStore, ownerName, synchroName, ourCommitSha, theirCommitSha, subject }
) {
  const store = {
    getCommitOrNull(sha) {
      try {
        return syncStore.getCommitSudo({ sha });
      } catch (err) {
        if (isOfErr(err, ERR_SYNCHRO_CONTENT_MISSING)) {
          return null;
        }
        throw err;
      }
    },
  };

  const mbs = mergeBases({
    ourSha: ourCommitSha, theirSha: theirCommitSha, store,
  });

  if (_.contains(mbs, theirCommitSha)) {
    return { status: 'up-to-date', commitSha: ourCommitSha };
  }

  if (_.contains(mbs, ourCommitSha)) {
    return { status: 'fast-forward', commitSha: theirCommitSha };
  }

  const ourCommit = syncStore.getCommitSudo({ sha: ourCommitSha });
  const theirCommit = syncStore.getCommitSudo({ sha: theirCommitSha });
  if (ourCommit.tree === theirCommit.tree) {
    const merge = {
      subject,
      message: '',
      parents: [ourCommitSha, theirCommitSha],
      tree: ourCommit.tree,
    };
    const commitSha = syncStore.createCommit(
      euid,
      { ownerName, synchroName, content: merge }
    );
    return { status: 'trivial-merge', commitSha };
  }

  console.log('[sync] merge', ourCommitSha, theirCommitSha);
  console.log('[sync] merge bases:', mbs.join(' '));

  if (mbs.length === 0) {
    const commitSha = mergeSnapshotsAnonC(
      euid,
      {
        syncStore, ownerName, synchroName,
        ourCommitSha, theirCommitSha, baseCommitSha: null,
        subject,
      }
    );
    return { status: 'merge-unrelated', commitSha };
  }

  let baseCommitSha = mbs[0];
  mbs.slice(1).forEach((mb) => {
    const { commitSha } = mergeRecursive(
      euid,
      {
        syncStore, ownerName, synchroName,
        ourCommitSha: baseCommitSha,
        theirCommitSha: mb,
        subject: 'virtual base',
      }
    );
    baseCommitSha = commitSha;
  });

  const commitSha = mergeSnapshotsAnonC(
    euid,
    {
      syncStore, ownerName, synchroName,
      ourCommitSha, theirCommitSha, baseCommitSha,
      subject,
    }
  );
  return { status: 'merge', commitSha };
}


function mergeSynchroAnonC(
  euid,
  { syncStore, ownerName, synchroName, branch, remoteName }
) {
  const refs = syncStore.getRefs(euid, { ownerName, synchroName });

  const ourRefName = `branches/${branch}`;
  const ourCommitSha = refs[ourRefName];
  if (ourCommitSha == null || ourCommitSha === NULL_SHA1) {
    nogthrow(ERR_REF_NOT_FOUND, { refName: ourRefName });
  }

  const remoteRefName = `remotes/${remoteName}/branches/${branch}`;
  const theirCommitSha = refs[remoteRefName];
  if (theirCommitSha == null || theirCommitSha === NULL_SHA1) {
    nogthrow(ERR_REF_NOT_FOUND, { refName: remoteRefName });
  }

  if (ourCommitSha === theirCommitSha) {
    return { status: 'up-to-date', commitSha: ourCommitSha };
  }

  const { status, commitSha } = mergeRecursive(
    euid,
    {
      syncStore, ownerName, synchroName,
      ourCommitSha, theirCommitSha,
      subject: `Merge '${remoteName}'`,
    }
  );

  if (status !== 'up-to-date' && status !== 'trivial-merge') {
    syncStore.setOp({ ownerName, synchroName, op: 'APPLYING', prevOp: '*' });
    applySnapshotDiffAnonC(euid, {
      syncStore, remoteName,
      aCommitSha: ourCommitSha,
      bCommitSha: commitSha,
    });
    /* eslint-disable no-underscore-dangle */
    // `_testingApplyErrorProb` allows tests to trigger fake errors to test
    // apply recovery.
    if (syncStore._testingApplyErrorProb &&
        Math.random() < syncStore._testingApplyErrorProb) {
      throw new Error('spurious fake apply error');
    }
    /* eslint-enable no-underscore-dangle */
    syncStore.clearOp({ ownerName, synchroName, prevOp: 'APPLYING' });
  }

  if (status !== 'up-to-date') {
    syncStore.updateRef(euid, {
      ownerName, synchroName,
      refName: ourRefName,
      old: ourCommitSha,
      new: commitSha,
    });
  }

  return { status, commitSha };
}


export { mergeSynchroAnonC };
