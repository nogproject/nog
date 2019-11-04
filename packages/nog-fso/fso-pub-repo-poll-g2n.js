import { Writable } from 'stream';
import { Promise } from 'meteor/promise';
import { _ } from 'meteor/underscore';
import { grpc, createAuthorizationCallCreds } from './grpc.js';
import {
  PS_NEW,
  PS_MODIFIED,
  PS_DELETED,
} from './proto.js';
import { Meteor } from 'meteor/meteor';
import { check } from 'meteor/check';
import { createGitlabLocation } from './gitlab.js';
import {
  ERR_FSO,
  ERR_LOGIC,
  nogthrow,
} from './errors.js';
import {
  KeyErrorMessage,
  KeyFilesSummary,
  KeyFsoId,
  KeyGitNogCommit,
  KeyGitNogHost,
  KeyGitlabHost,
  KeyGitlabProjectId,
  KeyGitlabUrl,
  KeyId,
  KeyMeta,
  KeyMetadata,
  KeyName,
  KeyReadme,
  KeyRefreshContentRequested,
  KeyRegistryId,
  KeyStatRequested,
  KeyStatStatus,
} from './collections.js';

const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_REFRESH_REPO = 'fso/refresh-repo';

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

// Use the request time as a substitute for the status server time.
//
// XXX `nogfsostad` should perhaps send a status time in the response, so that
// updates can be conditioned on the status server time.
function statStatusSummary(rpc, { repo }) {
  const ts = new Date();
  const rpcStream = rpc.statStatus({ repo });
  return Promise.await(new Promise((resolve, reject) => {
    let nNew = 0;
    let nModified = 0;
    let nDeleted = 0;

    // Include a limited list of individual paths.
    const maxStatStatusChangesN = 50;
    const changes = [];
    let changesIsComplete = true;
    function addChange(c) {
      if (changes.length >= maxStatStatusChangesN) {
        changesIsComplete = false;
        return;
      }
      changes.push(c);
    }

    const cbStream = new Writable({
      objectMode: true,
      write: Meteor.bindEnvironment((rsp, enc, next) => {
        rsp.paths.forEach(({ path, status }) => {
          switch (status) {
            case PS_NEW:
              addChange({ path, status: '?' });
              nNew += 1;
              break;
            case PS_MODIFIED:
              addChange({ path, status: 'M' });
              nModified += 1;
              break;
            case PS_DELETED:
              addChange({ path, status: 'D' });
              nDeleted += 1;
              break;
            default:
              logerr('Unknown PathStatus.', 'status', status);
          }
        });
        next();
      }),
    });
    cbStream.on('finish', () => {
      resolve({
        nNew, nModified, nDeleted, changes, changesIsComplete, ts,
      });
    });
    rpcStream.on('error', (err) => {
      cbStream.destroy();
      reject(err);
    });
    rpcStream.pipe(cbStream);
  }));
}

