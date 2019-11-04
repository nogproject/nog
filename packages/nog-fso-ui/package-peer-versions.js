/* eslint-disable quote-props */
import { checkNpmVersions } from 'meteor/tmeasday:check-npm-versions';

// `meteor npm run test-headless` fails without these explicit imports,
// although `meteor npm start` works.  The reason is unknown.  Maybe it is
// related to Meteor 1.5 dynamic imports.
import 'markdown-toc';
import 'marked';
import 'prop-types';
import 'react';
import 'react-autosuggest';
import 'sanitize-html';
import 'highlight.js';
checkNpmVersions(
  {
    'prop-types': '^15.6.0',
    'react': '^16.2.0',
    'react-autosuggest': '^9.3.2',

    // Used in `./markdown.jsx`.
    'highlight.js': '^9.12.0',
    'markdown-toc': '^1.2.0',
    'marked': '^0.6.1',
    'sanitize-html': '^1.18.2',
  },
  'nog-fso-ui',
);
