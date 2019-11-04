import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import { Promise } from 'meteor/promise';

import { NogError } from 'meteor/nog-error';
const {
  nogthrow,
  createError,
} = NogError;

import {
  // RegistryEvent
  EV_FSO_REGISTRY_ADDED,
  EV_FSO_ROOT_ADDED,
  EV_FSO_ROOT_REMOVED,
  EV_FSO_ROOT_UPDATED,
  EV_FSO_REPO_ACCEPTED,
  EV_FSO_REPO_ADDED,
  EV_FSO_REPO_REINIT_ACCEPTED,
  EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED,
  EV_FSO_REPO_NAMING_UPDATED,
  EV_FSO_REPO_NAMING_CONFIG_UPDATED,
  EV_FSO_REPO_INIT_POLICY_UPDATED,
  EV_FSO_REPO_MOVE_ACCEPTED,
  EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED,
  EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED,

  // RepoEvent
  EV_FSO_REPO_INIT_STARTED,
  EV_FSO_SHADOW_REPO_CREATED,
  EV_FSO_GIT_REPO_CREATED,
  EV_FSO_GIT_TO_NOG_CLONED,
  EV_FSO_REPO_ERROR_SET,
  EV_FSO_REPO_ERROR_CLEARED,
  EV_FSO_ENABLE_GITLAB_ACCEPTED,
  EV_FSO_REPO_MOVE_STARTED,
  EV_FSO_REPO_MOVED,
  EV_FSO_ARCHIVE_RECIPIENTS_UPDATED,
  EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED,

  // WorkflowEvent
  EV_FSO_REPO_MOVE_STA_RELEASED,
  EV_FSO_REPO_MOVE_APP_ACCEPTED,
  EV_FSO_REPO_MOVE_COMMITTED,
} from './proto.js';

import {
  KeyFsoId,
  KeyGitlabHost,
  KeyGitlabPath,
  KeyGitlabProjectId,
  KeyName,
  KeyRegistryId,
  KeyVid,
} from './collections.js';

import {
  ERR_FSO,
} from './errors.js';

function log(msg, ...args) {
  console.log(`[fso] ${msg}`, ...args);
}

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

function isDuplicateMongoIdError(err) {
  return err.code === 11000;
}

function processGrpcEventStream(rpcStream, { onEvent, onWillBlock }) {
  const doRpc = (resolve, reject) => {
    let isClosed = false;

    const ctrl = {
      close(err) {
        if (isClosed) {
          const msg = 'Unexpected close(), GRPC stream was already stopped.';
          if (err) {
            logerr(msg, 'err', err.message);
          } else {
            logerr(msg);
          }
          return;
        }
        isClosed = true;
        rpcStream.cancel();
        if (err) {
          reject(err);
        } else {
          resolve();
        }
      },
    };

    const applyStream = new Writable({
      objectMode: true,
      write: Meteor.bindEnvironment((rsp, enc, next) => {
        if (isClosed) {
          log('Unexpected response to canceled rpc.');
          return;
        }

        for (const ev of rsp.events) {
          try {
            onEvent(ctrl, ev);
          } catch (err) {
            logerr(
              'Unexpected throw from onEvent().',
              'err', err,
            );
          }
          if (isClosed) {
            return;
          }
        }

        // Read events until stream would block.
        if (rsp.willBlock) {
          try {
            onWillBlock(ctrl);
          } catch (err) {
            logerr(
              'Unexpected throw from onWillBlock().',
              'err', err,
            );
          }
        }
        if (isClosed) {
          return;
        }
        next();
      }),
    });

    rpcStream.on('error', Meteor.bindEnvironment((err) => {
      applyStream.destroy();
      if (!isClosed) {
        ctrl.close(err);
      }
    }));

    rpcStream.pipe(applyStream);
  };

  return new Promise((resolve, reject) => {
    try {
      doRpc(resolve, reject);
    } catch (err) {
      reject(err);
    }
  });
}

