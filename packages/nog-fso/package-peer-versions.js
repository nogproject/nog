/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// Version checks fail under some conditions without these `require`
// statements, which must be explicit toplevel statements.  They cannot be
// generated programmatically, for example from a list of names.  The reason is
// unknown.  It might be related to Meteor 1.5 dynamic imports.  GitHub issue
// <https://github.com/tmeasday/check-npm-versions/issues/11> seems related.
require('protobufjs/package.json');
require('grpc/package.json');
require('moment/package.json');
require('path-to-regexp/package.json');
require('jsonwebtoken/package.json');
require('node-forge/package.json');
checkNpmVersions(
  {
    'grpc': '^1.6.0',
    'jsonwebtoken': '^8.1.0',
    'moment': '^2.19.2',
    'node-forge': '^0.8.1',
    'path-to-regexp': '^3.0.0',
    'protobufjs': '^6.8.0',
  },
  'nog-fso',
);
