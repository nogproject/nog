import { check, Match } from 'meteor/check';
import { grpc, createAuthorizationCallCreds } from './grpc.js';
const {
  DEADLINE_EXCEEDED,
} = grpc.status;
import {
  ST_ENTER_SUBDIRS,
  ST_BUNDLE_SUBDIRS,
  ST_IGNORE_MOST,
  JC_WAIT,
} from './proto.js';
import {
  ERR_FSO,
  ERR_FSO_CLIENT,
  ERR_LOGIC,
  nogthrow,
} from './errors.js';
import {
  KeyFsoId,
  KeyGitlabHost,
  KeyId,
  KeyName,
  KeyRegistryId,
  KeyStatRequested,
  KeyRefreshContentRequested,
} from './collections.js';
import {
  matchExpiresIn,
  matchSan,
  matchScope,
} from './fso-jwt.js';

const AA_FSO_INIT_REPO = 'fso/init-repo';
const AA_FSO_REFRESH_REPO = 'fso/refresh-repo';
const AA_FSO_WRITE_REPO = 'fso/write-repo';
const AA_FSO_ISSUE_USER_TOKEN = 'fso/issue-user-token';
const AA_FSO_ISSUE_SYS_TOKEN = 'fso/issue-sys-token';

const TimeoutSeconds = {
  UpdateStat: 10,
  RefreshContent: 10,
  ReinitSubdirTracking: 10,
};

function log(msg, ...args) {
  console.log(`[fso] ${msg}`, ...args);
}

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

function afterSeconds(secs) {
  const d = new Date();
  d.setSeconds(d.getSeconds() + secs);
  return d;
}

const matchMeteorRandomId = Match.Where((x) => {
  check(x, String);
  if (!x.match(/^[a-zA-Z0-9]{10,30}$/)) {
    throw new Match.Error('malformed id');
  }
  return true;
});

const matchRegistryName = Match.Where((x) => {
  check(x, String);
  if (!x.match(/^[a-zA-Z0-9_-]+$/)) {
    throw new Match.Error('malformed registry name');
  }
  return true;
});

const matchGlobalPath = Match.Where((x) => {
  check(x, String);
  if (!x.startsWith('/')) {
    throw new Match.Error('malformed global path');
  }
  return true;
});

const matchGlobalRepoPath = Match.Where((x) => {
  check(x, matchGlobalPath);
  if (x.endsWith('/')) {
    throw new Match.Error('malformed global repo path');
  }
  return true;
});

const matchMeta = Match.Where((x) => {
  check(x, Object);
  for (const [k, v] of Object.entries(x)) {
    check(k, String);
    if (!k.match(/^[a-zA-Z0-9_]+$/)) {
      throw new Match.Error(`malformed key \`${k}\``);
    }
    check(v, Match.OneOf(
      String,
      [String],
    ));
  }
  return true;
});

const matchSubdirTracking = Match.Where((x) => {
  check(x, String);
  return (
    x === 'enter-subdirs' ||
    x === 'bundle-subdirs' ||
    x === 'ignore-most'
  );
});

function subdirTrackingAsPb(st) {
  return {
    'enter-subdirs': ST_ENTER_SUBDIRS,
    'bundle-subdirs': ST_BUNDLE_SUBDIRS,
    'ignore-most': ST_IGNORE_MOST,
  }[st];
}

