/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import {
  Link,
  BrowserRouter as Router,
  Route,
  Switch,
} from 'react-router-dom';
import { Mongo } from 'meteor/mongo';

import { ManageTokens } from './manage-tokens.jsx';
import { Login } from './login.jsx';
import { PageNotFound } from './page-not-found.jsx';

function AppTrackee({
  appTitle,
  user,
  loginWithGitimp,
  loginWithGitzib,
  logout,
  subscribeReadyJwts,
  readyJwts,
  callIssueToken,
  callDeleteUserToken,
  subscribeUserTokens,
  getUserTokens,
}) {
  const routes = {
    home: {
      path: '/',
      to() { return '/'; },
    },
    // XXX The future path to ManageTokens is not yet clear, it will be located
    // in some sub-app or behind another component like 'Admin', which is
    // currently simulated by `/a/mt`.  The path can later be split according
    // to the future app structure.
    manageTokens: {
      path: '/a/mt',
      to() { return '/a/mt'; },
    },
  };
  const loggedIn = !!user;

  function handleClickLogout(ev) {
    ev.preventDefault();
    logout((err) => {
      // Although `logout` usually should work, we will add some error display
      // when designing the page content.
      if (err) {
        console.error('Failed to log out:', err);
        return;
      }
      console.log('Logged out.');
    });
  }

  function switchRoutes() {
    const { username } = user;
    return (
      <div className="container-fluid">
        <div className="row mt-3">
          <div className="col">
            Logged in as
            {' '}
            {username}
          </div>
          <div className="col">
            <button
              type="button"
              className="btn btn-sm btn-outline-primary float-right"
              onClick={handleClickLogout}
            >
              Logout
            </button>
          </div>
        </div>
        <hr />
        <Switch>
          <Route
            exact
            path={routes.home.path}
            render={() => (
              <ul>
                <li>
                  <Link to={routes.manageTokens.to()}>Manage Tokens</Link>
                </li>
              </ul>
            )}
          />
          <Route
            path={routes.manageTokens.path}
            render={({ match }) => (
              <ManageTokens
                user={user}
                parentUrl={match.url}
                subscribeReadyJwts={subscribeReadyJwts}
                readyJwts={readyJwts}
                callIssueToken={callIssueToken}
                callDeleteUserToken={callDeleteUserToken}
                subscribeUserTokens={subscribeUserTokens}
                getUserTokens={getUserTokens}
              />
            )}
          />
          <Route
            component={PageNotFound}
          />
        </Switch>
      </div>
    );
  }

  function mainView() {
    if (!loggedIn) {
      return (
        <Route
          render={() => (
            <Login
              appTitle={appTitle}
              loginWithGitimp={loginWithGitimp}
              loginWithGitzib={loginWithGitzib}
            />
          )}
        />
      );
    }
    return switchRoutes();
  }

  return (
    <Router>
      {mainView()}
    </Router>
  );
}

AppTrackee.propTypes = {
  appTitle: PropTypes.string.isRequired,
  user: PropTypes.object,
  loginWithGitimp: PropTypes.func.isRequired,
  loginWithGitzib: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
  subscribeReadyJwts: PropTypes.func.isRequired,
  readyJwts: PropTypes.instanceOf(Mongo.Collection).isRequired,
  callIssueToken: PropTypes.func.isRequired,
  callDeleteUserToken: PropTypes.func.isRequired,
  subscribeUserTokens: PropTypes.func.isRequired,
  getUserTokens: PropTypes.func.isRequired,
};
AppTrackee.defaultProps = {
  user: null,
};

function AppTracker({
  appTitle,
  currentUser,
  loginWithGitimp,
  loginWithGitzib,
  logout,
  subscribeReadyJwts,
  readyJwts,
  callIssueToken,
  callDeleteUserToken,
  subscribeUserTokens,
  getUserTokens,
}) {
  const user = currentUser();

  return {
    appTitle,
    user,
    loginWithGitimp,
    loginWithGitzib,
    logout,
    subscribeReadyJwts,
    readyJwts,
    callIssueToken,
    callDeleteUserToken,
    subscribeUserTokens,
    getUserTokens,
  };
}

const App = withTracker(AppTracker)(AppTrackee);

App.propTypes = {
  appTitle: PropTypes.string.isRequired,
  currentUser: PropTypes.func.isRequired,
  loginWithGitimp: PropTypes.func.isRequired,
  loginWithGitzib: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
  subscribeReadyJwts: PropTypes.func.isRequired,
  readyJwts: PropTypes.instanceOf(Mongo.Collection).isRequired,
  callIssueToken: PropTypes.func.isRequired,
  callDeleteUserToken: PropTypes.func.isRequired,
  subscribeUserTokens: PropTypes.func.isRequired,
  getUserTokens: PropTypes.func.isRequired,
};


export {
  App,
};
