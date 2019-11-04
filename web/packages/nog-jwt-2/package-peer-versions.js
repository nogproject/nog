/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// Version checks fail under some conditions without these `require`
// statements, which must be explicit toplevel statements.  They cannot be
// generated programmatically, for example from a list of names.  The reason is
// unknown.  It might be related to Meteor 1.5 dynamic imports.  GitHub issue
// <https://github.com/tmeasday/check-npm-versions/issues/11> seems related.
require('jsonwebtoken/package.json');
require('node-forge/package.json');
require('underscore/package.json');
checkNpmVersions(
  {
    'jsonwebtoken': '^8.5.1',
    'node-forge': '^0.8.2',
    'underscore': '^1.9.1',
  },
  'nog-jwt-2',
);
