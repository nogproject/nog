import { check, Match } from 'meteor/check';
import { _ } from 'meteor/underscore';


const matchCheckSettingsToggle = Match.Where((x) => {
  check(x, Match.ObjectIncluding({ check: String }));
  return x.check === 'toggle';
});


const matchCheckSettingsHealthy = Match.Where((x) => {
  check(x, Match.ObjectIncluding({ check: String }));
  return x.check === 'healthy';
});


const matchCheckSettingsUnhealthy = Match.Where((x) => {
  check(x, Match.ObjectIncluding({ check: String }));
  return x.check === 'unhealthy';
});


const matchCheckSettingsRandom = Match.Where((x) => {
  check(x, Match.ObjectIncluding({
    check: String,
    checkFailureProb: Number,
  }));
  if (x.checkFailureProb < 0 || x.checkFailureProb > 1) {
    throw new Match.Error('checkFailureProb out of range [0, 1]');
  }
  return x.check === 'random';
});


const matchInterval = Match.Where((x) => {
  check(x, String);
  return x.match(/^\d+s/);
});


const matchCheckSettingsGetObject = Match.Where((x) => {
  check(x, Match.ObjectIncluding({
    check: String,
    checkKey: String,
    checkContent: String,
    checkInterval: matchInterval,
  }));
  return x.check === 'getObject';
});


const matchHealthCheckSettings = Match.Where((x) => {
  check(x.check, Match.OneOf(String, undefined));
  if (!x.check) {
    return true;
  }
  const checkmap = {
    healthy: matchCheckSettingsHealthy,
    unhealthy: matchCheckSettingsUnhealthy,
    toggle: matchCheckSettingsToggle,
    random: matchCheckSettingsRandom,
    getObject: matchCheckSettingsGetObject,
  };
  if (!_.contains(_.keys(checkmap), x.check)) {
    throw new Match.Error(`unknown check type '${x.check}'`);
  }
  check(x, checkmap[x.check]);
  return true;
});


const matchBucketSettingsRegion = Match.Where((x) => {
  check(x, Match.ObjectIncluding({
    name: String,
    accessKeyId: String,
    secretAccessKey: String,
    region: String,
  }));
  return !_.has(x, 'endpoint');
});

const matchSignatureVersion =  Match.Where((x) => {
  check(x, String);
  return !!x.match(/^v(2|4)$/);
});

const matchBucketSettingsEndpoint = Match.Where((x) => {
  check(x, Match.ObjectIncluding({
    name: String,
    accessKeyId: String,
    secretAccessKey: String,
    endpoint: String,
    signatureVersion: Match.Optional(matchSignatureVersion),
  }));
  return !_.has(x, 'region');
});


const matchBucketSettings = Match.Where((x) => {
  check(x, Match.OneOf(
    matchBucketSettingsRegion,
    matchBucketSettingsEndpoint
  ));
  check(x, matchHealthCheckSettings);
  return true;
});


const matchMultiBucketSettings = Match.Where((x) => {
  check(x, Match.ObjectIncluding({
    readPrefs: [String],
    writePrefs: [String],
    fallback: String,
    buckets: [matchBucketSettings],
  }));

  const buckets = x.buckets.map((b) => b.name);
  for (const rp of x.readPrefs) {
    if (!_.contains(buckets, rp)) {
      throw new Match.Error(`readPrefs '${rp}' not in buckets.`);
    }
  }
  for (const wp of x.writePrefs) {
    if (!_.contains(buckets, wp)) {
      throw new Match.Error(`writePrefs '${wp}' not in buckets.`);
    }
  }
  if (!_.contains(buckets, x.fallback)) {
    throw new Match.Error(`fallback '${x.fallback}' not in buckets.`);
  }

  return true;
});

export { matchMultiBucketSettings, matchInterval };