// `publishRepoFuncPollGitNogCached()` returns a `publishRepo()` func that
// works indirectly via Mongo.  It polls GRPC `nogfso.Registry` and
// `nogfso.GitNog`, storing state in Mongo.  It returns a collection cursor to
// publish the state to the client.
//
// Active subscriptions poll for updates in the background and save them to
// Mongo.  Meteor sends updates via the Mongo cursor.  If backend services are
// unavailable, the publication sends the latest cached state.
function publishRepoFuncPollGitNogCached({
  testAccess, registries, repos, registryConns, gitNogConns, gitlabs,
  broadcast, rpcAuthorization,
}) {
  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );

  const gitNogConnByRegistry = new Map(
    gitNogConns.map(({ registry, conn }) => [registry, conn]),
  );

  const gitlabByName = new Map(
    gitlabs.map(cfg => [cfg.name, createGitlabLocation(cfg)]),
  );

  return function publishRepo(opts) {
    check(opts, { repoName: String });
    const { repoName } = opts;

    const accessPath = repoName;
    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    if (!testAccess(euid, AA_FSO_READ_REPO, { path: accessPath })) {
      this.ready();
      return null;
    }
    const yesStatStatus = (
      testAccess(euid, AA_FSO_REFRESH_REPO, { path: accessPath })
    );

    const repoSel = { [KeyName]: repoName };
    const repoFields = {
      [KeyErrorMessage]: true,
      [KeyFilesSummary]: true,
      [KeyFsoId]: true,
      [KeyGitNogCommit]: true,
      [KeyGitNogHost]: true,
      [KeyGitlabHost]: true,
      [KeyGitlabProjectId]: true,
      [KeyGitlabUrl]: true,
      [KeyId]: true,
      [KeyMeta]: true,
      [KeyMetadata]: true,
      [KeyName]: true,
      [KeyReadme]: true,
      [KeyRefreshContentRequested]: true,
      [KeyRegistryId]: true,
      [KeyStatRequested]: true,
    };
    if (yesStatStatus) {
      Object.assign(repoFields, {
        [KeyStatStatus]: true,
      });
    }

    const repo = repos.findOne(repoSel, { fields: repoFields });
    if (!repo) {
      this.ready();
      return null;
    }

    const regFields = { [KeyName]: true };
    const reg = registries.findOne(repo.registryId(), { fields: regFields });
    if (!reg) {
      this.ready();
      return null;
    }

    function createStatGrpc(conn) {
      if (!yesStatStatus) {
        return null;
      }
      const creds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        expiresInS: 10 * 60,
        refreshPeriodS: 5 * 60,
        scope: { action: AA_FSO_REFRESH_REPO, path: repo.path() },
      });
      return conn.statClient(creds);
    }

    function findGrpcs() {
      const conn = connByRegistry.get(reg.name());
      let gitNogConn = null;
      // DEPRECATED: `g2nd` is not used.  We always use `regd`.  See details at
      // `optGitNogRegdOnly` in `./collections-server.js`.
      switch (repo.whichGitNogRead()) {
        case 'regd':
          gitNogConn = conn;
          break;
        case 'g2nd':
          gitNogConn = gitNogConnByRegistry.get(reg.name());
          break;
        default:
          nogthrow(ERR_LOGIC, { reason: 'Unexpected whichGitNog().' });
      }
      if (!conn || !gitNogConn) {
        return {};
      }

      const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        expiresInS: 10 * 60,
        refreshPeriodS: 5 * 60,
        scope: { action: AA_FSO_READ_REPO, path: repo.path() },
      });

      const reposGrpc = conn.reposClient(callCreds);
      const gitNogGrpc = gitNogConn.gitNogClient(callCreds);
      return {
        reposGrpc,
        gitNogGrpc,
        statGrpc: createStatGrpc(conn),
      };
    }

    const { reposGrpc, gitNogGrpc, statGrpc } = findGrpcs();
    if (!reposGrpc || !gitNogGrpc) {
      this.ready();
      return null;
    }
    if (yesStatStatus && !statGrpc) {
      this.ready();
      return null;
    }

    // Use the stored Mongo doc directly.  There seems to be little value in
    // hiding lowlevel details.  The code below calls `repos.update()` and
    // needs to handle the details anyway.
    const state = repo.d;

    // Migrate `KeyMeta` to `KeyMetadata`: A full update will be triggered if
    // `KeyMeta` still exists.  The new key will be ensured in `state`; the old
    // one will be unset during update.
    let migrateKeyMeta = false;
    if (state[KeyMeta]) {
      migrateKeyMeta = true;
    }

    // Set some empty defaults to ensure `cacheIsGoodForReady()` if optional
    // fields are not in MongoDB.
    if (!_.has(state, KeyErrorMessage)) {
      state[KeyErrorMessage] = '';
    }
    if (!_.has(state, KeyMetadata)) {
      state[KeyMetadata] = {
        kvs: [],
        isUpdating: false,
      };
    }
    if (!repo.hasGitlabRepo()) {
      state[KeyGitlabProjectId] = '';
      state[KeyGitlabUrl] = '';
    }

    // FIXME `reposFields` needs to be revised.  `KeyGitNogHost` probably needs
    // to be deleted.  Maybe more.
    function cacheIsGoodForReady() {
      return Object.keys(state).length === Object.keys(repoFields).length;
    }

    let pendingUpdateStatStatus = false;
    function updateStatStatus() {
      if (pendingUpdateStatStatus) {
        return;
      }
      pendingUpdateStatStatus = true;

      try {
        const summary = statStatusSummary(statGrpc, { repo: repo.fsoId() });
        const newStatus = {
          [KeyStatStatus]: summary,
        };
        const nUp = repos.update({
          [KeyId]: repo.id(),
          // Condition on the status time.  Update only if newer.
          $or: [
            { [KeyStatStatus]: { $exists: false } },
            { [`${KeyStatStatus}.ts`]: { $lt: summary.ts } },
          ],
        }, {
          $set: newStatus,
        });
        if (nUp > 0) {
          Object.assign(state, newStatus);
        }
      } catch (err) {
        logerr(
          'Failed to get stat status.',
          'repoName', repoName,
          'err', err,
        );
      }

      pendingUpdateStatStatus = false;
    }

    const poll = ({ deferDetails = false }) => {
      const changes = {};

      if (repo.hasGitlabRepo()) {
        const rsp = reposGrpc.getRepoSync({ repo: repo.fsoId() });
        if (rsp.gitlabProjectId !== state[KeyGitlabProjectId]) {
          changes[KeyGitlabProjectId] = rsp.gitlabProjectId;
        }

        const [gitlabHost, gitlabPath] = rsp.gitlab.split(':', 2);
        const gitlab = gitlabByName.get(gitlabHost);
        if (!gitlab) {
          nogthrow(ERR_FSO, {
            reason: `Missing GitLab config for \`${gitlabHost}\`.`,
          });
        }

        if (state[KeyGitlabHost] !== gitlabHost) {
          changes[KeyGitlabHost] = gitlabHost;
        }

        const uiUrl = gitlab.projectUiUrl(gitlabPath);
        if (uiUrl !== state[KeyGitlabUrl]) {
          changes[KeyGitlabUrl] = uiUrl;
        }
      }

      const head = gitNogGrpc.headSync({ repo: repo.fsoId() });
      const gnCommit = {
        id: head.commitId.toString('hex'),
        statAuthorName: head.statAuthor.name,
        statAuthorEmail: head.statAuthor.email,
        statDate: new Date(head.statAuthor.date),
        shaAuthorName: head.shaAuthor.name,
        shaAuthorEmail: head.shaAuthor.email,
        shaDate: new Date(head.shaAuthor.date),
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
      };
      if (!_.isEqual(gnCommit, state[KeyGitNogCommit]) || migrateKeyMeta) {
        changes[KeyGitNogCommit] = gnCommit;
        changes[KeyFilesSummary] = { isUpdating: true };
        changes[KeyReadme] = { isUpdating: true, text: null };
        changes[KeyMetadata] = { isUpdating: true };
        migrateKeyMeta = false;
      }

      // At this point, the essential information has been received from the
      // registry and GitLab.  Clear potential repo error when storing to
      // Mongo.
      if (state[KeyErrorMessage]) {
        changes[KeyErrorMessage] = '';
      }

      if (Object.keys(changes).length > 0) {
        repos.update(repo.id(), { $set: changes });
        Object.assign(state, changes);
      }

      // Unconditionally `updateStatStatus()`, because the filesystem may
      // change at any time.  But update in the background, because it may be
      // relatively slow.
      if (yesStatStatus) {
        Meteor.defer(updateStatStatus);
      }

      function summaryIsUpToDate() {
        const summary = state[KeyFilesSummary];
        if (!summary || summary.isUpdating) {
          return false;
        }

        const readme = state[KeyReadme];
        if (!readme || readme.isUpdating) {
          return false;
        }

        const metadata = state[KeyMetadata];
        if (!metadata.kvs || metadata.isUpdating) {
          return false;
        }

        return true;
      }

      if (summaryIsUpToDate()) {
        return;
      }

      function updateFiles() {
        const summary = gitNogGrpc.summarySync({ repo: repo.fsoId() });
        summary.isUpdating = false;
        const newSummary = {
          [KeyFilesSummary]: {
            isUpdating: false,
            nFiles: summary.numFiles,
            nDirs: summary.numDirs,
            nOther: summary.numOther,
          },
        };
        repos.update({
          [KeyId]: repo.id(),
          // Condition on commit id to protect against concurrent updates.
          [`${KeyGitNogCommit}.id`]: gnCommit.id,
        }, {
          $set: newSummary,
        });
        Object.assign(state, newSummary);
      }

      function updateMeta() {
        const m = gitNogGrpc.metaSync({ repo: repo.fsoId() });
        const meta = JSON.parse(m.metaJson);
        const keys = Object.keys(meta);
        keys.sort();
        const newMeta = {
          [KeyMetadata]: {
            kvs: keys.map(k => ({ k, v: meta[k] })),
            isUpdating: false,
          },
        };
        repos.update({
          [KeyId]: repo.id(),
          // Condition on commit id to protect against concurrent updates.
          [`${KeyGitNogCommit}.id`]: gnCommit.id,
        }, {
          $set: newMeta,
          $unset: { [KeyMeta]: '' },
        });
        Object.assign(state, newMeta);
      }

      function updateReadme() {
        let text = null;
        try {
          const file = gitNogGrpc.contentSync({
            repo: repo.fsoId(),
            path: 'README.md',
          });
          text = file.content.toString('utf-8');
        } catch (err) {
          // Silently ignore NOT_FOUND.  Log other errors.
          if (err.code !== grpc.status.NOT_FOUND) {
            logerr(
              'Ignored gitNogGrpc.contentSync() error',
              'err', err.message,
            );
          }
        }
        const newReadme = {
          [KeyReadme]: { isUpdating: false, text },
        };
        repos.update({
          [KeyId]: repo.id(),
          // Condition on commit id to protect against concurrent updates.
          [`${KeyGitNogCommit}.id`]: gnCommit.id,
        }, {
          $set: newReadme,
        });
        Object.assign(state, newReadme);
      }

      function updateDetails() {
        updateFiles();
        updateMeta();
        updateReadme();
      }

      function updateDetailsLogErr() {
        try {
          updateDetails();
        } catch (err) {
          logerr(
            'Defer update files failed.',
            'repoName', repoName,
            'err', err,
          );
        }
      }

      if (deferDetails) {
        Meteor.defer(updateDetailsLogErr);
      } else {
        updateDetails();
      }
    };

    function setRepoError() {
      const msg = (
        'Information may be outdated due to problems with internal services.'
      );
      try {
        const errField = { [KeyErrorMessage]: msg };
        repos.update(repo.id(), { $set: errField });
        Object.assign(state, errField);
      } catch (err) {
        logerr(
          'Failed to store repo error.',
          'repoName', repoName,
          'err', err,
        );
      }
    }

    // Poll once before the subscription becomes ready.  Then poll on broadcast
    // events and with `tickInterval` as a fallback in case broadcast events
    // got lost.
    try {
      poll({ deferDetails: true });
    } catch (err) {
      setRepoError();
      if (!cacheIsGoodForReady()) {
        logerr(
          'Failed to poll during publish repo, insufficient cached state.',
          'repoName', repoName,
          'err', err,
          'stack', err.stack,
        );
        // `ready() + return null` is translated to 'unknown repo' in the UI.
        // Maybe better publish a placeholder document and display a warning in
        // the UI with the recommendation to reload later.
        this.ready();
        return null;
      }
      logerr(
        'Failed to poll during publish repo, using cached state.',
        'repoName', repoName,
        'err', err,
      );
    }

    const bcSub = broadcast.subscribeGitRefUpdated(repo.fsoId(), () => {
      Meteor.defer(() => {
        try {
          poll({ deferDetails: false });
        } catch (err) {
          setRepoError();
          logerr(
            'Failed to poll repo details update.',
            'repoName', repoName,
            'err', err,
          );
        }
      });
    });

    // This is only a fallback.
    const tickInterval = 30 * 1000;
    let tick = null;

    const nextPoll = () => {
      try {
        poll({ deferDetails: false });
      } catch (err) {
        setRepoError();
        logerr(
          'Failed to poll repo details update.',
          'repoName', repoName,
          'err', err,
        );
      }
      tick = Meteor.setTimeout(nextPoll, tickInterval);
    };

    tick = Meteor.setTimeout(nextPoll, tickInterval);
    this.onStop(() => {
      Meteor.clearTimeout(tick);
      broadcast.unsubscribe(bcSub);
    });

    return repos.find(repo.id(), { fields: repoFields });
  };
}

export {
  publishRepoFuncPollGitNogCached,
};

