import { Match, check } from 'meteor/check';
import { defSetting } from 'meteor/nog-settings';


// XXX `matchMongoIndexStringLengthLimit` has been duplicated from
// `nog-app/.../01-settings.js`.  It should be factored out to nog-settings.

const matchMongoIndexStringLengthLimit = Match.Where((x) => {
  check(x, Number);
  const min = 50;
  const max = 1000;
  if (x < min) {
    throw new Match.Error(
      `Expected number >= ${min}; got ${x}. ` +
      'The limit must be sufficiently large to leave space for the ' +
      'truncation indicator.',
    );
  }
  if (x > max) {
    throw new Match.Error(
      `Expected number <= ${max}; got ${x}. ` +
      'The limit must be smaller than the MongoDB limit minus safety margin.',
    );
  }
  return true;
});


const matchMongoNumberOfDataIndexesLimit = Match.Where((x) => {
  check(x, Number);
  const min = 0;
  const max = 50;
  if (x < min) {
    throw new Match.Error(`Expected non-negative number; got ${x}.`);
  }
  if (x > max) {
    throw new Match.Error(
      `Expected number <= ${max}; got ${x}. ` +
      'The limit on flexible data indexes must be smaller than the maximum ' +
      'number of MongoDB indexes minus a margin that is preserved for fixed ' +
      'indexes.',
    );
  }
  return true;
});


defSetting({
  key: 'catalogMaxStringLength',
  val: 500,
  help: `
\`catalogMaxStringLength\` limits the length of meta value strings in catalog
entries.  Longer strings will be truncated to satisfy the MongoDB limit on
index keys, which must be smaller than 1024 including BSON overhead; see
<https://docs.mongodb.com/manual/reference/limits/#indexes>.
`,
  match: matchMongoIndexStringLengthLimit,
});

defSetting({
  key: 'catalogMaxNumMetaIndexes',
  val: 32,
  help: `
\`catalogMaxNumMetaIndexes\` limits the number of meta fields for which MongoDB
indexes are created.  Indexes are first created for the \`preferredMetaKeys\`
specified in the catalog config and then for meta fields that were discovered
in the content entries until the maximum number of indexes is reached.  MongoDB
supports up to 64 indexes per collection
<https://docs.mongodb.com/manual/reference/limits/#Number-of-Indexes-per-Collection>.
A few indexes are needed for other purposes and are not available for meta
fields.
`,
  match: matchMongoNumberOfDataIndexesLimit,
});


defSetting({
  key: 'catalogUpdateReadRateLimit',
  val: 100,
  help: `
\`catalogUpdateReadRateLimit\` restricts the rate of database read
operations for the background catalog update.  The limit is specified for
application ops, which may represent multiple database ops.
`,
  match: Number,
});

defSetting({
  key: 'catalogUpdateWriteRateLimit',
  val: 100,
  help: `
\`catalogUpdateWriteRateLimit\` restricts the rate of database write
operations for the background catalog update.  The limit is specified for
application ops, which may represent multiple database ops.
`,
  match: Number,
});
