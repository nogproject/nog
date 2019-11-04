import crypto from 'crypto';
import filepath from 'path';
import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import { Promise } from 'meteor/promise';
import { _ } from 'meteor/underscore';
import { check, Match } from 'meteor/check';
import {
  ERR_FSO_CATALOG,
  ERR_NOT_OF_KIND,
  nogthrow,
} from './errors.js';
import {
  matchCatalogConfig,
} from 'meteor/nog-catalog';
import {
  KeyFsoId,
  KeyId,
  KeyName,
  KeyRegistryId,
  createAuthorizationCallCreds,
} from 'meteor/nog-fso';

// XXX Maybe import from `nog-fso`.
const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_UPDATE_CATALOG = 'fso/update-catalog';

const matchMongoCollection = Match.Where((x) => {
  check(x.find, Function);
  return true;
});

const matchRegistryConn = Match.Where((x) => {
  check(x, { registry: String, conn: Match.Any });
  check(x.conn.gitNogClient, Function);
  return true;
});

function mongoRepoSelector(repoSelector, options) {
  const { $fso } = repoSelector;
  const sel = {};
  for (const [k, v] of Object.entries($fso)) {
    switch (k) {
      case 'path':
        sel[KeyName] = v;
        break;
      default:
        nogthrow(ERR_FSO_CATALOG, {
          reason: 'Unknown $fso.repoSelector key.',
        });
    }
  }

  const { fsoSelectRepos } = options;
  if (fsoSelectRepos) {
    sel[KeyFsoId] = { $in: fsoSelectRepos };
  }

  return sel;
}

function sha1Hex(d) {
  return crypto.createHash('sha1').update(d, 'utf8').digest('hex');
}

function isDuplicateMongoIdError(err) {
  return err.code === 11000;
}

function listMetaTree(rpc, { repo, metaGitCommit }, perRpcOpts, onPathMeta) {
  const rpcStream = rpc.listMetaTree({ repo, metaGitCommit }, perRpcOpts);
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
}

function ensureTrailingSlash(s) {
  if (s.endsWith('/')) {
    return s;
  }
  return `${s}/`;
}

// `pathTailN(path, n)` returns a path for the last `n` parts of the path.  It
// preserves a trailing slash.  Examples:
//
//  - `/a/b/c/d`, 3 -> 'b/c/d`.
//  - `/a/b/c/d/`, 3 -> 'b/c/d/`.
//
function pathTailN(path, n) {
  const norm = filepath.normalize(path);
  const tail = norm.endsWith('/') ? n + 1 : n;
  return norm.split('/').slice(-tail).join('/');
}

// `addFsoRepoTreeSudo()` adds the repo tree based on GRPC service
// `GitNogTree.ListMetaTree()` for `gitCommitId`.
function addFsoRepoTreeSudo({
  rpc, perRpcOpts, repo, head,
  volume, fieldManager, truncatedMetaVal,
  pipelineFuncs,
  logger, rateLimiters,
}) {
  const repoId = repo.id();
  const repoPath = ensureTrailingSlash(repo.path());
  let repoRootIsAdded = false;

  const addPathMeta = (pmd) => {
    const { path, meta } = pmd;

    // `content` mimics the structure of traditional Nog entries.
    let content = { meta };
    if (path === '.') {
      repoRootIsAdded = true;
      content.path = repoPath;
      content.name = pathTailN(repoPath, 3);
    } else {
      content.path = filepath.join(repoPath, path);
      content.name = filepath.basename(path);
    }
    for (const fn of pipelineFuncs) {
      content = fn(content);
      if (!content) {
        return;
      }
    }

    // `contentId` must be a string, because it is used as a MongoDB id.
    // `contentId` must also be a content address, so that `content` can be
    // immutable.  Since we want the path to be part of `content`, but the path
    // is not determined by the commit, we mangle path into a content hash, so
    // that different paths yield different hashes.
    const contentId = sha1Hex([
      'fso',
      repoPath,
      head.commitId.toString('hex'),
      path,
    ].join('\n'));

    const fixed = {
      _id: contentId,
      name: content.name,
      m: {},
    };

    // XXX Copied from `nog-catalog-main-server.js`.  Maybe refactor.
    for (const [k, v] of _.pairs(content.meta)) {
      // XXX We silently ignore null and objects.  Is this good enough?
      if (Match.test(v, Match.OneOf(String, Number, [String], [Number]))) {
        const { val, isTruncated } = truncatedMetaVal(v);
        if (isTruncated) {
          logger.log(
            '[nog-catalog-fso] Truncated meta value ' +
            `to satisfy max string length constraint: ` +
            `FSO repo ${path}.meta.${k}`,
          );
        }
        const field = fieldManager.getMongoField(k);
        // Cast to string, since search uses regex prefixes, which does not
        // work with numbers.
        //
        // Force array, so that `$unwind` in MongoDB pipelines works correctly
        // with MongoDB < 3.2.  This could be removed when we use MongoDB >=
        // 3.2 everywhere.  See `publishCatalogVolumeStats()`.
        if (_.isArray(val)) {
          fixed.m[field] = val.map(v => String(v));
        } else {
          fixed.m[field] = [String(val)];
        }
      }
    }

    const refpath = {
      type: 'fso',
      repoId,
      commitId: head.commitId,
      repoPath,
      treePath: path,
    };
    fixed.refpaths = [refpath];

    rateLimiters.writeLimiter.op();
    try {
      volume.insert(fixed);
    } catch (err) {
      if (!isDuplicateMongoIdError(err)) {
        throw err;
      }
    }
    volume.update(content._id, { $addToSet: { refpaths: refpath } });
  };

  // List tree if there is a meta commit.
  if (head.gitCommits.meta.length) {
    const listMetaTreeI = {
      repo: repo.fsoId(),
      metaGitCommit: head.gitCommits.meta,
    };
    listMetaTree(rpc, listMetaTreeI, perRpcOpts, addPathMeta);
  }
  // Add the repo root even if it has no path metadata.
  if (!repoRootIsAdded) {
    addPathMeta({ path: '.', meta: {} });
  }
}

