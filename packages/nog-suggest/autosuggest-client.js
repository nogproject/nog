import { check, Match } from 'meteor/check';
import { Mongo } from 'meteor/mongo';
import { defMethodCalls } from './autosuggest.js';

function createSuggestModuleClient({
  namespace,
}) {
  check(namespace, Match.ObjectIncluding({
    meth: String,
  }));

  // `maxAgeS` limits how long docs are kept in client-side collections.
  const maxAgeS = 10 * 3600;

  // `maxCount` limits how many docs are kept in client-side collections.
  const maxCount = 1000;

  // `mdProperties` is a client-only, unnamed collection.  There are no
  // publications that write to it.  Docs are only inserted via
  // `insertWithExpiry()`.
  const mdProperties = new Mongo.Collection(null);

  // `mdPropertyTypes` is a client-only collection like `mdProperties`.
  const mdPropertyTypes = new Mongo.Collection(null);

  // `mdItems` is a client-only collection like `mdProperties`.
  const mdItems = new Mongo.Collection(null);

  function removeExpired() {
    const now = new Date();
    this.remove({ expiry: { $lt: now } });
  }

  function insertWithExpiry(docs) {
    // First insert, which may update existing docs.
    const expiry = new Date(Date.now() + (maxAgeS * 1000));
    docs.forEach((doc) => {
      this.upsert(doc._id, { expiry, ...doc });
    });

    // Then remove oldest docs if there are too many.
    const n = this.find({}).count();
    if (n > maxCount) {
      this.find({}, {
        sort: { expiry: 1 },
        limit: n - maxCount,
      }).forEach((doc) => {
        this.remove(doc._id);
      });
    }
  }

  mdProperties.removeExpired = removeExpired.bind(mdProperties);
  mdProperties.insertWithExpiry = insertWithExpiry.bind(mdProperties);

  mdPropertyTypes.removeExpired = removeExpired.bind(mdPropertyTypes);
  mdPropertyTypes.insertWithExpiry = insertWithExpiry.bind(mdPropertyTypes);

  mdItems.removeExpired = removeExpired.bind(mdItems);
  mdItems.insertWithExpiry = insertWithExpiry.bind(mdItems);

  const {
    callFetchMetadataProperties,
    callFetchMetadataPropertyTypes,
    callFetchMetadataItems,
    callApplyFixedMdFromRepo,
    callApplyFixedSuggestionNamespacesFromRepo,
  } = defMethodCalls(null, { namespace });

  const module = {
    mdProperties,
    mdPropertyTypes,
    mdItems,
    callFetchMetadataProperties,
    callFetchMetadataPropertyTypes,
    callFetchMetadataItems,
    callApplyFixedMdFromRepo,
    callApplyFixedSuggestionNamespacesFromRepo,
  };
  return module;
}

export {
  createSuggestModuleClient,
};
