import { Meteor } from 'meteor/meteor';
import { check, Match } from 'meteor/check';

// `defMethodCalls()` registers Meteor methods and binds them to the
// server-side method implementation on `module`.  `module` must be `null` on
// the client.  The common code applies loose checks to catch obvious errors.
// The server methods on `module` must apply the real, stricter checks.
function defMethodCalls(module, { namespace }) {
  if (Meteor.isServer) {
    check(module, Match.ObjectIncluding({
      updateStat: Function,
      refreshContent: Function,
      storeMeta: Function,
      initRepo: Function,
      issueUserToken: Function,
      issueSysToken: Function,
      enableDiscoveryPath: Function,
    }));
  } else {
    check(module, null);
  }

  // For each call, register a Meteor method and return a wrapper function that
  // calls the method.
  function def(calls) {
    const wrapped = {};
    for (const [name, fn] of Object.entries(calls)) {
      const qualname = `${namespace.meth}.${name}`;
      Meteor.methods({ [qualname]: fn });
      wrapped[name] = (...args) => Meteor.call(qualname, ...args);
    }
    return wrapped;
  }

  return def({
    callUpdateStat(opts) {
      check(opts, {
        repoId: String,
        repoPath: String,
      });
      if (!module) {
        return null;
      }
      return module.updateStat(Meteor.user(), opts);
    },

    callRefreshContent(opts) {
      check(opts, {
        repoId: String,
        repoPath: String,
      });
      if (!module) {
        return null;
      }
      return module.refreshContent(Meteor.user(), opts);
    },

    callReinitSubdirTracking(opts) {
      check(opts, {
        repoId: String,
        repoPath: String,
        subdirTracking: String,
      });
      if (!module) {
        return null;
      }
      return module.reinitSubdirTracking(Meteor.user(), opts);
    },

    callStoreMeta(opts) {
      check(opts, {
        repoId: String,
        repoPath: String,
        meta: Object,
      });
      if (!module) {
        return null;
      }
      return module.storeMeta(Meteor.user(), opts);
    },

    callTriggerUpdateCatalogs(opts) {
      check(opts, {
        repoId: String,
        repoPath: String,
      });
      if (!module) {
        return null;
      }
      return module.triggerUpdateCatalogs(Meteor.user(), opts);
    },

    callInitRepo(opts) {
      check(opts, {
        registryName: String,
        globalPath: String,
      });
      if (!module) {
        return null;
      }
      return module.initRepo(Meteor.user(), opts);
    },

    callIssueUserToken(opts) {
      check(opts, {
        expiresIn: Number,
        scope: Match.Optional(Object),
        scopes: Match.Optional([Object]),
      });
      if (!module) {
        return null;
      }
      return module.issueUserToken(Meteor.user(), opts);
    },

    callIssueSysToken(opts) {
      check(opts, {
        subuser: String,
        expiresIn: Number,
        aud: Match.Optional([String]),
        san: Match.Optional([String]),
        scope: Match.Optional(Object),
        scopes: Match.Optional([Object]),
      });
      if (!module) {
        return null;
      }
      return module.issueSysToken(Meteor.user(), opts);
    },

    callEnableDiscoveryPath(opts) {
      check(opts, {
        registryName: String,
        globalRoot: String,
        depth: Number,
        globalPath: String,
      });
      if (!module) {
        return null;
      }
      return module.enableDiscoveryPath(Meteor.user(), opts);
    },
  });
}

export {
  defMethodCalls,
};
