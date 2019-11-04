import crypto from 'crypto';
import { EJSON } from 'meteor/ejson';
import { Match, check } from 'meteor/check';
import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { Counter } from 'meteor/natestrauser:publish-performant-counts';
import { _ } from 'meteor/underscore';
import { NogError } from 'meteor/nog-error';
const {
  ERR_CONFLICT,
  ERR_CONTENT_MISSING,
  ERR_LOGIC,
  ERR_LOST_LOCK,
  ERR_NOT_OF_KIND,
  ERR_REF_NOT_FOUND,
  ERR_REPO_MISSING,
  ERR_UNIMPLEMENTED,
  nogthrow,
} = NogError;
import { matchPositiveNumber } from 'meteor/nog-settings';

import { defCatalogMethods } from './nog-catalog-methods.js';
import { makeCollName, makePubName } from './nog-catalog-common.js';
import { createMongoDocLock } from './concurrency.js';
import {
  matchCatalogPipelineStep,
  compilePipeline,
} from './catalog-pipeline.js';
import './nog-catalog-settings.js';


const AA_UPDATE_CATALOG = 'nog-catalog/update';
const AA_CONFIGURE_CATALOG = 'nog-catalog/configure';
const AA_GET_CONTENT = 'nog-content/get';
const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_UPDATE_CATALOG = 'fso/update-catalog';

const {
  catalogMaxStringLength,
  catalogMaxNumMetaIndexes,
} = Meteor.settings;

const rgxUuid = (
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
);

const matchUuidStringOrBinary = Match.Where((x) => {
  if (Match.test(x, String)) {
    if (!x.match(rgxUuid)) {
      throw new Match.Error('Invalid string UUID.');
    }
    return true;
  } else if (Match.test(x, Match.Where(EJSON.isBinary))) {
    if (x.length !== 16) {
      throw new Match.Error('Invalid binary UUID.');
    }
    return true;
  }
  throw new Match.Error('Expected String or Binary.');
});

const matchSimpleName = Match.Where((x) => {
  check(x, String);
  if (!x.match(/^[a-zA-Z0-9-_]+$/)) {
    throw new Match.Error('Invalid simple name.');
  }
  return true;
});

const matchCatalogVolumeName = Match.Where((x) => {
  check(x, String);
  const rgxName = /^[\w.]+\.vol_\w+_\d+$/;
  if (!rgxName.test(x)) {
    throw new Match.Error('Invalid catalog volume name.');
  }
  return true;
});

const matchMetaFieldName = Match.Where((x) => {
  check(x, String);
  const rgxName = /^m\d+$/;
  if (!rgxName.test(x)) {
    throw new Match.Error('Invalid meta field name.');
  }
  return true;
});

const matchCatalogContentRepoConfig = Match.Where((x) => {
  check(x, {
    repoSelector: Object,  // MongoDB selector.
    pipeline: [matchCatalogPipelineStep],
  });
  return true;
});

const matchCatalogConfig = Match.Where((x) => {
  check(x, {
    preferredMetaKeys: [String],
    contentRepoConfigs: [matchCatalogContentRepoConfig],
  });
  return true;
});


function sha1Hex(d) {
  return crypto.createHash('sha1').update(d, 'utf8').digest('hex');
}


function isDuplicateMongoIdError(err) {
  return err.code === 11000;
}


// `tokenize()` splits a search string into text tokens and prefix tokens of
// format `<prefix>:<text>`.  It has been copied from `nog-search.coffee`.  See
// there for details.
//
// `compileQuery()` uses the prefix tokens to compile a regex MongoDB selector,
// which should be useful for expert users.  Text tokens are joined for
// fulltext search over the string fields.
//
// XXX The query language should probably be clarified and be factored out and
// unified with the query language for `nog-search`.

