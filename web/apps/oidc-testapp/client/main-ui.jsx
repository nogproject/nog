/* global document */

import React from 'react';
import { render } from 'react-dom';
import { App } from '../imports/ui';

function renderApp({
  currentUser,
  loginWithGitimp,
  loginWithGitzib,
  logout,
}) {
  const app = (
    <App
      currentUser={currentUser}
      loginWithGitimp={loginWithGitimp}
      loginWithGitzib={loginWithGitzib}
      logout={logout}
    />
  );
  const el = document.getElementById('react-target');
  render(app, el);
}

export {
  renderApp,
};
