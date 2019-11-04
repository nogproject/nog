/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// `meteor npm run test-headless` fails without these explicit imports for
// unknown reason, although `meteor npm start` works.  The problem might be
// related to Meteor 1.5 dynamic imports.
require('path-to-regexp');
checkNpmVersions(
  {
    'path-to-regexp': '^3.0.0',
  },
  'nog-catalog-fso',
);
