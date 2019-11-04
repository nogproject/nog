import { check, Match } from 'meteor/check';
import {
  matchExpiresIn,
  matchScope,
  matchSubuserName,
} from 'meteor/nog-jwt-2';

// `matchSysAbspath()` requires a `/sys/` path with a limited character set.
const matchSysAbspath = Match.Where((x) => {
  check(x, String);
  return (
    x.match(/^[a-z0-9/-]*$/)
    && x.startsWith('/sys/')
  );
});

const matchReadyJwt = Match.Where((x) => {
  check(x, {
    title: String,
    description: String,
    path: matchSysAbspath,
    subuser: matchSubuserName,
    expiresIn: matchExpiresIn,
    scopes: [matchScope],
  });
  return true;
});

export {
  matchReadyJwt,
  matchSysAbspath,
};
