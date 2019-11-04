import { Mongo } from 'meteor/mongo';
import { check, Match } from 'meteor/check';
import {
  CollNameMdItems,
  CollNameMdProperties,
  CollNameMdPropertyTypes,
  defMethodCalls,
  makeCollName,
} from './autosuggest.js';
import {
  createPropertyTypeLearner,
  sudoCreateKnownPropertyInserter,
} from './known-values.js';
import {
  fixedMd,
  fixedSuggestionNamespaces,
} from './fixed-md-data.js';
import {
  matchFixedMd,
  matchFixedSuggestionNamespaceOp,
  matchFixedSuggestionNamespaceOps,
  matchOneFixedMd,
  mdNamespaceForData,
  sudoApplyFixedSuggestionNamespaces,
  sudoInsertFixedMd,
} from './fixed-md.js';

const AA_SYS_READ = 'sys/read';
const AA_SYS_WRITE = 'sys/write';
const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_READ_REPO_TREE = 'fso/read-repo-tree';

function log(msg, ...args) {
  console.log(`[suggest] ${msg}`, ...args);
}

function logerr(msg, ...args) {
  console.error(`[suggest] ${msg}`, ...args);
}

// Only enable temporarily during development.
function logdebug(msg, ...args) {
  // console.error(`[suggest] DEBUG ${msg}`, ...args);
}

