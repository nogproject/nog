/* global document */

// Import Bootstrap once during main UI initialization.  All other components
// assume that Bootstrap is implicitly available.  Both JavaScript and CSS must
// be explicitly imported.  The JavaScript import implicitly imports the NPM
// peer dependencies `jquery` and `popper.js`.  The Meteor package `jquery` is
// not required and should not be used anymore; see Meteor HISTORY.md.
import 'bootstrap';
import 'bootstrap/dist/css/bootstrap.css';

import React from 'react';
import { render } from 'react-dom';
import { App } from '../imports/ui';

function renderApp({
  fakeTokens,
}) {
  const app = (
    <App
      fakeTokens={fakeTokens}
    />
  );
  const el = document.getElementById('react-target');
  render(app, el);
}

export {
  renderApp,
};