function createProcessor({
  conn, sysCallCreds, repos, registryId,
}) {
  const reposGrpc = conn.reposClient(sysCallCreds);

  return {
    repos,
    reposGrpc,
    registryId,

    async applyEvent(ev) {
      switch (ev.event) {
        case EV_FSO_REPO_ADDED:
          await this.applyRepoAdded(ev);
          break;

        case EV_FSO_REPO_ENABLE_GITLAB_ACCEPTED:
          await this.applyEnableGitlab(ev);
          break;

        case EV_FSO_REPO_MOVE_ACCEPTED:
          await this.applyRepoMoveAccepted(ev);
          break;

        case EV_FSO_REPO_MOVED:
          await this.applyRepoMoved(ev);
          break;

        // Not interested.
        case EV_FSO_REGISTRY_ADDED: break;
        case EV_FSO_ROOT_ADDED: break;
        case EV_FSO_ROOT_REMOVED: break;
        case EV_FSO_ROOT_UPDATED: break;
        case EV_FSO_REPO_ACCEPTED: break;
        case EV_FSO_REPO_REINIT_ACCEPTED: break;
        case EV_FSO_REPO_NAMING_UPDATED: break;
        case EV_FSO_REPO_NAMING_CONFIG_UPDATED: break;
        case EV_FSO_REPO_INIT_POLICY_UPDATED: break;
        case EV_FSO_ROOT_ARCHIVE_RECIPIENTS_UPDATED: break;
        case EV_FSO_ROOT_SHADOW_BACKUP_RECIPIENTS_UPDATED: break;

        default:
          log(
            'Ignored unknown registry event.',
            'event', ev,
          );
      }
    },

    async applyRepoAdded(ev) {
      const repoFsoId = ev.fsoRepoInfo.id;
      const name = ev.fsoRepoInfo.globalPath;
      this.createRepo({ repoFsoId, name });
      log(
        'Applied fso registry event:',
        'event', ev,
      );
      await this.initRepo({ repoFsoId, waitGitlab: false });
    },

    async applyEnableGitlab(ev) {
      await this.initRepo({ repoFsoId: ev.repoId, waitGitlab: true });
    },

    createRepo({ repoFsoId, name }) {
      try {
        const id = this.repos.insert({
          [KeyFsoId]: repoFsoId,
          [KeyName]: name,
          [KeyRegistryId]: this.registryId,
        });
        log(
          'Inserted repo.',
          'name', name,
          'id', id,
        );
      } catch (err) {
        if (isDuplicateMongoIdError(err)) {
          return;
        }
        throw err;
      }
    },

    initRepo({ repoFsoId, waitGitlab }) {
      const doRpc = (resolve, reject) => {
        const repo = this.repos.findOne({ [KeyFsoId]: repoFsoId });
        if (!repo) {
          nogthrow(ERR_FSO, {
            reason: 'Fso repo Mongo doc missing.',
          });
        }

        const repoId = repo.id();
        let gitlabHost = repo.gitlabHost();
        let gitlabPath = repo.gitlabPath();
        let gitlabProjectId = repo.gitlabProjectId() || '';
        let vid = null;

        function isCompleteShadowOnlyConfig() {
          return gitlabHost === '';
        }

        function isCompleteGitlabConfig() {
          return gitlabHost && gitlabPath && gitlabProjectId;
        }

        function isInitInfoComplete() {
          if (waitGitlab) {
            return isCompleteGitlabConfig();
          }
          return isCompleteShadowOnlyConfig() || isCompleteGitlabConfig();
        }

        // Stop if init info is already stored.
        if (isInitInfoComplete()) {
          log(
            'Fso init repo already up-to-date.',
            'repoId', repoId,
            'repoFsoId', repoFsoId,
          );
          resolve();
          return;
        }

        const updateRepo = () => {
          this.repos.update(repoId, {
            $set: {
              [KeyVid]: vid,
              [KeyGitlabHost]: gitlabHost,
              [KeyGitlabPath]: gitlabPath,
              [KeyGitlabProjectId]: gitlabProjectId,
            },
          });
        };

        const rpcStream = reposGrpc.events({ repo: repoFsoId, watch: true });
        let isStopped = false;
        let repoError = null;
        const gracePeriodS = 10;
        let timeout = null;

        function stop() {
          isStopped = true;
          if (timeout) {
            Meteor.clearTimeout(timeout);
            timeout = null;
          }
          rpcStream.cancel();
        }

        function stopErr(err) {
          if (isStopped) {
            log(
              'Unexpected call to stopErr(), rpc was already stopped.',
              'err', err.message,
            );
            return;
          }
          stop();
          reject(err);
        }

        function stopOk() {
          if (isStopped) {
            log('Unexpected call to stopOk(), rpc was already stopped.');
            return;
          }
          stop();
          resolve();
        }

        function applyEvent(ev) {
          switch (ev.event) {
            case EV_FSO_REPO_INIT_STARTED:
              ({ gitlabHost, gitlabPath } = ev.fsoRepoInitInfo);
              break;
            case EV_FSO_ENABLE_GITLAB_ACCEPTED:
              ({ gitlabHost, gitlabPath } = ev.fsoRepoInitInfo);
              break;
            case EV_FSO_SHADOW_REPO_CREATED:
              break;  // Not interested.
            case EV_FSO_GIT_REPO_CREATED:
              ({ gitlabProjectId } = ev.fsoGitRepoInfo);
              break;
            case EV_FSO_GIT_TO_NOG_CLONED:
              break;  // Not interested.
            case EV_FSO_REPO_ERROR_SET:
              repoError = ev.fsoRepoErrorMessage;
              break;
            case EV_FSO_REPO_ERROR_CLEARED:
              repoError = null;
              break;
            case EV_FSO_ARCHIVE_RECIPIENTS_UPDATED:
              break;  // Not interested.
            case EV_FSO_SHADOW_BACKUP_RECIPIENTS_UPDATED:
              break;  // Not interested.
            default:
              log(
                'Ignored unknown repo event.',
                'event', ev,
              );
              break;
          }
          vid = ev.id;
        }

        const applyStream = new Writable({
          objectMode: true,
          write: Meteor.bindEnvironment((rsp, enc, next) => {
            if (isStopped) {
              log('Unexpected response to canceled rpc.');
              return;
            }

            for (const ev of rsp.events) {
              applyEvent(ev);
            }

            // Read all events until stream would block.
            if (!rsp.willBlock) {
              next();
              return;
            }

            // Stop if we have all we want.
            if (isInitInfoComplete()) {
              try {
                updateRepo();
                stopOk();
                log(
                  'Completed fso repo initialization.',
                  'repoId', repoId,
                );
              } catch (err) {
                stopErr(err);
              }
              return;
            }

            // Don't wait for more events if there is a stored repo error.
            if (repoError) {
              log(
                'Ignored repo with stored error.',
                'repoError', repoError,
              );
              stopOk();
              return;
            }

            // Otherwise, wait a bit.  Maybe the GitLab initialization
            // needs to complete.
            timeout = Meteor.setTimeout(() => {
              // We've observed spurious calls after `clearTimeout()`.
              if (isStopped) {
                return;
              }
              stopErr(createError(ERR_FSO, {
                reason: 'Repo init events missing after grace period.',
              }));
            }, gracePeriodS * 1000);
            next();
          }),
        });

        rpcStream.on('error', Meteor.bindEnvironment((err) => {
          applyStream.destroy();
          if (!isStopped) {
            stopErr(err);
          }
        }));

        rpcStream.pipe(applyStream);
      };

      return new Promise((resolve, reject) => {
        try {
          doRpc(resolve, reject);
        } catch (err) {
          reject(err);
        }
      });
    },

    async applyRepoMoveAccepted(ev) {
      const { workflowId: workflowFsoId } = ev;
      const { id: repoFsoId } = ev.fsoRepoInfo;
      const repo = this.repos.findOne({ [KeyFsoId]: repoFsoId });
      if (!repo) {
        nogthrow(ERR_FSO, {
          reason: 'Fso repo Mongo doc missing.',
        });
      }
      await this.processMoveRepoWorkflow({ repo, repoFsoId, workflowFsoId });
    },

    processMoveRepoWorkflow({ repo, repoFsoId, workflowFsoId }) {
      const repoId = repo.id();

      let hasStarted = false;
      let newName = '';
      let appHasAccepted = false;

      return processGrpcEventStream(reposGrpc.workflowEvents({
        repo: repoFsoId,
        workflow: workflowFsoId,
        watch: true,
      }), {
        onEvent(stream, ev) {
          switch (ev.event) {
            case EV_FSO_REPO_MOVE_STARTED:
              newName = ev.newFsoRepoInitInfo.globalPath;
              hasStarted = true;
              break;

            case EV_FSO_REPO_MOVE_STA_RELEASED:
              break;

            case EV_FSO_REPO_MOVE_APP_ACCEPTED:
              appHasAccepted = true;
              break;

            case EV_FSO_REPO_MOVED:
              break;

            case EV_FSO_REPO_MOVE_COMMITTED:
              break;

            default:
              log(
                'Ignored unknown WorkflowEvent.',
                'event', ev,
              );
              break;
          }
        },

        onWillBlock(stream) {
          if (!hasStarted) {
            return;
          }
          if (appHasAccepted) {
            stream.close();
            return;
          }

          log(
            'move-repo workflow should mark repo while moving.',
            'repoId', repoId,
          );

          try {
            reposGrpc.postMoveRepoAppAccepted({
              repo: repoFsoId,
              workflow: workflowFsoId,
            });
            log(
              'Acknowledged repo move.',
              'repoId', repoId,
              'newName', newName,
            );
            appHasAccepted = true;
          } catch (err) {
            logerr(
              'Failed to acknowledged repo move.',
              'repoId', repoId,
              'err', err,
            );
            stream.close(err);
          }
        },
      });
    },

    async applyRepoMoved(ev) {
      const repoFsoId = ev.fsoRepoInfo.id;
      const name = ev.fsoRepoInfo.globalPath;
      const repo = this.repos.findOne({ [KeyFsoId]: repoFsoId });
      if (!repo) {
        nogthrow(ERR_FSO, {
          reason: 'Fso repo Mongo doc missing.',
        });
      }
      const repoId = repo.id();

      const n = this.repos.update(repoId, {
        $set: {
          [KeyName]: name,
        },
      });
      if (n === 1) {
        log(
          'Updated moved repo.',
          'repoId', repoId,
          'newName', name,
        );
      } else {
        logerr(
          'Failed to update moved repo.',
          'repoId', repoId,
          'ev', ev,
        );
      }
    },
  };
}

export {
  createProcessor,
};
