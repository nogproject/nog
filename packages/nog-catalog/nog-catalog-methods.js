import { _ } from 'meteor/underscore';
import { check, Match } from 'meteor/check';
import { EJSON } from 'meteor/ejson';
import { Meteor } from 'meteor/meteor';

// Loose check here.  Stricter check in server code.
const matchUuidStringOrBinary = Match.Where((x) => {
  check(x, Match.OneOf(String, Match.Where(EJSON.isBinary)));
  return true;
});

/* eslint-disable no-param-reassign */
function defCatalogMethods({
  namespace, mod,
}) {
  function defOne(basename, func) {
    const qualname = `${namespace.meth}.${basename}`;
    Meteor.methods({ [qualname]: func });
    mod[basename] = (...args) => Meteor.call(qualname, ...args);
  }

  function defMany(methods) {
    for (const [name, fn] of _.pairs(methods)) {
      defOne(name, fn);
    }
  }

  // The common code applies only loose checks to catch obvious errors.  The
  // server-only code applies the real, stricter checks.

  defMany({
    callUpdateCatalog(opts) {
      check(opts, {
        ownerName: String,
        repoName: String,
      });
      if (!Meteor.isServer) {
        return null;
      }
      return mod.updateCatalog(Meteor.user(), opts);
    },

    callUpdateCatalogFso(opts) {
      check(opts, {
        repoPath: String,
        selectRepos: Match.Optional([matchUuidStringOrBinary]),
      });
      if (!Meteor.isServer) {
        return null;
      }
      return mod.updateCatalogFso(Meteor.user(), opts);
    },

    callConfigureCatalog(opts) {
      check(opts, {
        ownerName: String,
        repoName: String,
        catalogConfig: Object,
      });
      if (!Meteor.isServer) {
        return null;
      }
      return mod.configureCatalog(Meteor.user(), opts);
    },

    callUpdateSuggestionsFromCatalog(opts) {
      check(opts, {
        repoPath: String,
        mdNamespace: String,
        sugNamespaces: [String],
      });
      if (!Meteor.isServer) {
        return null;
      }
      return mod.updateSuggestionsFromCatalog(Meteor.user(), opts);
    },
  });
}


export { defCatalogMethods };
