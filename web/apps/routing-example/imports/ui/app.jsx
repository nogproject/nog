/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import {
  BrowserRouter as Router,
  Redirect,
  Route,
  Switch,
} from 'react-router-dom';

import { Layout as LayoutVis } from './vis-layout.jsx';
import { Layout as LayoutBcp } from './bcp-layout.jsx';
import { PageNotFound } from './page-not-found.jsx';

const subapp = {
  bcp: 'bcp',
  vis: 'vis',
};

const routes = {
  home: {
    path: '/',
    to() { return '/'; },
  },

  bcp: {
    path: `/${subapp.bcp}`,
    to() { return `/${subapp.bcp}`; },
  },

  vis: {
    path: `/${subapp.vis}`,
    to() { return `/${subapp.vis}`; },
  },
};

function App({
  fakeTokens,
}) {
  function subAppRedirect() {
    // The logic for redirecting users according to their roles and actions
    // would be here.
    return (
      <Fragment>
        <Redirect to={routes.vis.to()} />
      </Fragment>
    );
  }

  function switchRoutes() {
    return (
      <Switch>
        <Route
          exact
          strict
          path={routes.home.path}
          render={() => subAppRedirect()}
        />
        <Route
          path={routes.bcp.path}
          render={({ match }) => (
            <LayoutBcp
              parentUrl={match.url}
              toVisApp={routes.vis.to}
            />
          )}
        />
        <Route
          path={routes.vis.path}
          render={({ match }) => (
            <LayoutVis
              parentUrl={match.url}
              toBcpApp={routes.bcp.to}
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
    <Router>
      {switchRoutes()}
    </Router>
  );
}

App.propTypes = {
  fakeTokens: PropTypes.object.isRequired,
};

export {
  App,
};
