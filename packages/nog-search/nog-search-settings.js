import { check, Match } from 'meteor/check';
import { defSetting } from 'meteor/nog-settings';


const matchMongoIndexStringLengthLimit = Match.Where(x => {
  check(x, Number);
  const min = 50;
  const max = 1000;
  if (x < min) {
    throw new Match.Error(
      `Expected number >= ${min}; got ${x}. ` +
      'The limit must be sufficiently large to leave space for the ' +
      'truncation indicator.'
    );
  }
  if (x > max) {
    throw new Match.Error(
      `Expected number <= ${max}; got ${x}. ` +
      'The limit must be smaller than the MongoDB limit minus a safety margin.'
    );
  }
  return true;
});

defSetting({
  key: 'searchIndexMaxStringLength',
  val: 500,
  help: `
\`searchIndexMaxStringLength\` limits the length of meta value strings in the
search index.  Longer strings will be truncated to satisfy the MongoDB limit
on index keys, which must be smaller than 1024 including BSON overhead; see
<https://docs.mongodb.com/manual/reference/limits/#indexes>.
`,
  match: matchMongoIndexStringLengthLimit,
});

defSetting({
  key: 'searchIndexUpdateReadRateLimit',
  val: 150,
  help: `
\`searchIndexUpdateReadRateLimit\` restricts the rate of database read
operations for the background search index update.  The limit is specified for
application ops, which may represent multiple database ops.
`,
  match: Number,
});

defSetting({
  key: 'searchIndexUpdateWriteRateLimit',
  val: 40,
  help: `
\`searchIndexUpdateWriteRateLimit\` restricts the rate of database write
operations for the background search index update.  The limit is specified for
application ops, which may represent multiple database ops.
`,
  match: Number,
});
