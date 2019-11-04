/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import {
  Route,
  Switch,
} from 'react-router-dom';

import { Navbar } from './navbar.jsx';
import { PageNotFound } from './page-not-found.jsx';
import { Home } from './vis-home.jsx';
import { Settings } from './vis-settings.jsx';

function Layout({
  parentUrl,
  toBcpApp,
  fakeTokens,
}) {
  const routes = {
    home: {
      path: `${parentUrl}`,
      to() { return `${parentUrl}`; },
    },
    settings: {
      path: `${parentUrl}/s`,
      to() { return `${parentUrl}/s`; },
    },
  };

  const navLinks = [
    {
      name: 'Home',
      key: 'nav-link-home',
      to: routes.home.to(),
    },
    {
      name: 'Settings',
      key: 'nav-link-settigns',
      to: routes.settings.to(),
    },
    {
      name: 'BCP App',
      key: 'nav-link-bcp',
      to: toBcpApp(),
    },
  ];

  function switchRoutes() {
    return (
      <Switch>
        <Route
          exact
          path={routes.home.path}
          component={Home}
        />
        <Route
          path={routes.settings.path}
          render={({ match }) => (
            <Settings
              parentUrl={match.url}
              fakeTokens={fakeTokens}
            />
          )}
        />
        <Route
          component={PageNotFound}
        />
      </Switch>
    );
  }

  return (
    <Fragment>
      <Navbar
        navLinks={navLinks}
      />
      <div className="container-fluid">
        <div className="row">
          <div className="col-12">
            {switchRoutes()}
          </div>
        </div>
      </div>
    </Fragment>
  );
}

Layout.propTypes = {
  parentUrl: PropTypes.string.isRequired,
  toBcpApp: PropTypes.func.isRequired,
  fakeTokens: PropTypes.object.isRequired,
};

export {
  Layout,
};
