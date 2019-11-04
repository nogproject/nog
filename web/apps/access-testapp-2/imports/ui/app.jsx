/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import {
  BrowserRouter as Router, Switch, Route,
} from 'react-router-dom';

import { NavBar } from './nav.jsx';
import { TestConfig } from './test-config.jsx';
import { AccessTable } from './access.jsx';
import { Profile } from './profile.jsx';
import { Admin } from './admin.jsx';
import { Home } from './home.jsx';

const AA_IS_USER = 'isUser';
const AA_IS_GUEST = 'isGuest';

function AppTitle() {
  return (
    <div className="row">
      <div className="col-md-12">
        <h1>access-testapp-2</h1>
      </div>
    </div>
  );
}

function SignIn() {
  return (
    <div className="row">
      <div className="col-md-12">
        <p>You need to sign in to do anything relevant.</p>
        <p>
          If you do not have an account, sign up, and ask an admin to confirm
          your new account.
        </p>
      </div>
    </div>
  );
}

function NotFound() {
  return (
    <div className="row">
      <div className="col-md-12">
        <p>Unknown path.</p>
      </div>
    </div>
  );
}

const routes = {
  home: {
    path: '/',
    to() { return '/'; },
  },

  admin: {
    path: '/admin/:detail?',
    to({ detail } = {}) {
      if (detail) {
        return `/admin/${detail}`;
      }
      return '/admin';
    },
  },

  profile: {
    path: '/profile',
    to() { return '/profile'; },
  },
};

function AppTrackee({
  accounts,
  loginWithPassword,
  logout,
  currentUser,
  testAccess,
  isUserOrGuest,
}) {
  function switchRoutes() {
    return (
      <Switch>
        <Route
          exact
          path={routes.home.path}
          component={Home}
        />
        <Route
          exact
          path={routes.admin.path}
          render={({ match }) => (
            <Admin
              detail={match.params.detail}
              testAccess={testAccess}
            />
          )}
        />
        <Route
          exact
          path={routes.profile.path}
          render={() => (
            <Profile
              testAccess={testAccess}
              currentUser={currentUser}
            />
          )}
        />
        <Route component={NotFound} />
      </Switch>
    );
  }

  function mainView() {
    if (!isUserOrGuest) {
      return (
        <SignIn />
      );
    }
    return switchRoutes();
  }

  return (
    <Router>
      <Fragment>
        <NavBar
          toHome={routes.home.to}
        />
        <div className="container-fluid">
          <AppTitle />
          <TestConfig
            accounts={accounts}
            loginWithPassword={loginWithPassword}
            logout={logout}
            currentUser={currentUser}
          />
          <AccessTable
            testAccess={testAccess}
            toAdmin={routes.admin.to}
            toProfile={routes.profile.to}
          />
        </div>

        <div className="container-fluid">
          <div className="row">
            <div className="col-md-12">
              <h4>Route View</h4>
            </div>
          </div>
          {mainView()}
        </div>
      </Fragment>
    </Router>
  );
}

AppTrackee.propTypes = {
  accounts: PropTypes.object.isRequired,
  loginWithPassword: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
  currentUser: PropTypes.func.isRequired,
  testAccess: PropTypes.func.isRequired,
  isUserOrGuest: PropTypes.bool.isRequired,
};

function AppTracker({
  accounts,
  loginWithPassword,
  logout,
  currentUser,
  testAccess,
}) {
  const isUser = testAccess(AA_IS_USER);
  const isGuest = testAccess(AA_IS_GUEST);
  const isUserOrGuest = !!(isUser || isGuest);
  return {
    accounts,
    loginWithPassword,
    logout,
    currentUser,
    testAccess,
    isUserOrGuest,
  };
}

const App = withTracker(AppTracker)(AppTrackee);

App.propTypes = {
  accounts: PropTypes.object.isRequired,
  loginWithPassword: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
  currentUser: PropTypes.func.isRequired,
  testAccess: PropTypes.func.isRequired,
};

export {
  App,
};
