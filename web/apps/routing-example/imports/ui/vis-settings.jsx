/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
import PropTypes from 'prop-types';
import {
  Link,
  Route,
  Switch,
} from 'react-router-dom';

import { PageNotFound } from './page-not-found.jsx';
import { ManageTokens } from './manage-tokens.jsx';

function Settings({
  parentUrl,
  fakeTokens,
}) {
  const routes = {
    home: {
      path: `${parentUrl}`,
      to() { return `${parentUrl}`; },
    },
    manageTokens: {
      path: `${parentUrl}/mt`,
      to() { return `${parentUrl}/mt`; },
    },
    otherSettings: {
      path: `${parentUrl}/misc`,
      to() { return `${parentUrl}/misc`; },
    },
  };

  function switchRoutes() {
    return (
      <Switch>
        <Route
          exact
          path={routes.home.path}
          render={() => <p>Choose a setting above</p>}
        />
        <Route
          path={routes.manageTokens.path}
          render={({ match }) => (
            <ManageTokens
              parentUrl={match.url}
              fakeTokens={fakeTokens}
            />
          )}
        />
        <Route
          path={routes.otherSettings.path}
          render={() => <h1>Other settings</h1>}
        />
        <Route
          component={PageNotFound}
        />
      </Switch>
    );
  }

  return (
    <div className="row">
      <div className="col-12">
        <h1>Vis Settings View</h1>
        <ul>
          <li>
            <Link to={routes.manageTokens.to()}>Manage Tokens</Link>
          </li>
          <li>
            <Link to={routes.otherSettings.to()}>Other Settings</Link>
          </li>
        </ul>
        {switchRoutes()}
      </div>
    </div>
  );
}

Settings.propTypes = {
  parentUrl: PropTypes.string.isRequired,
  fakeTokens: PropTypes.object.isRequired,
};

export {
  Settings,
};
