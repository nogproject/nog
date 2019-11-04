/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';

function AppTrackee({
  currentUser,
  loginWithGitimp,
  loginWithGitzib,
  logout,
}) {
  let userId = 'none';
  let username = 'none';
  let profile = 'none';
  if (currentUser) {
    userId = currentUser._id;
    ({ username } = currentUser);
    profile = JSON.stringify(currentUser.profile, null, 2);
  }

  function handleClickSignInGitImp(ev) {
    ev.preventDefault();
    loginWithGitimp((err) => {
      if (err) {
        console.error('Failed to sign in with git.imp:', err);
        return;
      }
      console.log('Signed in with git.imp.');
    });
  }

  function handleClickSignInGitZib(ev) {
    ev.preventDefault();
    loginWithGitzib((err) => {
      if (err) {
        console.error('Failed to sign in with git.zib:', err);
        return;
      }
      console.log('Signed in with git.zib.');
    });
  }

  function handleClickSignOut(ev) {
    ev.preventDefault();
    logout((err) => {
      if (err) {
        console.error('Failed to sign out:', err);
        return;
      }
      console.log('Signed out.');
    });
  }

  return (
    <Fragment>
      <div className="container-fluid">
        <div className="row">
          <div className="col-md-12">
            <h1>oidc-testapp</h1>
            <h2>How to test?</h2>
            <p>
              &ldquo;Sign in with git.imp&rdquo; and/or &ldquo;Sign in with
              git.zib&rdquo;.  Check accounts using
              {' '}
              <code>
                meteor mongo
              </code>
              .
            </p>
            <p>
              Errors are reported to the console.
            </p>
            <p>
              Use SSH tunnels to test LDAP:
            </p>
            <pre>
              <code>
                {[
                  'ssh -L localhost:13389:ldap.imp.fu-berlin.de:389 login.imp',
                  'ssh -L localhost:14389:tyr1.zib.de:389 login.zib',
                ].join('\n')}
              </code>
            </pre>
            <p>
              Use settings
              {' '}
              <code>
                {'"wellknownAccounts": "dev"'}
              </code>
              {' '}
              to test merging multiple login services into one account.
              <br />
              Use settings
              {' '}
              <code>
                {'"wellknownAccounts": []'}
              </code>
              {' '}
              to create a separate account for each login service.
            </p>
          </div>
          <div className="col-md-12">
            <h2>Actions</h2>
            <button
              type="button"
              className="btn btn-primary"
              onClick={handleClickSignInGitImp}
            >
              Sign in with git.imp
            </button>
            <button
              type="button"
              className="btn btn-primary"
              onClick={handleClickSignInGitZib}
            >
              Sign in with git.zib
            </button>
            <button
              type="button"
              className="btn btn-primary"
              onClick={handleClickSignOut}
            >
              Sign out
            </button>
          </div>
          <div className="col-md-12">
            <h2>Current User</h2>
            <p>
              User ID:
              {' '}
              {userId}
            </p>
            <p>
              Username:
              {' '}
              {username}
            </p>
            <pre>
              Profile:
              {' '}
              {profile}
            </pre>
          </div>
        </div>
      </div>
    </Fragment>
  );
}

AppTrackee.propTypes = {
  currentUser: PropTypes.object,
  loginWithGitimp: PropTypes.func.isRequired,
  loginWithGitzib: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
};
AppTrackee.defaultProps = {
  currentUser: null,
};

function AppTracker({
  currentUser,
  loginWithGitimp,
  loginWithGitzib,
  logout,
}) {
  return {
    currentUser: currentUser(),
    loginWithGitimp,
    loginWithGitzib,
    logout,
  };
}

const App = withTracker(AppTracker)(AppTrackee);

App.propTypes = {
  currentUser: PropTypes.func.isRequired,
  loginWithGitimp: PropTypes.func.isRequired,
  loginWithGitzib: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
};

export {
  App,
};