function tokenize(str) {
  const m = str.match(/(\S+:)?"[^"]+"|\S+/g);
  if (!m) {
    return [];
  }
  const tokens = [];
  for (let tok of m) {
    const isQuoted = !!tok.match(/"/);
    tok = tok.replace(/"/g, '');  // Remove quotes.
    const parts = tok.split(':');
    if (parts.length === 1) {
      tokens.push({ type: 'text', text: tok, isQuoted });
    } else {
      tokens.push({
        type: 'prefix',
        text: parts.slice(1).join(':'),
        prefix: parts[0],
        isQuoted,
      });
    }
  }
  return tokens;
}


function compileQuery(str) {
  const tokens = tokenize(str);
  const query = {};
  const text = [];
  for (const tok of tokens) {
    if (tok.type === 'prefix') {
      let { prefix } = tok;
      if (prefix.match(/^m\d+$/)) {
        prefix = `m.${prefix}`;
      }
      query[prefix] = { $regex: tok.text, $options: 'i' };
    } else if (tok.type === 'text') {
      text.push(tok.text);
    }
  }
  if (text.length > 0) {
    query.$text = { $search: text.join(' ') };
  }
  return query;
}


function getCatalogConfig({
  contentStore: store, euid, ownerName, repoName,
}) {
  const nameSel = { ownerName, repoName };
  const refName = 'branches/master';
  // `store` getters throw if missing.
  const refs = store.getRefs(euid, nameSel);
  const master = refs[refName];
  const commit = store.getCommit(euid, { ...nameSel, sha1: master });
  const tree = store.getTree(euid, { ...nameSel, sha1: commit.tree });
  const { catalog } = tree.meta;
  if (!_.isObject(catalog)) {
    nogthrow(ERR_NOT_OF_KIND);
  }
  check(catalog.config, String);
  const parsed = EJSON.parse(tree.meta.catalog.config);
  check(parsed, matchCatalogConfig);
  return parsed;
}


function createFieldManager({
  catalogs, catalogId, preferredMetaKeys, stage,
}) {
  const metaKeysField = `${stage}.metaKeys`;
  catalogs.update(
    { _id: catalogId, [metaKeysField]: { $exists: false } },
    { $set: { [metaKeysField]: preferredMetaKeys } },
  );

  const keyIndexOrder = [...preferredMetaKeys];
  const keyToField = {};
  const fieldToKey = {};

  function readKeyMap() {
    const { metaKeys } = catalogs.findOne(catalogId)[stage];
    // eslint-disable-next-line no-plusplus
    for (let i = 0; i < metaKeys.length; i++) {
      const key = metaKeys[i];
      const field = `m${i}`;
      keyToField[key] = field;
      fieldToKey[field] = key;
    }
  }

  function allocField(metaKey) {
    catalogs.update(
      { _id: catalogId, [metaKeysField]: { $ne: metaKey } },
      { $push: { [metaKeysField]: metaKey } },
    );
    readKeyMap();
    return keyToField[metaKey];
  }

  readKeyMap();

  return {
    getMongoField(metaKey) {
      const f = keyToField[metaKey];
      if (f) {
        return f;
      }
      return allocField(metaKey);
    },

    getKey(field) {
      return fieldToKey[field];
    },

    getMongoFields() {
      return _.values(keyToField);
    },

    getMongoIndexFields() {
      const fields = new Set();
      for (const k of keyIndexOrder) {
        fields.add(this.getMongoField(k));
      }
      for (const f of this.getMongoFields()) {
        fields.add(f);
      }
      return [...fields.values()].splice(0, catalogMaxNumMetaIndexes);
    },

    isMongoFieldName(name) {
      return !!name.match(/^m\d+$/);
    },
  };
}


function truncatedMetaVal(val) {
  let isTruncated = false;

  if (_.isNumber(val) || Match.test(val, [Number])) {
    return { val, isTruncated };
  }

  function truncated(s) {
    if (s.length <= catalogMaxStringLength) {
      return s;
    }
    isTruncated = true;
    const indicator = '[TRUNCATED]';
    const last = catalogMaxStringLength - indicator.length - 1;
    return s.slice(0, last) + indicator;
  }

  let trunc;
  if (_.isString(val)) {
    trunc = truncated(val);
  } else {
    trunc = val.map(truncated);
  }

  return { val: trunc, isTruncated };
}


function treeWalk({ contentStore, treeId, rateLimiters }, cb) {
  function walk(entry, parentPath) {
    let content = null;
    rateLimiters.readLimiter.op();
    if (entry.type === 'object') {
      content = contentStore.objects.findOne(entry.sha1);
    } else if (entry.type === 'tree') {
      content = contentStore.trees.findOne(entry.sha1);
    } else {
      const reason = `Unknown entry type ${entry.type}.`;
      nogthrow(ERR_LOGIC, { reason });
    }
    if (!content) {
      const reason = `Missing entry ${JSON.stringify(entry)}.`;
      nogthrow(ERR_CONTENT_MISSING, { reason });
    }

    let path;
    if (parentPath == null) {
      path = '';
    } else if (parentPath === '') {
      path = content.name;
    } else {
      path = `${parentPath}/${content.name}`;
    }

    cb({ path, ...content }, rateLimiters);

    if (content.entries) {
      for (const e of content.entries) {
        walk(e, path);
      }
    }
  }

  walk({ type: 'tree', sha1: treeId }, null);
}


function addContentRepoSudo({
  contentStore, repo, volume, fieldManager, pipelineFuncs, logger, renewLock,
  rateLimiters,
}) {
  const refName = 'branches/master';
  const repoId = repo._id;

  const commitId = repo.refs[refName];
  if (!commitId) {
    nogthrow(ERR_REF_NOT_FOUND);
  }
  const commit = contentStore.commits.findOne(commitId);
  if (!commit) {
    nogthrow(ERR_CONTENT_MISSING);
  }

  const repoFullname = `${repo.owner}/${repo.name}`;
  logger.log(
    `[nog-catalog] Begin full entry scan of repo \`${repoFullname}\`.`,
  );

  let totalCount = 0;
  let selectCount = 0;

  treeWalk({
    contentStore, treeId: commit.tree, rateLimiters,
  }, (origContent) => {
    renewLock();

    let content = { ...origContent };

    totalCount += 1;
    for (const fn of pipelineFuncs) {
      content = fn(content);
      if (!content) {
        return;
      }
    }
    selectCount += 1;

    const fixed = {
      _id: content._id,
      name: content.name,
      m: {},
    };

    for (const [k, v] of _.pairs(content.meta)) {
      // XXX We silently ignore null and objects.  Is this good enough?
      if (Match.test(v, Match.OneOf(String, Number, [String], [Number]))) {
        const { val, isTruncated } = truncatedMetaVal(v);
        if (isTruncated) {
          logger.log(
            '[nog-catalog] Truncated meta value in databank ' +
            `to satisfy max string length ${catalogMaxStringLength}: ` +
            `${content._id}.meta.${k}`,
          );
        }
        const field = fieldManager.getMongoField(k);
        // Force value to array, so that `$unwind` in MongoDB pipelines works
        // correctly with MongoDB < 3.2.  This could be removed when we use
        // MongoDB >= 3.2 everywhere.  See `publishCatalogVolumeStats()`.
        if (_.isArray(val)) {
          fixed.m[field] = val;
        } else {
          fixed.m[field] = [val];
        }
      }
    }

    const refpath = {
      repoId,
      owner: repo.owner,
      repo: repo.name,
      commitId,
      path: content.path,
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
  });

  volume.update(
    { 'refpaths.repoId': repoId },
    { $pull: { refpaths: { repoId, commitId: { $ne: commitId } } } },
    { multi: true },
  );

  logger.log(
    `[nog-catalog] End full entry scan of repo \`${repoFullname}\`: ` +
    `selected ${selectCount} of ${totalCount} entries.`,
  );
}


function dropRetiredVolumes({ catalogs, logger }) {
  catalogs.find(
    { retiredVolumes: { $exists: true } },
    { fields: { retiredVolumes: true } },
  ).forEach((catalog) => {
    catalog.retiredVolumes.forEach(({ name }) => {
      const c = new Mongo.Collection(name, { defineMutationMethods: false });
      c.rawCollection().drop();
      logger.log(`[nog-catalog] Dropped retired volume \`${name}\`.`);
    });
    catalogs.update(catalog._id, { $unset: { retiredVolumes: '' } });
  });
}


// `dropStaleCatalogs()` removes catalogs whose name does not match the
// `catalogId`.  Such catalogs may be leftovers of deleted catalog repos.
function dropStaleCatalogs({
  catalogs, catalogId, nameDetails, logger,
}) {
  catalogs.find(
    {
      ...nameDetails, // `{ owner, name }` or `{ fsoPath }`
      _id: { $ne: catalogId },
    },
    {
      fields: {
        retiredVolumes: true,
        'active.volumes': true,
        'building.volumes': true,
      },
    },
  ).forEach((catalog) => {
    const volumes = [].concat(
      catalog.retiredVolumes || [],
      catalog.active ? catalog.active.volumes : [],
      catalog.building ? catalog.building.volumes : [],
    );
    volumes.forEach(({ name }) => {
      const c = new Mongo.Collection(name, { defineMutationMethods: false });
      c.rawCollection().drop();
      logger.log(`[nog-catalog] Dropped stale volume \`${name}\`.`);
    });
    catalogs.remove(catalog._id);
    logger.log(
      `[nog-catalog] Dropped stale catalog \`${catalog._id}\` ` +
      `${EJSON.stringify(nameDetails)}.`,
    );
  });
}


// `createCollection(name)` ensures that the MongoDB collection exists on disk.

function createCollection(name) {
  const c = new Mongo.Collection(name);
  if (!c.findOne()) {
    c.remove(c.insert({}));
  }
  return c;
}


function createLogger() {
  return {
    messages: [],

    log(msg) {
      const nowIso = (new Date()).toISOString();
      this.messages.push(`${nowIso}: ${msg}`);
      console.log(msg);
    },
  };
}

const matchCatalogPlugin = Match.Where((x) => {
  check(x, {
    name: String,
    version: Number,
    repoSelectorKey: String,
    addContent: Function,
    getCatalogConfig: Function,
  });
  return true;
});

function createPluginRegistry() {
  return {
    // `plugins` contains all plugins in registration order.
    plugins: [],

    // `fso` contains the `fso` plugin if is has been registered.
    fso: null,

    versions() {
      return this.plugins.map(p => ({
        name: p.name,
        version: p.version,
      }));
    },

    register(plug) {
      check(plug, matchCatalogPlugin);
      this.plugins.push(plug);
      if (plug.name === 'fso') {
        this.fso = plug;
      }
    },

    findAddContentHook(repoSelector) {
      for (const plug of this.plugins) {
        if (_.has(repoSelector, plug.repoSelectorKey)) {
          return plug;
        }
      }
      return null;
    },
  };
}

function createCatalogServerModule({
  namespace,
  contentStore,
  checkAccess, testAccess,
  rateLimiters,
}) {
  const catalogs = new Mongo.Collection(makeCollName(namespace, 'catalogs'));
  const volumes = {};
  const plugins = createPluginRegistry();
  let nogSuggest = null;

  function findCatalogVolume(euid, { ownerName, repoName, volumeName }) {
    if (!testAccess(euid, AA_GET_CONTENT, { ownerName, repoName })) {
      console.log(
        `[nog-catalog] Access denied to catalog volume \`${volumeName}\`.`,
      );
      return null;
    }

    const catalog = catalogs.findOne(
      { owner: ownerName, name: repoName },
      { fields: { 'active.volumes': 1 } },
    );
    if (!catalog || !catalog.active) {
      return null;
    }

    const volNames = new Set(catalog.active.volumes.map(v => v.name));
    if (!volNames.has(volumeName)) {
      return null;
    }

    const vol = volumes[volumeName] || createCollection(volumeName);
    volumes[volumeName] = vol;
    return vol;
  }

  function findCatalogVolumeFso(euid, { repoPath, volumeName }) {
    if (!testAccess(euid, AA_FSO_READ_REPO, { path: repoPath })) {
      console.log(
        `[nog-catalog] Access denied to catalog volume \`${volumeName}\`.`,
      );
      return null;
    }

    const catalog = catalogs.findOne(
      { fsoPath: repoPath },
      { fields: { 'active.volumes': 1 } },
    );
    if (!catalog || !catalog.active) {
      return null;
    }

    const volNames = new Set(catalog.active.volumes.map(v => v.name));
    if (!volNames.has(volumeName)) {
      return null;
    }

    const vol = volumes[volumeName] || createCollection(volumeName);
    volumes[volumeName] = vol;
    return vol;
  }

  function updateCatalogCommon(euid, {
    catalogId, config, nameDetails, options = {},
  }) {
    const logger = createLogger();
    const version = 2;  // Bump to force rebuild when implementation changes.
    const pluginVersions = plugins.versions();
    const configHash = sha1Hex(EJSON.stringify(
      { version, pluginVersions, config }, { canonical: true },
    ));
    const { contentRepoConfigs, preferredMetaKeys } = config;

    try {
      catalogs.insert({ _id: catalogId });
    } catch (err) {
      if (!isDuplicateMongoIdError(err)) {
        throw err;
      }
    }
    // Save the last user that ran update.  The `FsoCatalogUpdater` uses it for
    // automatic background updates.
    const updateUser = _.isObject(euid) ? euid._id : euid;
    catalogs.update(catalogId, {
      $set: {
        updateUser,
        ...nameDetails,
      },
    });

    const lock = createMongoDocLock({
      collection: catalogs, docId: catalogId, core: { op: 'UPDATE' },
      logPrefix: `[nog-catalog] updateCatalog \`${catalogId}\``,
    });
    if (!lock.tryLock()) {
      nogthrow(ERR_CONFLICT);
    }

    function renewLock() {
      if (!lock.tryRenew()) {
        nogthrow(ERR_LOST_LOCK);
      }
    }

    dropStaleCatalogs({
      catalogs, catalogId, nameDetails, logger,
    });

    let catalog = catalogs.findOne(catalogId);
    let stage = null;
    let serial = null;
    if (catalog.active && catalog.active.configHash === configHash) {
      stage = 'active';
      ({ serial } = catalog.active);
      if (catalog.building) {
        const retire = catalog.building.volumes || [];
        catalogs.update(catalogId, {
          $addToSet: { retiredVolumes: { $each: retire } },
          $unset: { building: '' },
        });
      }
    } else {
      stage = 'building';
      let retire = [];
      if (catalog.building) {
        retire = catalog.building.volumes || [];
      }
      catalogs.update(
        { _id: catalogId, 'building.configHash': { $ne: configHash } },
        {
          $addToSet: { retiredVolumes: { $each: retire } },
          $set: { building: { configHash, volumes: [] } },
          $inc: { serial: 1 },
        },
      );
      catalog = catalogs.findOne(catalogId);
      ({ serial } = catalog);
      catalogs.update(catalogId, {
        $set: { 'building.serial': serial },
      });
    }
    logger.log(
      `[nog-catalog] Using catalog serial ${catalog.serial}, ` +
      `config hash ${configHash}, in stage \`${stage}\`.`,
    );

    const volumeName = makeCollName(
      namespace, `catalogs.vol_${catalogId}_${serial}`,
    );
    const volume = volumes[volumeName] || createCollection(volumeName);
    volumes[volumeName] = volume;

    catalogs.update(catalogId, {
      $addToSet: { [`${stage}.volumes`]: { name: volumeName } },
    });

    const fieldManager = createFieldManager({
      catalogs, catalogId, preferredMetaKeys, stage,
    });

    for (const cfg of contentRepoConfigs) {
      renewLock();

      const { repoSelector, pipeline } = cfg;
      const pipelineFuncs = compilePipeline(pipeline);

      const plug = plugins.findAddContentHook(repoSelector);
      if (plug) {
        plug.addContent(euid, {
          catalogs, catalogId, stage,
          volume, fieldManager, truncatedMetaVal,
          repoSelector, pipelineFuncs,
          logger, renewLock, rateLimiters,
          options,
        });
        continue; // eslint-disable-line no-continue
      }

      contentStore.deletedRepos.find(repoSelector).forEach((repo) => {
        renewLock();

        const repoId = repo._id;
        if (contentStore.repos.findOne(repoId)) {
          return;  // Content not yet fully deleted.
        }
        const sel = {
          _id: catalogId,
          [`${stage}.deletedRepos`]: repoId,
        };
        if (catalogs.findOne(sel)) {
          return;  // Catalog entries already removed.
        }

        const repoFullname = `${repo.owner}/${repo.name}`;
        logger.log(
          `[nog-catalog] Begin entry cleanup for ` +
          `deleted repo \`${repoFullname}\`.`,
        );

        volume.update(
          { 'refpaths.repoId': repoId },
          { $pull: { refpaths: { repoId } } },
          { multi: true },
        );
        volume.remove({ refpaths: [] });
        catalogs.update(catalogId, {
          $pull: { [`${stage}.repos`]: { repoId } },
          $addToSet: { [`${stage}.deletedRepos`]: repoId },
        });

        logger.log(
          `[nog-catalog] Completed entry cleanup for ` +
          `deleted repo \`${repoFullname}\`.`,
        );
      });

      // XXX Repos that may not be accessed are ignored.  The behavior is
      // undefined when access permissions change.  The current
      // implementation preserves catalog items if the repo has been indexed
      // before.

      contentStore.repos.find(repoSelector).forEach((repo) => {
        renewLock();

        const haveAccess = testAccess(euid, AA_GET_CONTENT, {
          ownerName: repo.owner,
          repoName: repo.name,
        });
        if (!haveAccess) {
          logger.log(
            `[nog-catalog] Ignored repo \`${repo.owner}/${repo.name}\` ` +
            'without access.',
          );
          return;
        }

        const repoId = repo._id;
        const commit = repo.refs['branches/master'];
        const sel = {
          _id: catalogId,
          [`${stage}.repos`]: { repoId, commit },
        };
        if (catalogs.findOne(sel)) {
          return;  // Already up-to-date.
        }
        addContentRepoSudo({
          contentStore, repo, volume, fieldManager, pipelineFuncs, logger,
          renewLock, rateLimiters,
        });
        volume.remove({ refpaths: [] });
        catalogs.update(catalogId, {
          $addToSet: { [`${stage}.repos`]: { repoId, commit } },
        });
        catalogs.update(catalogId, {
          $pull: { [`${stage}.repos`]: { repoId, commit: { $ne: commit } } },
        });
      });
    }

    const volumeRaw = volume.rawCollection();
    volumeRaw.indexesSync = Meteor.wrapAsync(volumeRaw.indexes, volumeRaw);

    const currentIndexes = new Set(
      volumeRaw.indexesSync().map(idx => idx.name),
    );
    const desiredIndexes = new Set(fieldManager.getMongoIndexFields());

    for (const idx of currentIndexes) {
      renewLock();
      if (idx === 'text') {
        continue; // eslint-disable-line no-continue
      }
      if (fieldManager.isMongoFieldName(idx) && !desiredIndexes.has(idx)) {
        volume._dropIndex(idx);
        logger.log(`[nog-catalog] Dropped index ${idx}`);
      }
    }

    if (!currentIndexes.has('text')) {
      volume._ensureIndex({ '$**': 'text' }, { name: 'text' });
    }
    for (const idx of desiredIndexes) {
      renewLock();
      if (!currentIndexes.has(idx)) {
        volume._ensureIndex({ [`m.${idx}`]: 1 }, { name: idx });
      }
    }

    renewLock();
    volume._ensureIndex({ name: 1 });
    renewLock();
    volume._ensureIndex({ 'refpaths.repoId': 1 });

    const memoState = catalogs.findOne(catalogId, {
      fields: {
        [`${stage}.metaKeyCountsMemoHash`]: 1,
        [`${stage}.configHash`]: 1,
        [`${stage}.repos`]: 1,
        [`${stage}.deletedRepos`]: 1,
      },
    })[stage];
    const memoArgs = {
      configHash: memoState.configHash,
      repos: _.sortBy(memoState.repos, r => r.repoId),
      deletedRepos: _.sortBy(memoState.deletedRepos),
    };
    const memoHash = sha1Hex(EJSON.stringify(memoArgs, { canonical: true }));

    if (memoState.metaKeyCountsMemoHash !== memoHash) {
      const metaKeyCounts = {};
      for (const f of fieldManager.getMongoFields()) {
        renewLock();
        const cnt = volume.find({ [`m.${f}`]: { $exists: true } }).count();
        metaKeyCounts[f] = cnt;
      }
      catalogs.update(catalogId, {
        $set: {
          [`${stage}.metaKeyCounts`]: metaKeyCounts,
          [`${stage}.metaKeyCountsMemoHash`]: memoHash,
        },
      });
    }

    renewLock();
    if (stage === 'building') {
      if (catalog.active && catalog.active.volumes) {
        catalogs.update(catalogId, {
          $addToSet: { retiredVolumes: { $each: catalog.active.volumes } },
        });
      }

      catalogs.update(catalogId, { $rename: { building: 'active' } });
    }

    renewLock();
    dropRetiredVolumes({ catalogs, logger });

    lock.unlock();

    logger.log(
      `[nog-catalog] Completed catalog update: serial ${catalog.serial}, ` +
      `config hash ${configHash}.`,
    );

    return {
      status: 'ok',
      messages: logger.messages,
    };
  }

  const mod = {
    catalogs,
    volumes,

    registerPlugin(plug) {
      plugins.register(plug);
    },

    registerNogSuggest(plug) {
      nogSuggest = plug;
    },

    updateCatalog(euid, { ownerName, repoName }) {
      check(ownerName, matchSimpleName);
      check(repoName, matchSimpleName);

      checkAccess(euid, AA_UPDATE_CATALOG, { ownerName, repoName });

      const { _id: catalogId } = contentStore.repos.findOne(
        { owner: ownerName, name: repoName },
        { fields: { _id: 1 } },
      );
      if (!catalogId) {
        nogthrow(ERR_REPO_MISSING);
      }

      const config = getCatalogConfig({
        contentStore, euid, ownerName, repoName,
      });
      const nameDetails = { owner: ownerName, name: repoName };

      return updateCatalogCommon(euid, {
        catalogId, config, nameDetails,
      });
    },

    updateCatalogFso(euid, opts) {
      check(opts, {
        repoPath: String,
        selectRepos: Match.Optional([matchUuidStringOrBinary]),
      });

      if (!plugins.fso) {
        nogthrow(ERR_UNIMPLEMENTED, {
          reason: 'The fso catalog plugin is disabled.',
        });
      }

      // The plugin calls `checkAccess()`.
      const { repoPath } = opts;
      const {
        catalogId, config, nameDetails,
      } = plugins.fso.getCatalogConfig(euid, { repoPath });

      function parseSelectRepos() {
        if (!opts.selectRepos) {
          return null;
        }
        return opts.selectRepos.map((id) => {
          if (typeof id === 'string') {
            return Buffer.from(id.replace(/-/g, ''), 'hex');
          }
          return Buffer.from(id);
        });
      }

      const options = {
        fsoSelectRepos: parseSelectRepos(),
      };
      return updateCatalogCommon(euid, {
        catalogId, config, nameDetails, options,
      });
    },

    configureCatalog(euid, {
      ownerName, repoName, catalogConfig,
    }) {
      check(ownerName, matchSimpleName);
      check(repoName, matchSimpleName);
      check(catalogConfig, matchCatalogConfig);

      checkAccess(euid, AA_CONFIGURE_CATALOG, {
        ownerName, repoName, catalogConfig,
      });
      const refName = 'branches/master';
      const refs = contentStore.getRefs(euid, { ownerName, repoName });
      const master = refs[refName];
      const commit = contentStore.getCommit(euid, {
        ownerName, repoName, sha1: master,
      });
      const tree = contentStore.getTree(euid, {
        ownerName, repoName, sha1: commit.tree,
      });

      const content = _.pick(tree, 'name', 'entries', 'meta');
      content.meta = { ...tree.meta };
      content.meta.catalog = {
        config: EJSON.stringify(catalogConfig, { canonical: true }),
      };
      const newTreeId = contentStore.createTree(euid, {
        ownerName, repoName, content,
      });

      const newCommitId = contentStore.createCommit(euid, {
        ownerName, repoName,
        content: {
          subject: 'Configure catalog',
          message: '',
          meta: {},
          parents: [master],
          tree: newTreeId,
        },
      });
      contentStore.updateRef(euid, {
        ownerName, repoName, refName, new: newCommitId, old: master,
      });
    },

    updateSuggestionsFromCatalog(euid, opts) {
      check(opts, {
        repoPath: String,
        mdNamespace: String,
        sugNamespaces: [String],
      });
      const { repoPath, mdNamespace, sugNamespaces } = opts;

      // XXX We should probably use a different AA here when NogSuggest
      // supports per-group suggestions.
      checkAccess(euid, AA_FSO_UPDATE_CATALOG, { path: repoPath });

      // Create `inserter` early, since it performs additional access checks.
      const inserter = nogSuggest.createPropertyInserter(euid, {
        mdNamespace, sugNamespaces,
      });

      const catalog = catalogs.findOne(
        { fsoPath: repoPath },
        {
          fields: {
            'active.metaKeys': 1,
            'active.serial': 1,
          },
        },
      );
      if (!catalog || !catalog.active) {
        nogthrow(ERR_REPO_MISSING);
      }
      const { _id: catalogId } = catalog;
      const { serial, metaKeys } = catalog.active;

      const volumeName = makeCollName(
        namespace, `catalogs.vol_${catalogId}_${serial}`,
      );
      const volume = volumes[volumeName];
      if (!volume) {
        nogthrow(ERR_REPO_MISSING);
      }

      const learner = nogSuggest.createPropertyTypeLearner();
      volume.find({}).forEach((doc) => {
        for (const [k, values] of Object.entries(doc.m)) {
          const property = metaKeys[Number(k.substr(1))];
          learner.update({ property, values });
        }
      });

      inserter.insertKnownProperties({
        properties: learner.predict({ threshold: 0.5, minCount: 1 }),
      });

      let nCandidates = 0;
      let nInserted = 0;
      volume.find({}).forEach((doc) => {
        for (const [k, values] of Object.entries(doc.m)) {
          nCandidates += values.length;
          const property = metaKeys[Number(k.substr(1))];
          nInserted += inserter.insertKnownPropertyValues({
            propertySymbol: property,
            values,
          });
        }
      });

      return (
        `Inserted ${nInserted} of ${nCandidates} suggestion candidates.`
      );
    },
  };

  defCatalogMethods({ namespace, mod });

  function publishCatalog({ ownerName, repoName }) {
    check(ownerName, matchSimpleName);
    check(repoName, matchSimpleName);

    if (!testAccess(this.userId, AA_GET_CONTENT, { ownerName, repoName })) {
      this.ready();
      return null;
    }

    return catalogs.find(
      { owner: ownerName, name: repoName },
      { fields: { owner: 1, name: 1, active: 1 } },
    );
  }

  function publishCatalogHitCount({
    ownerName, repoName, volumeName, filter,
  }) {
    check(ownerName, matchSimpleName);
    check(repoName, matchSimpleName);
    check(volumeName, matchCatalogVolumeName);
    check(filter, String);

    const euid = this.userId;
    const vol = findCatalogVolume(euid, { ownerName, repoName, volumeName });
    if (!vol) {
      console.error(`[nog-catalog] Did not find volume \`${volumeName}\`.`);
      this.ready();
      return null;
    }

    const query = compileQuery(filter);
    return new Counter(volumeName, vol.find(query));
  }

  function publishCatalogVolume({
    ownerName, repoName, volumeName, filter,
  }) {
    check(ownerName, matchSimpleName);
    check(repoName, matchSimpleName);
    check(volumeName, matchCatalogVolumeName);
    check(filter, String);

    const euid = this.userId;
    const vol = findCatalogVolume(euid, { ownerName, repoName, volumeName });
    if (!vol) {
      console.error(`[nog-catalog] Did not find volume \`${volumeName}\`.`);
      this.ready();
      return null;
    }

    const query = compileQuery(filter);
    return vol.find(query, {
      sort: { name: 1 },
      limit: 50,
    });
  }

  function publishCatalogVolumeStats({
    ownerName, repoName, volumeName, field, limit,
  }) {
    check(ownerName, matchSimpleName);
    check(repoName, matchSimpleName);
    check(volumeName, matchCatalogVolumeName);
    check(field, matchMetaFieldName);
    check(limit, matchPositiveNumber);

    const euid = this.userId;
    const volume = findCatalogVolume(euid, {
      ownerName, repoName, volumeName,
    });
    if (!volume) {
      console.error(`[nog-catalog] Did not find volume \`${volumeName}\`.`);
      this.ready();
      return null;
    }

    const statsName = `${volumeName}.stats`;
    const volumeRaw = volume.rawCollection();
    volume.aggregateSync = Meteor.wrapAsync(volumeRaw.aggregate, volumeRaw);

    // XXX The topk may contain spurious counts while the volume is updated,
    // since old entries are added long before new entries are finally removed.
    // The volume, therefore, may contain old and new entries at the same time.

    // XXX The stats won't reactively update on the client if the volume
    // changes.  We accept this for now.  An alternative would be to recompute
    // and resend them every x seconds.  Or we could implement logic on the
    // client to resubscribe if the databank state changes.  Or this
    // publication could observe something like `databank.active.xMemoHash` to
    // get notified when a volume update completes.  We could probably use
    // package `jcbernack:reactive-aggregate` to automate observe.

    // `$unwind` has been changed in MongoDB 3.2 to work on scalar values.  For
    // MongoDB < 3.2, `$unwind` fails on scalars.  `addContentRepoSudo()`
    // forces all values to arrays, so that `$unwind` can be used
    // unconditionally here also with MongoDB < 3.2.
    const topk = volume.aggregateSync([
      { $unwind: `$m.${field}` },
      { $group: { _id: `$m.${field}`, cnt: { $sum: 1 } } },
      { $sort: { cnt: -1 } },
      { $limit: Math.min(limit + 5, 1000) },
    ]);

    let nAdded = 0;
    topk.forEach((t) => {
      if (nAdded < limit && t._id != null) {
        this.added(statsName, `${field}:${t._id}`, {
          field, val: t._id, count: t.cnt,
        });
        nAdded += 1;
      }
    });

    this.ready();
    return null;
  }

  function publishCatalogFso({ repoPath }) {
    check(repoPath, String);

    if (!testAccess(this.userId, AA_FSO_READ_REPO, { path: repoPath })) {
      this.ready();
      return null;
    }

    return catalogs.find(
      { fsoPath: repoPath },
      { fields: { fsoPath: 1, active: 1 } },
    );
  }

  function publishCatalogHitCountFso({ repoPath, volumeName, filter }) {
    check(repoPath, String);
    check(volumeName, matchCatalogVolumeName);
    check(filter, String);

    const euid = this.userId;
    const vol = findCatalogVolumeFso(euid, { repoPath, volumeName });
    if (!vol) {
      console.error(`[nog-catalog] Did not find volume \`${volumeName}\`.`);
      this.ready();
      return null;
    }

    const query = compileQuery(filter);
    return new Counter(volumeName, vol.find(query));
  }

  function publishCatalogVolumeFso({ repoPath, volumeName, filter }) {
    check(repoPath, String);
    check(volumeName, matchCatalogVolumeName);
    check(filter, String);

    const euid = this.userId;
    const vol = findCatalogVolumeFso(euid, { repoPath, volumeName });
    if (!vol) {
      console.error(`[nog-catalog] Did not find volume \`${volumeName}\`.`);
      this.ready();
      return null;
    }

    const query = compileQuery(filter);
    return vol.find(query, {
      sort: { name: 1 },
      limit: 50,
    });
  }

  function publishCatalogVolumeStatsFso({
    repoPath, volumeName, field, limit,
  }) {
    check(repoPath, String);
    check(volumeName, matchCatalogVolumeName);
    check(field, matchMetaFieldName);
    check(limit, matchPositiveNumber);

    const euid = this.userId;
    const vol = findCatalogVolumeFso(euid, { repoPath, volumeName });
    if (!vol) {
      console.error(`[nog-catalog] Did not find volume \`${volumeName}\`.`);
      this.ready();
      return null;
    }

    const statsName = `${volumeName}.stats`;
    const volRaw = vol.rawCollection();
    vol.aggregateSync = Meteor.wrapAsync(volRaw.aggregate, volRaw);

    // XXX The topk may contain spurious counts while the volume is updated,
    // since old entries are added long before new entries are finally removed.
    // The volume, therefore, may contain old and new entries at the same time.

    // XXX The stats won't reactively update on the client if the volume
    // changes.  We accept this for now.  An alternative would be to recompute
    // and resend them every x seconds.  Or we could implement logic on the
    // client to resubscribe if the databank state changes.  Or this
    // publication could observe something like `databank.active.xMemoHash` to
    // get notified when a volume update completes.  We could probably use
    // package `jcbernack:reactive-aggregate` to automate observe.

    // `$unwind` has been changed in MongoDB 3.2 to work on scalar values.  For
    // MongoDB < 3.2, `$unwind` fails on scalars.  `addContentRepoSudo()`
    // forces all values to arrays, so that `$unwind` can be used
    // unconditionally here also with MongoDB < 3.2.
    const topk = vol.aggregateSync([
      { $unwind: `$m.${field}` },
      { $group: { _id: `$m.${field}`, cnt: { $sum: 1 } } },
      { $sort: { cnt: -1 } },
      { $limit: Math.min(limit + 5, 1000) },
    ]);

    let nAdded = 0;
    topk.forEach((t) => {
      if (nAdded < limit && t._id != null) {
        this.added(statsName, `${field}:${t._id}`, {
          field, val: t._id, count: t.cnt,
        });
        nAdded += 1;
      }
    });

    this.ready();
    return null;
  }

  const pubs = {
    // nog-content
    catalog: publishCatalog,
    catalogHitCount: publishCatalogHitCount,
    catalogVolume: publishCatalogVolume,
    catalogVolumeStats: publishCatalogVolumeStats,
    // fso
    catalogFso: publishCatalogFso,
    catalogHitCountFso: publishCatalogHitCountFso,
    catalogVolumeFso: publishCatalogVolumeFso,
    catalogVolumeStatsFso: publishCatalogVolumeStatsFso,
  };
  for (const [name, fn] of _.pairs(pubs)) {
    Meteor.publish(makePubName(namespace, name), fn);
  }

  return mod;
}


export {
  createCatalogServerModule,
  matchCatalogConfig,
};