// From
// <https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions>.
function escapeRegExp(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function createMethods({
  checkAccess, mdProperties, mdPropertyTypes, mdItems,
}) {
  return {
    fetchMetadataProperties(euid, opts) {
      check(opts, {
        sugnss: [String],
        needle: String,
      });
      const { sugnss, needle } = opts;

      if (!sugnss.length) {
        return [];
      }
      for (const ns of sugnss) {
        checkAccess(euid, AA_SYS_READ, { path: ns });
      }

      const sel = {
        sugnss: { $in: sugnss },
        tokens: {
          $regex: new RegExp(`^${escapeRegExp(needle.toLowerCase())}`),
        },
      };
      const sort = { symbol: 1 };
      const limit = 100;
      return mdProperties.find(sel, { sort, limit }).fetch();
    },

    fetchMetadataPropertyTypes(euid, opts) {
      check(opts, {
        sugnss: [String],
        symbols: [String],
      });
      const { sugnss, symbols } = opts;

      if (!sugnss.length) {
        return [];
      }
      for (const ns of sugnss) {
        checkAccess(euid, AA_SYS_READ, { path: ns });
      }

      const sel = {
        sugnss: { $in: sugnss },
        symbol: { $in: symbols },
      };
      return mdPropertyTypes.find(sel).fetch();
    },

    fetchMetadataItems(euid, opts) {
      check(opts, {
        sugnss: [String],
        ids: [String],
        needle: String,
        ofType: [String],
      });
      const {
        sugnss, ids, needle, ofType,
      } = opts;

      if (!sugnss.length) {
        return [];
      }
      for (const ns of sugnss) {
        checkAccess(euid, AA_SYS_READ, { path: ns });
      }

      let items = [];

      // Send all type definitions.
      const selType = {
        sugnss: { $in: sugnss },
        _id: { $in: ids },
      };
      items = items.concat(items,
        mdItems.find(selType).fetch(),
      );

      // If `needle` is non-trivial, send a limited number of items that are
      // `ofType`.
      if (needle.length >= 2) {
        const sel = {
          sugnss: { $in: sugnss },
          ofType: { $in: ofType },
          tokens: {
            $regex: new RegExp(`^${escapeRegExp(needle.toLowerCase())}`),
          },
        };
        const sort = { symbol: 1 };
        const limit = 100;
        items = items.concat(items,
          mdItems.find(sel, { sort, limit }).fetch(),
        );
      }

      return items;
    },
  };
}

function createAdminMethods({
  checkAccess, mdProperties, mdPropertyTypes, mdItems, openFsoRepo,
}) {
  return {
    applyFixedMdFromRepo(euid, opts) {
      check(opts, {
        repo: String,
        mdnss: [String],
      });
      const { repo, mdnss } = opts;

      if (!mdnss.length) {
        return '';
      }
      for (const ns of mdnss) {
        checkAccess(euid, AA_SYS_WRITE, { path: ns });
      }

      const fp = openFsoRepo(euid, {
        actions: [AA_FSO_READ_REPO, AA_FSO_READ_REPO_TREE],
        path: repo,
      });

      let nPaths = 0;
      let nInserted = 0;
      function insertPathMeta({ path, meta }) {
        nPaths += 1;

        function insertMd(dat) {
          const ns = mdNamespaceForData(dat);
          if (!mdnss.includes(ns)) {
            logerr(
              'Denied insert to mdns.',
              'path', path,
              'mdns', ns,
            );
            return;
          }

          sudoInsertFixedMd({
            mdProperties, mdPropertyTypes, mdItems, fixedMd: [dat],
          });
          nInserted += 1;
          logdebug(
            'Inserted fixed MD.',
            'path', path,
            'type', dat.type,
            'symbol', dat.symbol,
          );
        }

        if (Match.test(meta, matchOneFixedMd)) {
          insertMd(meta);
          return;
        }

        const metaKey = 'nog_fixed_md';
        const fixed = meta[metaKey];
        if (!fixed) {
          logdebug(
            `Ignored path metadata without \`${metaKey}\`.`,
            'path', path,
          );
          return;
        }

        if (Match.test(fixed, matchOneFixedMd)) {
          insertMd(fixed);
          return;
        }
        if (Match.test(fixed, matchFixedMd)) {
          fixed.forEach(insertMd);
          return;
        }

        logdebug(
          `Ignored path metadata with invalid \`${metaKey}\`.`,
          'path', path,
        );
      }

      const head = fp.head();
      fp.listMetaTree({
        metaGitCommit: head.gitCommits.meta,
        onPathMeta: insertPathMeta,
      });

      log('Inserted fixed MD.', 'nInserted', nInserted);
      return (
        `Inserted ${nInserted} fixed md ` +
        `from ${nPaths} path metadata.`
      );
    },

    applyFixedSuggestionNamespacesFromRepo(euid, opts) {
      check(opts, {
        repo: String,
        mdnss: [String],
        sugnss: [String],
      });
      const { repo, mdnss, sugnss } = opts;

      if (!mdnss.length || !sugnss.length) {
        return '';
      }
      for (const ns of mdnss) {
        checkAccess(euid, AA_SYS_READ, { path: ns });
      }
      for (const ns of sugnss) {
        checkAccess(euid, AA_SYS_WRITE, { path: ns });
      }

      const fp = openFsoRepo(euid, {
        actions: [AA_FSO_READ_REPO, AA_FSO_READ_REPO_TREE],
        path: repo,
      });

      let nPaths = 0;
      let nApplied = 0;
      function applyPathMeta({ path, meta }) {
        nPaths += 1;

        function applyOp(dat) {
          if (!sugnss.includes(dat.sugns)) {
            logerr(
              'Denied insert into sugnss.',
              'path', path,
              'sugnss', dat.sugns,
            );
            return;
          }

          if (dat.mdns && !mdnss.includes(dat.mdns)) {
            logerr(
              'Denied read from mdnss.',
              'path', path,
              'mdnss', dat.mdns,
            );
            return;
          }
          if (dat.suggestFromMdnss) {
            for (const ns of dat.suggestFromMdnss) {
              if (!mdnss.includes(ns)) {
                logerr(
                  'Denied read from mdnss.',
                  'path', path,
                  'mdnss', ns,
                );
                return;
              }
            }
          }

          sudoApplyFixedSuggestionNamespaces({
            mdProperties, mdPropertyTypes, mdItems,
            fixedSuggestionNamespaces: [dat],
          });
          nApplied += 1;
          logdebug(
            'Applied suggestion namespace op.',
            'path', path,
            'op', dat.op,
            'id', dat.id,
            'symbol', dat.symbol,
            'mdns', dat.mdns,
          );
        }

        if (Match.test(meta, matchFixedSuggestionNamespaceOp)) {
          applyOp(meta);
          return;
        }

        const metaKey = 'nog_fixed_suggestion_namespace';
        const fixed = meta[metaKey];
        if (!fixed) {
          logdebug(
            `Ignored path metadata without \`${metaKey}\`.`,
            'path', path,
          );
          return;
        }

        if (Match.test(fixed, matchFixedSuggestionNamespaceOp)) {
          applyOp(fixed);
          return;
        }
        if (Match.test(fixed, matchFixedSuggestionNamespaceOps)) {
          fixed.forEach(applyOp);
          return;
        }

        logdebug(
          `Ignored path metadata with invalid \`${metaKey}\`.`,
          'path', path,
        );
      }

      const head = fp.head();
      fp.listMetaTree({
        metaGitCommit: head.gitCommits.meta,
        onPathMeta: applyPathMeta,
      });

      log('Applied suggestion namespace ops.', 'nApplied', nApplied);
      return (
        `Applied ${nApplied} suggestion namespace ops ` +
        `from ${nPaths} path metadata.`
      );
    },
  };
}

function createSuggestModuleServer({
  namespace, checkAccess, openFsoRepo,
}) {
  check(namespace, Match.ObjectIncluding({
    coll: String,
    meth: String,
  }));
  check(checkAccess, Function);
  check(openFsoRepo, Function);

  const mdProperties = new Mongo.Collection(
    makeCollName(namespace, CollNameMdProperties),
  );
  mdProperties.rawCollection().createIndex({ tokens: 1 });

  const mdPropertyTypes = new Mongo.Collection(
    makeCollName(namespace, CollNameMdPropertyTypes),
  );
  mdPropertyTypes.rawCollection().createIndex({ symbol: 1 });

  const mdItems = new Mongo.Collection(
    makeCollName(namespace, CollNameMdItems),
  );
  mdItems.rawCollection().createIndex({ tokens: 1 });

  sudoInsertFixedMd({
    mdProperties, mdPropertyTypes, mdItems, fixedMd,
  });
  sudoApplyFixedSuggestionNamespaces({
    mdProperties, mdPropertyTypes, mdItems, fixedSuggestionNamespaces,
  });

  function createPropertyInserter(euid, {
    mdNamespace, sugNamespaces,
  }) {
    checkAccess(euid, AA_SYS_WRITE, { path: mdNamespace });
    for (const ns of sugNamespaces) {
      checkAccess(euid, AA_SYS_WRITE, { path: ns });
    }
    return sudoCreateKnownPropertyInserter({
      mdPropertyTypes, mdItems, mdNamespace, sugNamespaces,
    });
  }

  const module = {
    ...createMethods({
      checkAccess, mdProperties, mdPropertyTypes, mdItems,
    }),
    ...createAdminMethods({
      checkAccess, mdProperties, mdPropertyTypes, mdItems, openFsoRepo,
    }),
    createPropertyInserter,
    createPropertyTypeLearner,
  };
  // Register Meteor methods without assigning `callX()` functions to `module`.
  // Server code should call the real functions, not via a Meteor method.
  defMethodCalls(module, { namespace });
  return module;
}

export {
  createSuggestModuleServer,
};
