/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

import {
  describe, it, before, after,
} from 'meteor/practicalmeteor:mocha';

import { expect } from 'meteor/practicalmeteor:chai';

import { mergeSynchroAnonC } from './nog-sync-merge-anonc.js';

import { applySnapshotDiff } from './nog-sync-apply.js';
import { applySnapshotDiffAnonC } from './nog-sync-apply-anonc.js';

import { createTestPeers } from './nog-sync-peer-tests.coffee';
import { createContentFaker } from './nog-sync-store-tests.coffee';

const NULL_SHA1 = '0000000000000000000000000000000000000000';


describe('nog-sync', function () {
  describe('applySnapshotDiff()', function () {
    const euid = null;
    let peers = null;
    let syncOwner = null;
    let syncStore = null;
    let contentStore = null;
    let contentFaker = null;
    let remoteName = null;

    before(function () {
      peers = createTestPeers();
      const alice = peers.AliceMain;
      syncOwner = peers.aliceOwner;
      syncStore = alice.syncStore;
      contentStore = peers.aliceOpts.contentStore;
      remoteName = peers.rndBob;

      alice.ensureSyncUsers();
      alice.ensureMainSynchro(euid);

      contentFaker = createContentFaker();
      contentFaker.insertFakeUsers({ users: contentStore.users });
    });

    after(function () {
      peers.cleanup();
    });

    function getMaster() {
      const synchro = syncStore.synchros.findOne({ name: 'all' });
      return synchro.refs['branches/master'];
    }

    function setMaster(sha) {
      syncStore.synchros.update(
        { name: 'all' },
        { $set: { 'refs.branches/master': sha } },
      );
    }

    function setRemote(sha) {
      const refName = `remotes/${remoteName}/branches/master`;
      syncStore.synchros.update(
        { name: 'all' },
        { $set: { [`refs.${refName}`]: sha } },
      );
    }

    function snapshot() {
      syncStore.snapshot(euid, { ownerName: syncOwner, synchroName: 'all' });
      return getMaster();
    }

    function mergeAnonC({ ourSyn, theirSyn }) {
      setMaster(ourSyn);
      setRemote(theirSyn);
      const { commitSha } = mergeSynchroAnonC(
        euid,
        {
          syncStore,
          ownerName: syncOwner,
          synchroName: 'all',
          branch: 'master',
          remoteName,
        }
      );
      return commitSha;
    }

    // Suffix `Syn`: synchro commit sha.
    // Suffix `Con`: content commit sha.

    let repoName = null;

    let baseEmptySyn = null;
    let addedRepoSyn = null;
    let addedRepoCon = null;
    let addedRepoAltSyn = null;
    let addedRepoAltCon = null;

    let baseSyn = null;
    let baseCon = null;
    let modifiedSyn = null;
    let modifiedCon = null;
    let modifiedAltSyn = null;
    let modifiedAltCon = null;

    let modifiedMergeAnonConflictSyn = null;
    let resolvedSyn = null;
    let resolvedCon = null;

    it('prepare synchro commits', function () {
      contentStore.repos.remove({});
      baseEmptySyn = snapshot();

      contentFaker.createFakeContent({ euid, store: contentStore });
      repoName = contentFaker.spec.repo.name;
      addedRepoAltCon = contentFaker.spec.commit._id;
      addedRepoAltSyn = snapshot();

      setMaster(baseEmptySyn);
      contentFaker.amendFakeContent();
      addedRepoSyn = snapshot();
      addedRepoCon = contentFaker.spec.commit._id;

      baseSyn = addedRepoSyn;
      baseCon = addedRepoCon;

      contentFaker.commitFakeContent();
      modifiedAltSyn = snapshot();
      modifiedAltCon = contentFaker.spec.commit._id;

      setMaster(baseSyn);
      contentFaker.amendFakeContent();
      modifiedSyn = snapshot();
      modifiedCon = contentFaker.spec.commit._id;

      modifiedMergeAnonConflictSyn = mergeAnonC({
        ourSyn: modifiedSyn,
        theirSyn: modifiedAltSyn,
      });

      contentStore.repos.update(
        { name: repoName },
        {
          $set: {
            refs: { 'branches/master': modifiedCon },
            conflicts: {},
          },
        },
      );
      contentFaker.commitFakeContent();
      resolvedSyn = snapshot();
      resolvedCon = contentFaker.spec.commit._id;
    });

    function apply({ aSha, bSha }) {
      applySnapshotDiff(
        euid, { syncStore, remoteName, aCommitSha: aSha, bCommitSha: bSha }
      );
    }

    function applyAnonC({ aSha, bSha }) {
      applySnapshotDiffAnonC(
        euid, { syncStore, remoteName, aCommitSha: aSha, bCommitSha: bSha }
      );
    }

    it('adds repo', function () {
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': addedRepoCon,
      });
      expect(repo.conflicts).to.deep.eql({});
    });

    it('handles real local add repo conflict, anonC', function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      applyAnonC({ aSha: baseEmptySyn, bSha: addedRepoAltSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': addedRepoCon,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [addedRepoAltCon],
      });
    });

    it('handles trivial local add repo conflict', function () {
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': addedRepoCon,
      });
      expect(repo.conflicts).to.deep.eql({});
    });

    it('handles add repo for unknown user', function () {
      contentStore.users.remove({ _id: contentFaker.fakeUserDocs.owner._id });
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.ownerId).to.eql('unknown');

      contentFaker.insertFakeUsers({ users: contentStore.users });
    });

    it('deletes non-conflicting repo', function () {
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      apply({ aSha: addedRepoSyn, bSha: baseEmptySyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.not.exist;
    });

    it('stores a copy of deleted repos', function () {
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      const repoId = contentStore.repos.findOne({ name: repoName })._id;
      apply({ aSha: addedRepoSyn, bSha: baseEmptySyn });

      const repo = contentStore.deletedRepos.findOne({ _id: repoId });
      expect(repo).to.exist;
    });

    it('handles repoId conflict during double delete', function () {
      contentStore.repos.remove({});
      contentStore.deletedRepos.remove({});

      apply({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      const repo = contentStore.repos.findOne({ name: repoName });
      apply({ aSha: addedRepoSyn, bSha: baseEmptySyn });
      contentStore.repos.insert(repo);
      apply({ aSha: addedRepoSyn, bSha: baseEmptySyn });

      const n = contentStore.deletedRepos.find({ name: repoName }).count();
      expect(n).to.eql(2);
    });

    it('handles real local modified conflict during delete, anonC',
    function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      applyAnonC({ aSha: addedRepoAltSyn, bSha: baseEmptySyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': addedRepoCon,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [NULL_SHA1],
      });
    });

    it('applies modified', function () {
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: baseSyn });
      apply({ aSha: baseSyn, bSha: modifiedSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': modifiedCon,
      });
      expect(repo.conflicts).to.deep.eql({});
    });

    it('handles real local modified conflict, anonC', function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: baseSyn });
      applyAnonC({ aSha: baseSyn, bSha: modifiedSyn });
      applyAnonC({ aSha: baseSyn, bSha: modifiedAltSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': modifiedCon,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [modifiedAltCon],
      });
    });

    it('handles trivial local modified conflict', function () {
      contentStore.repos.remove({});

      apply({ aSha: baseEmptySyn, bSha: baseSyn });
      apply({ aSha: baseSyn, bSha: modifiedSyn });
      apply({ aSha: baseSyn, bSha: modifiedSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': modifiedCon,
      });
      expect(repo.conflicts).to.deep.eql({});
    });


    it('handles add repo that conflicts with local repo, ' +
    'local master match, anonC', function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: modifiedSyn });
      applyAnonC({ aSha: baseEmptySyn, bSha: modifiedMergeAnonConflictSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': modifiedCon,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [modifiedAltCon],
      });
    });

    it('handles add repo that conflicts with local repo, ' +
    'local master mismatch, anonC', function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: addedRepoSyn });
      applyAnonC({ aSha: baseEmptySyn, bSha: modifiedMergeAnonConflictSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': addedRepoCon,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [modifiedCon, modifiedAltCon].sort(),
      });
    });

    it('handled add repo with conflicting snapshot refs, anonC', function () {
      contentStore.repos.remove({});

      // It is not obvious that this can happen in practice.  If the repo does
      // not exists, who could have created a conflicting merge?  But it does
      // not harm to handle it.
      applyAnonC({ aSha: baseEmptySyn, bSha: modifiedMergeAnonConflictSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': NULL_SHA1,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [modifiedCon, modifiedAltCon].sort(),
      });
    });

    it('deletes repo with conflicts, anonC', function () {
      contentStore.repos.remove({});

      // Only the old master master is checked.  The repo is deleted even if
      // there are conflicts recorded in the repo.  The assumption is that a
      // conflict resolution would have modified master.
      applyAnonC({ aSha: baseEmptySyn, bSha: modifiedSyn });
      applyAnonC({ aSha: modifiedSyn, bSha: modifiedMergeAnonConflictSyn });
      applyAnonC({ aSha: modifiedSyn, bSha: baseEmptySyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.not.exist;
    });

    it('handles modified snapshot conflict, anonC', function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: baseSyn });
      applyAnonC({ aSha: baseSyn, bSha: modifiedMergeAnonConflictSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      // If snapshot contains conflicts, `branches/master` is left alone.
      expect(repo.refs).to.deep.eql({
        'branches/master': baseCon,
      });
      expect(repo.conflicts).to.deep.eql({
        'branches/master': [modifiedCon, modifiedAltCon].sort(),
      });
    });

    it('clears conflict on conflict resolution update, anonC', function () {
      contentStore.repos.remove({});

      applyAnonC({ aSha: baseEmptySyn, bSha: baseSyn });
      applyAnonC({ aSha: baseSyn, bSha: modifiedSyn });

      // Create and confirm conflict.
      applyAnonC({ aSha: modifiedSyn, bSha: modifiedMergeAnonConflictSyn });
      const repoConflict = contentStore.repos.findOne({ name: repoName });
      expect(repoConflict).to.exist;
      expect(repoConflict.refs).to.deep.eql({
        'branches/master': modifiedCon,
      });
      expect(repoConflict.conflicts).to.deep.eql({
        'branches/master': [modifiedAltCon],
      });

      // Resolve conflict.
      applyAnonC({ aSha: modifiedMergeAnonConflictSyn, bSha: resolvedSyn });

      const repo = contentStore.repos.findOne({ name: repoName });
      expect(repo).to.exist;
      expect(repo.refs).to.deep.eql({
        'branches/master': resolvedCon,
      });
      expect(repo.conflicts).to.deep.eql({});
    });
  });
});
