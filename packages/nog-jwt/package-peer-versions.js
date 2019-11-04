/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// `meteor npm run test-headless` fails without these explicit imports,
// although `meteor npm start` works.  The reason is unknown.  Maybe it is
// related to Meteor 1.5 dynamic imports.
require('jsonwebtoken');
require('node-forge');
checkNpmVersions(
  {
    'jsonwebtoken': '^8.1.0',
    'node-forge': '^0.7.1',
  },
  'nog-jwt',
);
