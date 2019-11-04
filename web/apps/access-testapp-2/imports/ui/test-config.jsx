/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';

function Buttons({
  accounts,
  loginWithPassword,
  logout,
}) {
  function handleClickLogInUser(ev) {
    ev.preventDefault();
    const { username, password } = accounts.user;
    loginWithPassword({ username }, password, () => {
      console.log('User logged in.');
    });
  }

  function handleClickLogInGuest(ev) {
    ev.preventDefault();
    const { username, password } = accounts.guest;
    loginWithPassword({ username }, password, () => {
      console.log('Guest logged in.');
    });
  }

  function handleClickLogInAdmin(ev) {
    ev.preventDefault();
    const { username, password } = accounts.admin;
    loginWithPassword({ username }, password, () => {
      console.log('Admin logged in.');
    });
  }

  function handleClickLogOut(ev) {
    ev.preventDefault();
    logout(() => {
      console.log('Logged out.');
    });
  }

  return (
    <Fragment>
      <div className="col-md-8">
        <button
          type="button"
          className="btn btn-primary"
          onClick={handleClickLogInGuest}
        >
          Log in guest
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={handleClickLogInUser}
        >
          Log in user
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={handleClickLogInAdmin}
        >
          Log in admin
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={handleClickLogOut}
        >
          Log out
        </button>
      </div>
    </Fragment>
  );
}

Buttons.propTypes = {
  accounts: PropTypes.object.isRequired,
  loginWithPassword: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
};

function CurrentUserTrackee({
  user,
}) {
  const username = user ? user.username : 'none';
  return (
    <div className="col-md-2">
      <span>
        user:
        {' '}
        {username}
      </span>
    </div>
  );
}

CurrentUserTrackee.propTypes = {
  user: PropTypes.object,
};
CurrentUserTrackee.defaultProps = {
  user: null,
};

function CurrentUserTracker({ currentUser }) {
  return {
    user: currentUser(),
  };
}

const CurrentUser = withTracker(CurrentUserTracker)(CurrentUserTrackee);

CurrentUser.propTypes = {
  currentUser: PropTypes.func.isRequired,
};

function TestConfig({
  accounts,
  loginWithPassword,
  logout,
  currentUser,
}) {
  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h4>Test Options</h4>
        </div>
      </div>
      <div className="row">
        <CurrentUser
          currentUser={currentUser}
        />
        <Buttons
          accounts={accounts}
          loginWithPassword={loginWithPassword}
          logout={logout}
        />
      </div>
    </Fragment>
  );
}

TestConfig.propTypes = {
  accounts: PropTypes.object.isRequired,
  loginWithPassword: PropTypes.func.isRequired,
  logout: PropTypes.func.isRequired,
  currentUser: PropTypes.func.isRequired,
};

export {
  TestConfig,
};
