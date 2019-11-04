import { check, Match } from 'meteor/check';
import * as _ from './underscore.js';

// Restrict subuser to protect agains unreasonable tokens:
//
//  - minimum and maximum length;
//  - limited character set.
//
const matchSubuserName = Match.Where((x) => {
  check(x, String);
  return (x.length > 2) && (x.length < 60) && !!x.match(/^[a-z0-9.+-]+$/);
});

const matchExpiresIn = Match.Where((x) => {
  check(x, Number);
  if (Math.floor(x) !== x) {
    throw new Match.Error('not an integer');
  }
  if (x < 1) {
    throw new Match.Error('not positive');
  }
  if (x > 63 * 24 * 60 * 60) {
    throw new Match.Error('duration longer than 2m');
  }
  return true;
});

const matchOneKnownAudience = Match.Where((x) => {
  check(x, String);
  return x === 'nogapp' || x === 'fso';
});

// Restrict SAN to loose DNS label of limited length to protect against
// unreasonable tokens.
const matchOneSan = Match.Where((x) => {
  check(x, String);
  return (x.length < 60) && !!x.match(/^DNS:[a-zA-Z0-9.-]+$/);
});

const matchSan = [matchOneSan];

// Restrict audience to lowercase alphanum of limited length to protect against
// unreasonable tokens.
const matchOneAudience = Match.Where((x) => {
  check(x, String);
  return (x.length < 30) && !!x.match(/^[a-z0-9]+$/);
});

// eslint-disable-next-line no-unused-vars
const matchSimpleScope = Match.Where((x) => {
  check(x, {
    action: String,
    path: Match.Optional(String),
    name: Match.Optional(String),
  });
  if (!(x.path || x.name)) {
    throw new Match.Error(
      'require at least one of `path` or `name`.',
    );
  }
  return true;
});

const matchScope = Match.Where((x) => {
  check(x, {
    action: Match.Optional(String),
    actions: Match.Optional([String]),
    path: Match.Optional(String),
    paths: Match.Optional([String]),
    name: Match.Optional(String),
    names: Match.Optional([String]),
  });
  if (!(x.action || x.actions)) {
    throw new Match.Error(
      'scope require at least one of `action` or `actions`',
    );
  }
  if (!(x.path || x.paths || x.name || x.names)) {
    throw new Match.Error(
      'scope requires at least one of `path`, `paths`, `name`, or `names`',
    );
  }
  return true;
});

const matchStringNonEmpty = Match.Where((x) => {
  check(x, String);
  if (x.length === 0) {
    throw new Match.Error('require non-empty string');
  }
  return true;
});

// An `XorScope` is a restricted scope representation:
//
//  - either `action` or `actions` but not both;
//  - only one of `name`, `names`, `path`, `paths`;
//  - all strings must be non-empty.
//
// The restrictions should avoid confusion.
const matchXorScope = Match.Where((x) => {
  check(x, {
    action: Match.Optional(matchStringNonEmpty),
    actions: Match.Optional([matchStringNonEmpty]),
    name: Match.Optional(matchStringNonEmpty),
    names: Match.Optional([matchStringNonEmpty]),
    path: Match.Optional(matchStringNonEmpty),
    paths: Match.Optional([matchStringNonEmpty]),
  });
  if (_.size(_.pick(x, 'action', 'actions')) !== 1) {
    throw new Match.Error('require either `action` or `actions`');
  }
  if (_.size(_.pick(x, 'name', 'names', 'path', 'paths')) !== 1) {
    throw new Match.Error(
      'require exactly one of `name`, `names`, `path`, or `paths`',
    );
  }
  return true;
});

const rgxUuid = (
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
);

const matchUuid = Match.Where((x) => {
  check(x, String);
  return !!x.match(rgxUuid);
});

const matchPathNameRepoIdScope = Match.Where((x) => {
  check(x, {
    action: String,
    path: Match.Optional(String),
    name: Match.Optional(String),
    repoId: Match.Optional(matchUuid),
  });
  if (!(x.path || x.name || x.repoId)) {
    throw new Match.Error(
      'require at least one of `path`, `name`, or `repoId`.',
    );
  }
  if (x.path && x.repoId) {
    throw new Match.Error(
      '`path` and `repoId` cannot be used together.',
    );
  }
  return true;
});

export {
  matchExpiresIn,
  matchOneAudience,
  matchOneKnownAudience,
  matchPathNameRepoIdScope,
  matchSan,
  matchScope,
  matchSubuserName,
  matchXorScope,
};
