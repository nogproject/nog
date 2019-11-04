import crypto from 'crypto';
import { createAuthorizationCallCreds } from './grpc.js';
import { Writable } from 'stream';
import { _ } from 'meteor/underscore';
import { Meteor } from 'meteor/meteor';
import { Random } from 'meteor/random';
import { check, Match } from 'meteor/check';
import {
  AA_FSO_READ_REPO_TREE,
  CollNameContent,
  CollNameFiles,
  CollNameTreeErrors,
  KeyContent,
  KeyMeta,
  KeyRepoName,
  KeyStatInfo,
  KeyTreeErrorMessage,
  KeyTreePath,
  PubNameTree,
  PubNameTreePathContent,
} from './fso-tree.js';
import {
  KeyFsoId,
  KeyName,
  KeyRegistryId,
  makeCollName,
} from './collections.js';
import { makePubName } from './fso-pubsub.js';

const AA_FSO_READ_REPO = 'fso/read-repo';

function nameHashId(name) {
  const s = crypto.createHash('sha1').update(name, 'utf8').digest('base64');
  // Shorten and replace confusing characters.
  return s.substr(0, 20).replace(/[=+/]/g, 'x');
}

function stripTrailingSlash(s) {
  return s.replace(/\/$/, '');
}

// Tree uses client-only collections, see `fso-tree-client.js`.
function createCollections() {
  return {};
}

