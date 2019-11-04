import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import { Promise } from 'meteor/promise';
import { check, Match } from 'meteor/check';
import {
  nogthrow,
  ERR_FSO,
} from './errors.js';
import { createAuthorizationCallCreds } from './grpc.js';
import {
  KeyFsoId,
  KeyName,
  KeyRegistryId,
} from './collections.js';

import {
  PS_NEW,
  PS_MODIFIED,
  PS_DELETED,
} from './proto.js';

const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_READ_REPO_TREE = 'fso/read-repo-tree';
const AA_FSO_REFRESH_REPO = 'fso/refresh-repo';

const matchMongoCollection = Match.Where((x) => {
  if (!x) {
    return false;
  }
  check(x.find, Function);
  return true;
});

const matchRegistryConn = Match.Where((x) => {
  check(x, { registry: String, conn: Match.Any });
  check(x.conn.gitNogClient, Function);
  return true;
});

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

function createFsoIoModuleServer({
  registries, repos, checkAccess, registryConns, rpcAuthorization,
}) {
  check(registries, matchMongoCollection);
  check(repos, matchMongoCollection);
  check(checkAccess, Function);
  check(registryConns, [matchRegistryConn]);
  check(rpcAuthorization, Function);

  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );

  function resolveRepo(repoPath) {
    const repoSel = { [KeyName]: repoPath };
    const repoFields = {
      [KeyFsoId]: true,
      [KeyRegistryId]: true,
      [KeyName]: true,
    };
    const repo = repos.findOne(repoSel, { fields: repoFields });
    if (!repo) {
      return null;
    }

    const regFields = { [KeyName]: true };
    const reg = registries.findOne(repo.registryId(), { fields: regFields });
    if (!reg) {
      return null;
    }

    const conn = connByRegistry.get(reg.name());
    if (!conn) {
      return null;
    }

    return {
      conn,
      repoId: repo.id(),
      repoFsoId: repo.fsoId(),
    };
  }

  return {
    openRepo(euid, opts) {
      check(opts, {
        actions: [String],
        path: String,
      });
      const { actions, path: repoPath } = opts;

      for (const a of actions) {
        checkAccess(euid, a, { path: repoPath });
      }

      const details = resolveRepo(repoPath);
      if (!details) {
        nogthrow(ERR_FSO, { reason: 'repo not found' });
      }
      const { conn, repoFsoId } = details;

      const scope = { actions: [], path: repoPath };
      // The backend has no separate access action for read tree.  It uses
      // `AA_FSO_READ_REPO`.
      if (
        actions.includes(AA_FSO_READ_REPO) ||
        actions.includes(AA_FSO_READ_REPO_TREE)
      ) {
        scope.actions.push(AA_FSO_READ_REPO);
      }
      if (
        actions.includes(AA_FSO_REFRESH_REPO)
      ) {
        scope.actions.push(AA_FSO_REFRESH_REPO);
      }

      const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        expiresInS: 10 * 60,
        refreshPeriodS: 5 * 60,
        scope,
      });
      const rpcGitNog = conn.gitNogClient(callCreds);
      const rpcGitNogTree = conn.gitNogTreeClient(callCreds);
      const rpcTartt = conn.tarttClient(callCreds);
      const rpcStat = conn.statClient(callCreds);

      const fp = {
        repoId: details.repoId,
      };

      if (actions.includes(AA_FSO_READ_REPO)) {
        Object.assign(fp, {
          head() {
            return rpcGitNog.headSync({ repo: repoFsoId });
          },

          content({ path }) {
            return rpcGitNog.contentSync({ repo: repoFsoId, path });
          },

          meta() {
            return rpcGitNog.metaSync({ repo: repoFsoId });
          },

          summary() {
            return rpcGitNog.summarySync({ repo: repoFsoId });
          },

          tarttHead() {
            return rpcTartt.tarttHeadSync({ repo: repoFsoId });
          },

          listTars({ commit }) {
            return rpcTartt.listTarsSync({ repo: repoFsoId, commit });
          },
        });
      }

      if (actions.includes(AA_FSO_READ_REPO_TREE)) {
        Object.assign(fp, {
          listMetaTree({ metaGitCommit, onPathMeta }) {
            const rpcStream = rpcGitNogTree.listMetaTree({
              repo: repoFsoId, metaGitCommit,
            });
            return Promise.await(new Promise((resolve, reject) => {
              const cbStream = new Writable({
                objectMode: true,
                write: Meteor.bindEnvironment((rsp, enc, next) => {
                  try {
                    rsp.paths.forEach((pmd) => {
                      const { path, metadataJson } = pmd;
                      const meta = JSON.parse(metadataJson);
                      onPathMeta({ path, meta });
                    });
                  } catch (err) {
                    rpcStream.cancel();
                    reject(err);
                  }
                  next();
                }),
              });
              cbStream.on('finish', resolve);
              rpcStream.on('error', (err) => {
                cbStream.destroy();
                reject(err);
              });
              rpcStream.pipe(cbStream);
            }));
          },
        });
      }

      if (actions.includes(AA_FSO_REFRESH_REPO)) {
        Object.assign(fp, {
          // Use the request time as a substitute for the status server time.
          //
          // XXX `nogfsostad` should perhaps send a status time in the
          // response, so that updates can be conditioned on the status server
          // time.
          statStatusSummary() {
            const ts = new Date();
            const rpcStream = rpcStat.statStatus({ repo: repoFsoId });
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
          },
        });
      }

      return fp;
    },
  };
}

export {
  createFsoIoModuleServer,
};
