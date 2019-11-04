/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// `meteor npm run test-headless` fails without these explicit imports,
// although `meteor npm start` works.  The reason is unknown.  Maybe it is
// related to Meteor 1.5 dynamic imports.
import 'prop-types';
import 'react';
checkNpmVersions(
  {
    'prop-types': '^15.6.0',
    'react': '^16.2.0',
  },
  'nog-home-v2',
);