function publishFilesFunc({
  namespace, testAccess, registryConns, broadcast,
  registries, repos, rpcAuthorization,
}) {
  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );
  const filesCollName = makeCollName(namespace, CollNameFiles);
  const errCollName = makeCollName(namespace, CollNameTreeErrors);

  return function publishFiles(opts) {
    check(opts, { repoName: String });
    const { repoName } = opts;

    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    if (!testAccess(euid, AA_FSO_READ_REPO_TREE, { path: repoName })) {
      this.ready();
      return null;
    }

    const repoSel = { [KeyName]: repoName };
    const repoFields = {
      [KeyFsoId]: true,
      [KeyRegistryId]: true,
      [KeyName]: true,
    };

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

    const conn = connByRegistry.get(reg.name());
    if (!conn) {
      this.ready();
      return null;
    }

    const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
      expiresInS: 10 * 60,
      refreshPeriodS: 5 * 60,
      // The backend has no separate access action to control read tree.  It
      // uses `AA_FSO_READ_REPO`.
      scope: { action: AA_FSO_READ_REPO, path: repo.path() },
    });
    const rpcHead = conn.gitNogClient(callCreds);
    const rpcTree = conn.gitNogTreeClient(callCreds);

    const state = {
      statIds: new Set(),
      metaIds: new Set(),

      knowStatIds() { return new Set(this.statIds.values()); },
      knownMetaIds() { return new Set(this.metaIds.values()); },

      statId(path) {
        const id = nameHashId(`${repoName}/${path}`);
        let isNew = false;
        if (!this.statIds.has(id)) {
          this.statIds.add(id);
          if (!this.metaIds.has(id)) {
            isNew = true;
          }
        }
        return { id, isNew };
      },

      metaId(path) {
        const id = nameHashId(`${repoName}/${path}`);
        let isNew = false;
        if (!this.metaIds.has(id)) {
          this.metaIds.add(id);
          if (!this.statIds.has(id)) {
            isNew = true;
          }
        }
        return { id, isNew };
      },

      // `deleteIds()` removes the ids from the state and returns how to handle
      // the published docs:
      //
      //  - `rm`: remove doc.
      //  - `rmStat`: keep doc, but unset stat.
      //  - `rmMeta`: keep doc, but unset meta.
      //
      deleteIds({ unseenStat, unseenMeta }) {
        const rm = new Set();
        const rmStat = new Set();
        const rmMeta = new Set();

        for (const id of unseenStat) {
          this.statIds.delete(id);
          if (unseenMeta.has(id)) {
            rm.add(id);
          } else if (this.metaIds.has(id)) {
            rmStat.add(id);
          } else {
            rm.add(id);
          }
        }

        for (const id of unseenMeta) {
          this.metaIds.delete(id);
          if (unseenStat.has(id)) {
            rm.add(id);
          } else if (this.statIds.has(id)) {
            rmMeta.add(id);
          } else {
            rm.add(id);
          }
        }

        return { rm, rmStat, rmMeta };
      },
    };

    function strippedInfo(info) {
      const strip = ['path'];
      if (info.symlink === '') {
        strip.push('symlink');
      }
      if (info.gitlink.length === 0) {
        strip.push('gitlink');
      }
      return _.omit(info, ...strip);
    }

    // GRPC stores int64 as String.  Parse them as ints, since they are all
    // ECMAScript-int-safe.
    function parseInfoNumbers(info) {
      return {
        ...info,
        mtime: Number.parseInt(info.mtime || '0', 10),
        size: Number.parseInt(info.size || '0', 10),
        dirs: Number.parseInt(info.dirs || '0', 10),
        files: Number.parseInt(info.files || '0', 10),
        links: Number.parseInt(info.links || '0', 10),
        others: Number.parseInt(info.others || '0', 10),
      };
    }

    const addError = (err) => {
      const errId = Random.id();
      const msg = (
        `Failed to list files for repo \`${repoName}\`: ` +
        `${err.message}.`
      );
      this.added(errCollName, errId, {
        [KeyTreeErrorMessage]: msg,
      });
    };

    const createStreams = () => {
      try {
        const head = rpcHead.headSync({ repo: repo.fsoId() });

        const streams = {};
        streams.stat = rpcTree.listStatTree({
          repo: repo.fsoId(),
          statGitCommit: head.gitCommits.stat,
        });

        const metaGitCommit = head.gitCommits.meta;
        if (metaGitCommit.length > 0) {
          streams.meta = rpcTree.listMetaTree({
            repo: repo.fsoId(),
            metaGitCommit,
          });
        }

        return streams;
      } catch (err) {
        addError(err);
        return null;
      }
    };

    let havePendingPoll = false;

    const fetchOnce = ({ isFirstFetch }) => {
      havePendingPoll = false;

      const streams = createStreams();
      if (!streams) {
        if (isFirstFetch) {
          this.ready();
        }
        return;
      }

      // Track which previously known ids are still present.  Remove unseen
      // files when all streams ended successfully.
      const unseenStat = state.knowStatIds();
      const unseenMeta = state.knownMetaIds();

      const nExpected = _.size(streams);
      let nDone = 0;

      const doneError = Meteor.bindEnvironment((err) => {
        addError(err);

        nDone += 1;
        if (nDone < nExpected) {
          return;
        }

        if (isFirstFetch) {
          this.ready();
        }
      });

      const doneEnd = Meteor.bindEnvironment(() => {
        nDone += 1;
        if (nDone < nExpected) {
          return;
        }

        const { rm, rmStat, rmMeta } = state.deleteIds({
          unseenStat, unseenMeta,
        });
        for (const id of rmStat) {
          this.changed(filesCollName, id, {
            [KeyStatInfo]: undefined,
          });
        }
        for (const id of rmMeta) {
          this.changed(filesCollName, id, {
            [KeyMeta]: undefined,
          });
        }
        for (const id of rm) {
          this.removed(filesCollName, id);
        }

        if (isFirstFetch) {
          this.ready();
        }
      });

      // Pipe the streams to process messages one by one.  `on.data()` would be
      // called in parallel with messages as they arrive.
      const statCbStream = new Writable({
        objectMode: true,
        write: Meteor.bindEnvironment((rsp, enc, next) => {
          rsp.paths.forEach((info) => {
            const { path } = info;
            const { id, isNew } = state.statId(path);
            if (isNew) {
              this.added(filesCollName, id, {
                [KeyRepoName]: repoName,
                [KeyTreePath]: path,
                [KeyStatInfo]: parseInfoNumbers(strippedInfo(info)),
              });
            } else {
              this.changed(filesCollName, id, {
                [KeyStatInfo]: parseInfoNumbers(strippedInfo(info)),
              });
              unseenStat.delete(id);
            }
          });
          next();
        }),
      });
      streams.stat.on('error', (err) => {
        statCbStream.destroy();
        doneError(err);
      });
      statCbStream.on('finish', doneEnd);
      streams.stat.pipe(statCbStream);

      if (!streams.meta) {
        return;
      }

      const metaCbStream = new Writable({
        objectMode: true,
        write: Meteor.bindEnvironment((rsp, enc, next) => {
          rsp.paths.forEach((pmd) => {
            const path = stripTrailingSlash(pmd.path);

            let md;
            try {
              md = JSON.parse(pmd.metadataJson);
            } catch (err) {
              addError(err);
              return;
            }

            const { id, isNew } = state.metaId(path);
            if (isNew) {
              this.added(filesCollName, id, {
                [KeyRepoName]: repoName,
                [KeyTreePath]: path,
                [KeyMeta]: md,
              });
            } else {
              this.changed(filesCollName, id, {
                [KeyMeta]: md,
              });
              unseenMeta.delete(id);
            }
          });
          next();
        }),
      });
      streams.meta.on('error', (err) => {
        metaCbStream.destroy();
        doneError(err);
      });
      metaCbStream.on('finish', doneEnd);
      streams.meta.pipe(metaCbStream);
    };

    fetchOnce({ isFirstFetch: true });

    const bcSub = broadcast.subscribeGitRefUpdated(repo.fsoId(), () => {
      if (havePendingPoll) {
        return;
      }
      Meteor.defer(() => fetchOnce({ isFirstFetch: false }));
      havePendingPoll = true;
    });

    this.onStop(() => {
      broadcast.unsubscribe(bcSub);
    });

    return null;
  };
}

