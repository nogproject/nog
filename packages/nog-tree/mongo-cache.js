import { Mongo } from 'meteor/mongo';
import { Meteor } from 'meteor/meteor';

const optGlobalReadOnly = Meteor.settings.optGlobalReadOnly;


// `createMongoCache()` returns a cache that uses a MongoDB collection `name`
// with a TTL `expireAfterSeconds`.
//
// Use the cache via `add(key, entry)` and `get(key)`.
//
// The TTL index is created if necessary.  A mismatching TTL of an existing
// index is reported, but the index is not automatically updated.
function createMongoCache({ name, expireAfterSeconds }) {
  const collection = new Mongo.Collection(
    name, { defineMutationMethods: false },
  );

  const indexName = 'ts_ttl';
  const raw = collection.rawCollection();
  if (optGlobalReadOnly) {
    console.log(
      `[nog] [GRO] Skipped creating TTL index on collection \`${name}\` ` +
      `in read-only mode.`
    );
  } else {
    raw.createIndex(
      { ts: 1 },
      { name: indexName, expireAfterSeconds },
    );
  }

  raw.indexes((err, res) => {
    for (const idx of res) {
      if (idx.name !== indexName) {
        continue;
      }
      if (idx.expireAfterSeconds !== expireAfterSeconds) {
        console.log(
          `Warning: Wrong TTL on collection '${name}' index '${indexName}'; ` +
          `expected ${expireAfterSeconds}s, got ${idx.expireAfterSeconds}s. ` +
          `You could drop the index manually and restart the application.`,
        );
      }
    }
  });

  return {
    collection,

    add(key, entry) {
      this.collection.upsert(
        key,
        {
          $set: { entry },
          $currentDate: { ts: true },
        },
      );
    },

    get(key) {
      const e = this.collection.findOne(key);
      if (!e) {
        return null;
      }
      return e.entry;
    },
  };
}


export { createMongoCache };
