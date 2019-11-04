import { Meteor } from 'meteor/meteor';
import { check, Match } from 'meteor/check';

const Suggest = {
  TypedItem: 'TypedItem',
  Quantity: 'Quantity',
};

// `CollNameMdProperties` is a collection that contains properties for
// auto-suggesting metadata keys.
const CollNameMdProperties = 'mdProperties';

// `CollNameMdPropertyTypes` is a collection that contains property type
// details for auto-suggesting metadata values.
const CollNameMdPropertyTypes = 'mdPropertyTypes';

// `CollNameMdItems` is a collection that contains items, which are used for
// auto-suggesting metadata values.
const CollNameMdItems = 'mdItems';

function makeCollName(namespace, basename) {
  return `${namespace.coll}.${basename}`;
}

// `defMethodCalls()` registers Meteor methods and binds them to the
// server-side method implementation on `module`.  `module` must be `null` on
// the client.  The common code applies loose checks to catch obvious errors.
// The server methods on `module` must apply the real, stricter checks.
function defMethodCalls(module, { namespace }) {
  if (Meteor.isServer) {
    check(module, Match.ObjectIncluding({
      fetchMetadataProperties: Function,
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
    callFetchMetadataProperties(opts) {
      check(opts, {
        sugnss: [String],
        needle: String,
      });
      if (!module) {
        return null;
      }
      return module.fetchMetadataProperties(Meteor.user(), opts);
    },

    callFetchMetadataPropertyTypes(opts) {
      check(opts, {
        sugnss: [String],
        symbols: [String],
      });
      if (!module) {
        return null;
      }
      return module.fetchMetadataPropertyTypes(Meteor.user(), opts);
    },

    callFetchMetadataItems(opts) {
      check(opts, {
        sugnss: [String],
        ids: [String],
        needle: String,
        ofType: [String],
      });
      if (!module) {
        return null;
      }
      return module.fetchMetadataItems(Meteor.user(), opts);
    },

    callApplyFixedMdFromRepo(opts) {
      check(opts, {
        repo: String,
        mdnss: [String],
      });
      if (!module) {
        return null;
      }
      return module.applyFixedMdFromRepo(Meteor.user(), opts);
    },

    callApplyFixedSuggestionNamespacesFromRepo(opts) {
      check(opts, {
        repo: String,
        mdnss: [String],
        sugnss: [String],
      });
      if (!module) {
        return null;
      }
      return module.applyFixedSuggestionNamespacesFromRepo(
        Meteor.user(), opts,
      );
    },
  });
}

export {
  CollNameMdItems,
  CollNameMdProperties,
  CollNameMdPropertyTypes,
  Suggest,
  defMethodCalls,
  makeCollName,
};