// `publishTreePathContent()` published the content of an FSO file once,
// without reactively updating it.
function publishTreePathContentFunc({
  namespace, testAccess, registryConns,
  registries, repos, rpcAuthorization,
}) {
  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );
  const contentCollName = makeCollName(namespace, CollNameContent);
  const errCollName = makeCollName(namespace, CollNameTreeErrors);

  return function publishTreePathContent(opts) {
    check(opts, {
      repoName: String,
      treePath: String,
    });
    const { repoName, treePath } = opts;

    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    if (!testAccess(euid, AA_FSO_READ_REPO_TREE, { path: repoName })) {
      this.ready();
      return null;
    }

    const addError = (err) => {
      const errId = Random.id();
      const msg = (
        `Failed to get content ` +
        `for repo \`${repoName}\` file \`${treePath}\`: ` +
        `${err.message}.`
      );
      this.added(errCollName, errId, {
        [KeyTreeErrorMessage]: msg,
      });
    };

    const repoSel = { [KeyName]: repoName };
    const repoFields = {
      [KeyFsoId]: true,
      [KeyRegistryId]: true,
      [KeyName]: true,
    };

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

    const conn = connByRegistry.get(reg.name());
    if (!conn) {
      this.ready();
      return null;
    }

    const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
      expiresInS: 10 * 60,
      refreshPeriodS: 5 * 60,
      // The backend has no separate access action to control read tree.  It
      // uses `AA_FSO_READ_REPO`.
      scope: { action: AA_FSO_READ_REPO, path: repo.path() },
    });
    const rpc = conn.gitNogClient(callCreds);
    let file;
    try {
      file = rpc.contentSync({
        repo: repo.fsoId(),
        path: treePath,
      });
    } catch (err) {
      addError(err);
      this.ready();
      return null;
    }

    // TODO some content-type heuristic.
    const text = file.content.toString('utf-8');

    const id = nameHashId(`${repoName}/${treePath}`);
    this.added(contentCollName, id, {
      [KeyRepoName]: repoName,
      [KeyTreePath]: treePath,
      [KeyContent]: text,
    });

    this.ready();
    return null;
  };
}

function registerPublications({
  publisher, namespace, testAccess, registryConns, broadcast,
  registries, repos, rpcAuthorization,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }

  defPub(PubNameTree, publishFilesFunc({
    namespace, testAccess, registryConns, broadcast,
    registries, repos, rpcAuthorization,
  }));

  defPub(PubNameTreePathContent, publishTreePathContentFunc({
    namespace, testAccess, registryConns,
    registries, repos, rpcAuthorization,
  }));
}

function createFsoTreeModuleServer({
  namespace, checkAccess, testAccess, publisher, registryConns, broadcast,
  registries, repos, rpcAuthorization,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(checkAccess, Function);
  check(publisher, Match.ObjectIncluding({ publish: Function }));
  check(registryConns, [{ registry: String, conn: Object }]);
  check(rpcAuthorization, Function);

  registerPublications({
    publisher, namespace, testAccess, registryConns, broadcast,
    registries, repos, rpcAuthorization,
  });

  const module = {
    ...createCollections({ namespace }),
  };
  return module;
}

export {
  createFsoTreeModuleServer,
};
