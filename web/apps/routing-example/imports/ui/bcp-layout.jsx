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
import { Home } from './bcp-home.jsx';
import { Profile } from './bcp-profile.jsx';

function Layout({
  parentUrl,
  toVisApp,
}) {
  const routes = {
    home: {
      path: `${parentUrl}`,
      to() { return `${parentUrl}`; },
    },
    profile: {
      path: `${parentUrl}/p`,
      to() { return `${parentUrl}/p`; },
    },
  };

  const navLinks = [
    {
      name: 'Home',
      key: 'nav-link-home',
      to: routes.home.to(),
    },
    {
      name: 'Profile',
      key: 'nav-link-profile',
      to: routes.profile.to(),
    },
    {
      name: 'Vis App',
      key: 'nav-link-vis',
      to: toVisApp(),
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
          path={routes.profile.path}
          component={Profile}
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
  toVisApp: PropTypes.func.isRequired,
};

export {
  Layout,
};
