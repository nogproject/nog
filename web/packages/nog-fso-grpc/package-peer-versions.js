/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// Version checks fail under some conditions without these `require`
// statements, which must be explicit toplevel statements.  They cannot be
// generated programmatically, for example from a list of names.  The reason is
// unknown.  It might be related to Meteor 1.5 dynamic imports.  GitHub issue
// <https://github.com/tmeasday/check-npm-versions/issues/11> seems related.
require('protobufjs/package.json');
require('grpc/package.json');
require('underscore/package.json');
checkNpmVersions(
  {
    'grpc': '^1.19.0',
    'protobufjs': '^6.8.8',
    'underscore': '^1.9.1',
  },
  'nog-fso-grpc',
);