// `createMethodsServer()` returns an object with server-side Meteor method
// implementations.  Each method must have a corresponding `defMethodCalls()`
// stub that forwards to the implementation, see `./fso-methods.js`.
function createMethodsServer({
  checkAccess, registries, repos, registryConns, gitNogConns, rpcAuthorization,
  rpcTokenProvider, catalogUpdater,
}) {
  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );

  const gitNogConnByRegistry = new Map(
    gitNogConns.map(({ registry, conn }) => [registry, conn]),
  );

  return {
    updateStat(euid, opts) {
      check(euid, Match.ObjectIncluding({
        username: String,
        emails: [Match.ObjectIncluding({ address: String })],
      }));
      check(opts, {
        repoId: matchMeteorRandomId,
        repoPath: matchGlobalRepoPath,
      });
      const { repoId, repoPath } = opts;
      const deadline = afterSeconds(TimeoutSeconds.UpdateStat);

      checkAccess(euid, AA_FSO_REFRESH_REPO, { path: repoPath });

      const repoFields = {
        [KeyId]: true,
        [KeyName]: true,
        [KeyFsoId]: true,
        [KeyRegistryId]: true,
      };
      const repo = repos.findOne(repoId, { fields: repoFields });
      if (!repo) {
        nogthrow(ERR_FSO, { reason: `Unknown repo \`${repoId}\`.` });
      }
      if (repo.path() !== repoPath) {
        nogthrow(ERR_FSO, {
          reason: `Repo id \`${repoId}\` path \`${repoPath}\` mismatch.`,
        });
      }
      const registryId = repo.registryId();

      const regFields = { [KeyName]: true };
      const reg = registries.findOne(registryId, { fields: regFields });
      if (!reg) {
        nogthrow(ERR_FSO, { reason: `Unknown registry \`${registryId}\`.` });
      }
      const regName = reg.name();

      const conn = connByRegistry.get(regName);
      if (!conn) {
        nogthrow(ERR_FSO, { reason: `No registry GRPC for \`${regName}\`.` });
      }

      // `euid` is the full `Meteor.user()`.
      if (euid.emails.length < 1) {
        nogthrow(ERR_FSO, { reason: 'Missing user email.' });
      }
      const email = euid.emails[0].address;

      try {
        const callCreds = createAuthorizationCallCreds(
          rpcAuthorization,
          euid,
          { scope: { action: AA_FSO_REFRESH_REPO, path: repo.path() } },
        );
        const statGrpc = conn.statClient(callCreds);
        statGrpc.statSync({
          jobControl: JC_WAIT,
          repo: repo.fsoId(),
          authorName: euid.username,
          authorEmail: email,
        }, {
          deadline,
        });
      } catch (err) {
        if (err.code === DEADLINE_EXCEEDED) {
          nogthrow(ERR_FSO_CLIENT, {
            reason: (
              'Refresh stat timeout.  ' +
              'The operation might complete in the background.'
            ),
            cause: err,
          });
        }
        nogthrow(ERR_FSO_CLIENT, {
          reason: 'Backend stat error.',
          cause: err,
        });
      }

      function updateStatTime() {
        const now = new Date();
        return repos.update({
          [KeyId]: repo.id(),
          $or: [
            { [KeyStatRequested]: { $lt: now } },
            { [KeyStatRequested]: { $exists: false } },
          ],
        }, {
          $set: { [KeyStatRequested]: now },
        });
      }

      try {
        updateStatTime();
      } catch (err) {
        // Don't send error to the client.  The UI will still displaying the
        // old value.  If the update fails, there is probably a more severe
        // problem, like Mongo is down.  Assume that the user will retry later.
        logerr(
          'Failed to store stat requested time.',
          'repo', repoId,
          'err', err,
        );
      }

      log(
        'Triggered repo stat.',
        'repo', repoId,
        'registry', regName,
      );
    },

    refreshContent(euid, opts) {
      check(euid, Match.ObjectIncluding({
        username: String,
        emails: [Match.ObjectIncluding({ address: String })],
      }));
      check(opts, {
        repoId: matchMeteorRandomId,
        repoPath: matchGlobalRepoPath,
      });
      const { repoId, repoPath } = opts;
      const deadline = afterSeconds(TimeoutSeconds.RefreshContent);

      checkAccess(euid, AA_FSO_REFRESH_REPO, { path: repoPath });

      const repoFields = {
        [KeyId]: true,
        [KeyName]: true,
        [KeyFsoId]: true,
        [KeyRegistryId]: true,
      };
      const repo = repos.findOne(repoId, { fields: repoFields });
      if (!repo) {
        nogthrow(ERR_FSO, { reason: `Unknown repo \`${repoId}\`.` });
      }
      if (repo.path() !== repoPath) {
        nogthrow(ERR_FSO, {
          reason: `Repo id \`${repoId}\` path \`${repoPath}\` mismatch.`,
        });
      }
      const registryId = repo.registryId();

      const regFields = { [KeyName]: true };
      const reg = registries.findOne(registryId, { fields: regFields });
      if (!reg) {
        nogthrow(ERR_FSO, { reason: `Unknown registry \`${registryId}\`.` });
      }
      const regName = reg.name();

      const conn = connByRegistry.get(regName);
      if (!conn) {
        nogthrow(ERR_FSO, { reason: `No registry GRPC for \`${regName}\`.` });
      }

      // `euid` is the full `Meteor.user()`.
      if (euid.emails.length < 1) {
        nogthrow(ERR_FSO, { reason: 'Missing user email.' });
      }
      const email = euid.emails[0].address;

      try {
        const callCreds = createAuthorizationCallCreds(
          rpcAuthorization,
          euid,
          { scope: { action: AA_FSO_REFRESH_REPO, path: repo.path() } },
        );
        const statGrpc = conn.statClient(callCreds);
        statGrpc.refreshContentSync({
          jobControl: JC_WAIT,
          repo: repo.fsoId(),
          authorName: euid.username,
          authorEmail: email,
        }, {
          deadline,
        });
      } catch (err) {
        if (err.code === DEADLINE_EXCEEDED) {
          nogthrow(ERR_FSO_CLIENT, {
            reason: (
              'Refresh content timeout.  ' +
              'The operation might complete in the background.'
            ),
            cause: err,
          });
        }
        nogthrow(ERR_FSO_CLIENT, {
          reason: 'Backend refresh content error.',
          cause: err,
        });
      }

      function updateContentTime() {
        const now = new Date();
        return repos.update({
          [KeyId]: repo.id(),
          $or: [
            { [KeyRefreshContentRequested]: { $lt: now } },
            { [KeyRefreshContentRequested]: { $exists: false } },
          ],
        }, {
          $set: { [KeyRefreshContentRequested]: now },
        });
      }

      try {
        updateContentTime();
      } catch (err) {
        // Don't send error to the client.  The UI will still displaying the
        // old value.  If the update fails, there is probably a more severe
        // problem, like Mongo is down.  Assume that the user will retry later.
        logerr(
          'Failed to store refresh content requested time.',
          'repo', repoId,
          'err', err,
        );
      }

      log(
        'Triggered repo refresh content.',
        'repo', repoId,
        'registry', regName,
      );
    },

    storeMeta(euid, opts) {
      check(euid, Match.ObjectIncluding({
        username: String,
        emails: [Match.ObjectIncluding({ address: String })],
      }));
      check(opts, {
        repoId: matchMeteorRandomId,
        repoPath: matchGlobalRepoPath,
        meta: matchMeta,
      });
      const { repoId, repoPath, meta } = opts;

      checkAccess(euid, AA_FSO_WRITE_REPO, { path: repoPath });

      const repoFields = {
        [KeyId]: true,
        [KeyName]: true,
        [KeyFsoId]: true,
        [KeyRegistryId]: true,
        [KeyGitlabHost]: true,
      };
      const repo = repos.findOne(repoId, { fields: repoFields });
      if (!repo) {
        nogthrow(ERR_FSO, { reason: `Unknown repo \`${repoId}\`.` });
      }
      if (repo.path() !== repoPath) {
        nogthrow(ERR_FSO, {
          reason: `Repo id \`${repoId}\` path \`${repoPath}\` mismatch.`,
        });
      }
      const registryId = repo.registryId();

      const regFields = { [KeyName]: true };
      const reg = registries.findOne(registryId, { fields: regFields });
      if (!reg) {
        nogthrow(ERR_FSO, { reason: `Unknown registry \`${registryId}\`.` });
      }
      const regName = reg.name();

      function findConn() {
        switch (repo.whichGitNogWrite()) {
          case 'regd':
            return connByRegistry.get(regName);
          case 'g2nd':
            return gitNogConnByRegistry.get(regName);
          default:
            nogthrow(ERR_LOGIC, { reason: 'Unexpected whichGitNog().' });
        }
        return null; // Keep lint happy.
      }

      const conn = findConn();
      if (!conn) {
        nogthrow(ERR_FSO, {
          reason: `No GitNog backend for repo \`${repoId}\`.`,
        });
      }

      // `euid` is the full `Meteor.user()`.
      if (euid.emails.length < 1) {
        nogthrow(ERR_FSO, { reason: 'Missing user email.' });
      }
      const email = euid.emails[0].address;

      const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        scope: { action: AA_FSO_WRITE_REPO, path: repo.path() },
      });
      const gitNogGrpc = conn.gitNogClient(callCreds);
      const res = gitNogGrpc.putMetaSync({
        repo: repo.fsoId(),
        metaJson: Buffer.from(JSON.stringify(meta)),
        authorName: euid.username,
        authorEmail: email,
        commitMessage: 'Meta update via UI',
      });

      return {
        gitNogCommitId: res.gitNogCommit.toString('hex'),
        metaCommitId: res.gitCommits.meta.toString('hex'),
        metaAuthor: res.metaAuthor,
        isNewCommit: res.isNewCommit,
      };
    },

    triggerUpdateCatalogs(euid, opts) {
      if (!catalogUpdater) {
        return;
      }

      check(opts, {
        repoId: matchMeteorRandomId,
        repoPath: matchGlobalRepoPath,
      });
      const { repoId, repoPath } = opts;

      // Require write permission, because updates are usually triggered by
      // writing metadata.  `triggerUpdateCatalogs()` is only a fallback for
      // lost metadata update events.
      checkAccess(euid, AA_FSO_WRITE_REPO, { path: repoPath });

      const repoFields = {
        [KeyId]: true,
        [KeyName]: true,
        [KeyFsoId]: true,
      };
      const repo = repos.findOne(repoId, { fields: repoFields });
      if (!repo) {
        nogthrow(ERR_FSO, { reason: `Unknown repo \`${repoId}\`.` });
      }
      if (repo.path() !== repoPath) {
        nogthrow(ERR_FSO, {
          reason: `Repo id \`${repoId}\` path \`${repoPath}\` mismatch.`,
        });
      }

      catalogUpdater.update({ repoFsoId: repo.fsoId() });
    },

    initRepo(euid, opts) {
      check(euid, Match.ObjectIncluding({
        username: String,
        emails: [Match.ObjectIncluding({ address: String })],
      }));
      check(opts, {
        registryName: matchRegistryName,
        globalPath: matchGlobalRepoPath,
      });
      const { registryName, globalPath } = opts;

      checkAccess(euid, AA_FSO_INIT_REPO, { path: globalPath, registryName });

      const conn = connByRegistry.get(registryName);
      if (!conn) {
        nogthrow(ERR_FSO_CLIENT, { reason: 'Unknown registry.' });
      }

      // `euid` is the full `Meteor.user()`.
      const { username } = euid;
      if (euid.emails.length < 1) {
        nogthrow(ERR_FSO, { reason: 'Missing user email.' });
      }
      const email = euid.emails[0].address;

      const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        scope: { action: AA_FSO_INIT_REPO, path: globalPath },
      });
      const regRpc = conn.registryClient(callCreds);
      try {
        regRpc.initRepoSync({
          registry: registryName,
          globalPath,
          creatorName: username,
          creatorEmail: email,
        });
      } catch (cause) {
        nogthrow(ERR_FSO_CLIENT, { reason: 'Init failed.', cause });
      }
    },

    reinitSubdirTracking(euid, opts) {
      check(euid, Match.ObjectIncluding({
        username: String,
        emails: [Match.ObjectIncluding({ address: String })],
      }));
      check(opts, {
        repoId: matchMeteorRandomId,
        repoPath: matchGlobalRepoPath,
        subdirTracking: matchSubdirTracking,
      });
      const { repoId, repoPath } = opts;
      const subdirTracking = subdirTrackingAsPb(opts.subdirTracking);
      const deadline = afterSeconds(TimeoutSeconds.ReinitSubdirTracking);

      checkAccess(euid, AA_FSO_INIT_REPO, { path: repoPath });

      const repoFields = {
        [KeyId]: true,
        [KeyName]: true,
        [KeyFsoId]: true,
        [KeyRegistryId]: true,
      };
      const repo = repos.findOne(repoId, { fields: repoFields });
      if (!repo) {
        nogthrow(ERR_FSO, { reason: `Unknown repo \`${repoId}\`.` });
      }
      if (repo.path() !== repoPath) {
        nogthrow(ERR_FSO, {
          reason: `Repo id \`${repoId}\` path \`${repoPath}\` mismatch.`,
        });
      }
      const registryId = repo.registryId();

      const regFields = { [KeyName]: true };
      const reg = registries.findOne(registryId, { fields: regFields });
      if (!reg) {
        nogthrow(ERR_FSO, { reason: `Unknown registry \`${registryId}\`.` });
      }
      const regName = reg.name();

      const conn = connByRegistry.get(regName);
      if (!conn) {
        nogthrow(ERR_FSO, { reason: `No registry GRPC for \`${regName}\`.` });
      }

      // `euid` is the full `Meteor.user()`.
      if (euid.emails.length < 1) {
        nogthrow(ERR_FSO, { reason: 'Missing user email.' });
      }
      const email = euid.emails[0].address;

      try {
        const callCreds = createAuthorizationCallCreds(
          rpcAuthorization,
          euid,
          { scope: { action: AA_FSO_INIT_REPO, path: repo.path() } },
        );
        const statGrpc = conn.statClient(callCreds);
        statGrpc.reinitSubdirTrackingSync({
          jobControl: JC_WAIT,
          repo: repo.fsoId(),
          authorName: euid.username,
          authorEmail: email,
          subdirTracking,
        }, {
          deadline,
        });
      } catch (err) {
        if (err.code === DEADLINE_EXCEEDED) {
          nogthrow(ERR_FSO_CLIENT, {
            reason: (
              'Reinit subdir tracking timeout.  ' +
              'The operation might complete in the background.'
            ),
            cause: err,
          });
        }
        nogthrow(ERR_FSO_CLIENT, {
          reason: 'Backend reinit subdir tracking error.',
          cause: err,
        });
      }

      // XXX Maybe automatically call updateStat().
    },

    issueUserToken(euid, opts) {
      check(opts, {
        expiresIn: matchExpiresIn,
        scope: Match.Optional(matchScope),
        scopes: Match.Optional([matchScope]),
      });
      checkAccess(euid, AA_FSO_ISSUE_USER_TOKEN, { path: '/' });
      return rpcTokenProvider.fsoToken(euid, opts);
    },

    issueSysToken(euid, opts) {
      check(opts, {
        subuser: String,
        expiresIn: matchExpiresIn,
        aud: Match.Optional([String]),
        san: Match.Optional(matchSan),
        scope: Match.Optional(matchScope),
        scopes: Match.Optional([matchScope]),
      });
      checkAccess(euid, AA_FSO_ISSUE_SYS_TOKEN, { path: '/' });
      return rpcTokenProvider.fsoSysToken(euid, opts);
    },
  };
}

export {
  createMethodsServer,
};
