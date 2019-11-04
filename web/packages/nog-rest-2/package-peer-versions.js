/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// Version checks fail under some conditions without these `require`
// statements, which must be explicit toplevel statements.  They cannot be
// generated programmatically, for example from a list of names.  The reason is
// unknown.  It might be related to Meteor 1.5 dynamic imports.  GitHub issue
// <https://github.com/tmeasday/check-npm-versions/issues/11> seems related.
require('body-parser/package.json');
require('connect/package.json');
require('path-to-regexp/package.json');
require('underscore/package.json');
checkNpmVersions(
  {
    'body-parser': '^1.18.3',
    'connect': '^3.6.6',
    'path-to-regexp': '^3.0.0',
    'underscore': '^1.9.1',
  },
  'nog-rest-2',
);
