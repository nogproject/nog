import { Meteor } from 'meteor/meteor';
import { check } from 'meteor/check';

import {
  KeyErrorMessage,
  KeyFilesSummary,
  KeyFsoId,
  KeyGitNogCommit,
  KeyId,
  KeyMeta,
  KeyMetadata,
  KeyName,
  KeyReadme,
  KeyRefreshContentRequested,
  KeyStatStatus,
  KeyStatRequested,
} from './collections.js';

import { grpc } from './grpc.js';

const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_REFRESH_REPO = 'fso/refresh-repo';

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

function publishRepoFunc({
  testAccess, repos, broadcast, openRepo,
}) {
  return function publishRepo(opts) {
    check(opts, { repoName: String });
    const { repoName } = opts;

    // Rely on access check in `openRepo()`.
    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    const yesStatStatus = (
      testAccess(euid, AA_FSO_REFRESH_REPO, { path: repoName })
    );
    const a = [AA_FSO_READ_REPO];
    if (yesStatStatus) {
      a.push(AA_FSO_REFRESH_REPO);
    }
    let repoConn;
    try {
      repoConn = openRepo(euid, {
        actions: a,
        path: repoName,
      });
    } catch (err) {
      logerr('Failed to publish repo.', 'path', repoName, 'err', err);
      this.ready();
      return null;
    }

    const repoFields = {
      [KeyErrorMessage]: true,
      [KeyFsoId]: true,
      [KeyFilesSummary]: true,
      [KeyGitNogCommit]: true,
      [KeyId]: true,
      [KeyMeta]: true,
      [KeyMetadata]: true,
      [KeyName]: true,
      [KeyReadme]: true,
      [KeyRefreshContentRequested]: true,
      [KeyStatRequested]: true,
    };
    if (yesStatStatus) {
      Object.assign(repoFields, {
        [KeyStatStatus]: true,
      });
    }
    const repoSel = { [KeyName]: repoName };
    const repo = repos.findOne(repoSel, { fields: repoFields });
    if (!repo) {
      this.ready();
      return null;
    }

    // Migrate `KeyMeta` to `KeyMetadata`: A full update will be triggered if
    // `KeyMeta` still exists.  The new key will be added during updateMeta();
    // the old one will be unset as deprecated field.
    let migrateKeyMeta = false;
    // Check and delete deprecated fields once.
    const depFields = {};
    if (repo.hasMeta()) {
      migrateKeyMeta = true;
      Object.assign(depFields, { [KeyMeta]: '' });
    }
    if (repo.hasFilesSummaryIsUpdating()) {
      Object.assign(depFields, { [`${KeyFilesSummary}.isUpdating`]: '' });
    }
    if (repo.hasReadmeIsUpdating) {
      Object.assign(depFields, { [`${KeyReadme}.isUpdating`]: '' });
    }
    if (Object.keys(depFields).length > 0) {
      repos.update(repo.id(), { $unset: depFields });
    }

    function updateStatStatus() {
      try {
        const summary = repoConn.statStatusSummary();
        const newStatus = {
          [KeyStatStatus]: summary,
        };
        repos.update({
          [KeyId]: repo.id(),
          // Condition on the status time.  Update only if newer.
          $or: [
            { [`${KeyStatStatus}.ts`]: { $exists: false } },
            { [`${KeyStatStatus}.ts`]: { $lt: summary.ts } },
          ],
        }, {
          $set: newStatus,
        });
      } catch (err) {
        logerr(
          'Failed to get stat status.',
          'repoName', repoName,
          'err', err,
        );
      }
    }

    function updateRepoInfo() {
      const head = repoConn.head();
      const gnCommit = {
        id: head.commitId.toString('hex'),
        statAuthorName: head.statAuthor.name,
        statAuthorEmail: head.statAuthor.email,
        statDate: new Date(head.statAuthor.date),
        statCommitId: head.gitCommits.stat.toString('hex'),
        shaAuthorName: head.shaAuthor.name,
        shaAuthorEmail: head.shaAuthor.email,
        shaDate: new Date(head.shaAuthor.date),
        shaCommitId: head.gitCommits.sha.toString('hex'),
        metaAuthorName: head.metaAuthor ? head.metaAuthor.name : null,
        metaAuthorEmail: head.metaAuthor ? head.metaAuthor.email : null,
        metaDate: head.metaAuthor ? new Date(head.metaAuthor.date) : null,
        metaCommitId: (
          head.gitCommits.meta ? head.gitCommits.meta.toString('hex') : null
        ),
        contentAuthorName: (
          head.contentAuthor ? head.contentAuthor.name : null
        ),
        contentAuthorEmail: (
          head.contentAuthor ? head.contentAuthor.email : null
        ),
        contentDate: (
          head.contentAuthor ? new Date(head.contentAuthor.date) : null
        ),
        contentCommitId: (
          head.gitCommits.content ?
            head.gitCommits.content.toString('hex') : null
        ),
      };

      const repoState = repos.findOne(repo.id());
      // `repoGncoId` will be used as condition when updating the doc.
      // Initially, the doc has no `gitNogCommitId`.  It is set to null in that
      // case.  The item selector then requires that either the ID matches or
      // the field does not exist.
      let repoGncoId = null;
      if (repoState.gitNogCommit()) {
        repoGncoId = repoState.gitNogCommitId();
      }
      const sel = {
        [KeyId]: repo.id(),
        [`${KeyGitNogCommit}.id`]: repoGncoId,
      };

      function updateMeta() {
        if (
          repoState.gitNogCommit() &&
          repoState.metaCommitId() === gnCommit.metaCommitId &&
          !migrateKeyMeta
        ) {
          return;
        }

        // The update state is currently used in MetadataForm to block user
        // interaction.
        repos.update(sel, { $set: { [`${KeyMetadata}.isUpdating`]: true } });

        const m = repoConn.meta();
        const meta = JSON.parse(m.metaJson);
        const keys = Object.keys(meta);
        keys.sort();
        const newMeta = {
          [KeyMetadata]: {
            kvs: keys.map(k => ({ k, v: meta[k] })),
            isUpdating: false,
          },
        };
        repos.update(sel, { $set: newMeta });
      }

      function updateSummary() {
        if (
          repoState.gitNogCommit() &&
          repoState.statCommitId() === gnCommit.statCommitId
        ) {
          return;
        }
        const summary = repoConn.summary();
        const newSummary = {
          [KeyFilesSummary]: {
            nFiles: summary.numFiles,
            nDirs: summary.numDirs,
            nOther: summary.numOther,
          },
        };
        repos.update(sel, { $set: newSummary });
      }

      function updateReadme() {
        if (
          repoState.gitNogCommit() &&
          repoState.contentCommitId() === gnCommit.contentCommitId
        ) {
          return;
        }
        let text = null;
        try {
          const file = repoConn.content({ path: 'README.md' });
          text = file.content.toString('utf-8');
        } catch (err) {
          // Silently ignore NOT_FOUND.  Log other errors.
          if (err.code !== grpc.status.NOT_FOUND) {
            logerr(
              `Ignored Grpc content update error for repo: ${repo.id()}`,
              'err', err.message,
            );
          }
        }
        const newReadme = {
          [KeyReadme]: { text },
        };
        repos.update(sel, { $set: newReadme });
      }

      // `KeyGitNogCommit` incl. the commit ID is updated after the other
      // update function have completed.
      // The update functions only update if the git commit Ids of FSO head and
      // Mongo differ, while `KeygitNogCommit` data will always be updated.
      // Thus, a poll may fetch content or meta beyond the processed
      // `gitNogCommitId`, which then will be adjusted during the next run.
      updateMeta();
      updateReadme();
      updateSummary();

      repos.update(sel, {
        $set: {
          [KeyGitNogCommit]: gnCommit,
          [KeyErrorMessage]: '',
        },
      });
    }

    function poll() {
      updateRepoInfo();
      if (yesStatStatus) {
        updateStatStatus();
      }
    }

    function setRepoError() {
      const msg = (
        'Information may be outdated due to problems with internal services.'
      );
      try {
        const errField = { [KeyErrorMessage]: msg };
        repos.update(repo.id(), { $set: errField });
      } catch (err) {
        logerr(
          'Failed to store repo error.',
          'repoName', repoName,
          'err', err,
        );
      }
    }

    // Background polling:
    //
    // `tickInterval` is the period from the end of one poll until the next
    // poll.
    //
    // `tick` is the active timeout.  It is `null` if there is no active
    // timeout.
    //
    // `isStopped` ensures that polling stops even if `onStop()` happens while
    // `poll()` has not yet returned.
    //
    const tickInterval = 30 * 1000;
    let tick = null;
    let isStopped = false;

    const nextPoll = () => {
      try {
        poll();
      } catch (err) {
        setRepoError();
        logerr(
          'Failed to poll repo details update.',
          'repoName', repoName,
          'err', err,
        );
      }
      if (isStopped) {
        return;
      }
      tick = Meteor.setTimeout(nextPoll, tickInterval);
    };

    const bcSub = broadcast.subscribeGitRefUpdated(repo.fsoId(), () => {
      if (tick) {
        Meteor.clearTimeout(tick);
        tick = null;
      }
      Meteor.defer(nextPoll);
    });
    Meteor.defer(nextPoll);

    this.onStop(() => {
      if (tick) {
        Meteor.clearTimeout(tick);
      }
      broadcast.unsubscribe(bcSub);
      isStopped = true;
    });

    return repos.find(repo.id(), { fields: repoFields });
  };
}

export {
  publishRepoFunc,
};