function createFsoCatalogPlugin({
  registries, repos,
  checkAccess, testAccess,
  registryConns,
  rpcAuthorization,
}) {
  check(registries, matchMongoCollection);
  check(repos, matchMongoCollection);
  check(checkAccess, Function);
  check(testAccess, Function);
  check(registryConns, [matchRegistryConn]);
  check(rpcAuthorization, Function);

  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );

  return {
    name: 'fso',
    version: 3,  // Bump to force catalog rebuild after implementation changes.
    repoSelectorKey: '$fso',

    addContent(euid, {
      catalogs, catalogId, stage,
      volume, fieldManager, truncatedMetaVal,
      repoSelector, pipelineFuncs,
      logger, renewLock, rateLimiters,
      options,
    }) {
      // `gitNogRpcs` caches GRPC clients for the duration of the
      // `addContent()` call.
      const gitNogRpcs = new Map();

      function getGitNogRpc(reg) {
        const rpc = gitNogRpcs.get(reg.name());
        if (rpc) {
          return rpc;
        }

        const conn = connByRegistry.get(reg.name());
        if (!conn) {
          nogthrow(ERR_FSO_CATALOG, {
            reason: `Missing GitNog GRPC for registry \`${reg.name()}\`.`,
          });
        }
        const newRpc = conn.gitNogClient();
        gitNogRpcs.set(reg.name(), newRpc);
        return newRpc;
      }

      // `gitNogTreeRpcs` caches GRPC clients for the duration of the
      // `addContent()` call.
      const gitNogTreeRpcs = new Map();

      function getGitNogTreeRpc(reg) {
        const rpc = gitNogTreeRpcs.get(reg.name());
        if (rpc) {
          return rpc;
        }

        const conn = connByRegistry.get(reg.name());
        if (!conn) {
          nogthrow(ERR_FSO_CATALOG, {
            reason: `Missing GitNog GRPC for registry \`${reg.name()}\`.`,
          });
        }
        const newRpc = conn.gitNogTreeClient();
        gitNogTreeRpcs.set(reg.name(), newRpc);
        return newRpc;
      }

      const repoSel = mongoRepoSelector(repoSelector, options);
      repos.find(repoSel).forEach((repo) => {
        renewLock();

        const path = repo.path();
        if (!testAccess(euid, AA_FSO_READ_REPO, { path })) {
          logger.log(
            `[nog-catalog-fso] Ignored FSO repo \`${path}\` ` +
            `due to lacking \`${AA_FSO_READ_REPO}\` access.`,
          );
          return;
        }

        const regFields = { [KeyName]: true };
        const reg = registries.findOne(repo.registryId(), {
          fields: regFields,
        });
        if (!reg) {
          nogthrow(ERR_FSO_CATALOG, {
            reason: `Failed to find registry for FSO repo \`${repo.id()}\`.`,
          });
        }

        const perRpcOpts = {
          deadline: Date.now() + 30 * 1000, // 30s.
          credentials: createAuthorizationCallCreds(
            rpcAuthorization,
            euid,
            { scope: { action: AA_FSO_READ_REPO, path } },
          ),
        };

        const rpc = getGitNogRpc(reg);
        let head;
        try {
          head = rpc.headSync({ repo: repo.fsoId() }, perRpcOpts);
        } catch (err) {
          logger.log(
            '[nog-catalog-fso] Ignored head() failure. ' +
            `repoId=${repo.id()} ` +
            `err=${err}`,
          );
          return;
        }

        const repoId = repo.id();

        // XXX Maybe refactor to share up-to-date logic with nog-catalog.  Or
        // completely drop up-to-date logic and support only rebuilding from
        // scratch.  Storing an array of `{ repoId, commit }` fundamentally
        // limits the number of repos per catalog.  It is also unclear whether
        // Mongo remains reasonably fast if the array gets large.
        //
        // We should change this at some point to an scalable implementation
        // that supports an unbounded number of repos.  We should keep
        // incremental updates.  They seem useful, in particular since we
        // introduced the incremental `catalogUpdater`.  We should probably
        // reimplement the `${stage}.repos` state as a Mongo collection, so
        // that scalability is not limited by the document size nor linear
        // array searches.
        const selCatUpToDate = {
          _id: catalogId,
          [`${stage}.repos`]: { repoId, commit: head.commitId },
        };
        if (catalogs.findOne(selCatUpToDate)) {
          return;  // Already up-to-date.
        }

        // First add refpaths.  Then remove refpaths for old commits and
        // finally docs with empty refpaths, because they became useless.
        addFsoRepoTreeSudo({
          rpc: getGitNogTreeRpc(reg),
          perRpcOpts,
          repo, head,
          volume, fieldManager, truncatedMetaVal,
          pipelineFuncs,
          logger, rateLimiters,
        });

        volume.update(
          { 'refpaths.repoId': repoId },
          {
            $pull: { refpaths: { repoId, commitId: { $ne: head.commitId } } },
          },
          { multi: true },
        );
        volume.remove({ refpaths: [] });

        catalogs.update(catalogId, {
          $addToSet: {
            [`${stage}.repos`]: { repoId, commit: head.commitId },
          },
        });
        catalogs.update(catalogId, {
          $pull: {
            [`${stage}.repos`]: { repoId, commit: { $ne: head.commitId } },
          },
        });
      });
    },

    getCatalogConfig(euid, opts) {
      check(opts, {
        repoPath: String,
      });
      const { repoPath } = opts;

      checkAccess(euid, AA_FSO_UPDATE_CATALOG, { path: repoPath });

      const repoSel = {
        [KeyName]: repoPath,
      };
      const repoFields = {
        [KeyId]: true,
        [KeyName]: true,
        [KeyFsoId]: true,
        [KeyRegistryId]: true,
      };
      const repo = repos.findOne(repoSel, { fields: repoFields });
      if (!repo) {
        nogthrow(ERR_FSO_CATALOG, {
          reason: `Unknown repo \`${repoPath}\`.`,
        });
      }

      const regFields = { [KeyName]: true };
      const reg = registries.findOne(repo.registryId(), {
        fields: regFields,
      });
      if (!reg) {
        nogthrow(ERR_FSO_CATALOG, {
          reason: `Failed to find registry for FSO repo \`${repo.id()}\`.`,
        });
      }

      const conn = connByRegistry.get(reg.name());
      if (!conn) {
        nogthrow(ERR_FSO_CATALOG, {
          reason: `Missing GRPC for registry \`${reg.name()}\`.`,
        });
      }

      const rpc = conn.gitNogClient();
      const perRpcOpts = {
        deadline: Date.now() + 30 * 1000, // 30s.
        credentials: createAuthorizationCallCreds(
          rpcAuthorization,
          euid,
          { scope: { action: AA_FSO_READ_REPO, path: repoPath } },
        ),
      };

      const metaO = rpc.metaSync({ repo: repo.fsoId() }, perRpcOpts);
      const meta = JSON.parse(metaO.metaJson);
      const { catalog } = meta;
      if (!_.isObject(catalog)) {
        nogthrow(ERR_NOT_OF_KIND);
      }
      check(catalog.config, String);
      const parsed = EJSON.parse(catalog.config);
      check(parsed, matchCatalogConfig);
      return {
        catalogId: repo.id(),
        config: parsed,
        nameDetails: { fsoPath: repo.path() },
      }
    },
  };
}

export {
  createFsoCatalogPlugin,
};
