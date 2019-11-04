/* global document */

import React from 'react';
import { render } from 'react-dom';
import { App } from '../imports/ui';

function renderApp({
  accounts,
  loginWithPassword,
  logout,
  currentUser,
  testAccess,
}) {
  const app = (
    <App
      accounts={accounts}
      loginWithPassword={loginWithPassword}
      logout={logout}
      currentUser={currentUser}
      testAccess={testAccess}
    />
  );
  const el = document.getElementById('react-target');
  render(app, el);
}

export {
  renderApp,
};
