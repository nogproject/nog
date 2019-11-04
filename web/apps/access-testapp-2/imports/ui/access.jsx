/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import { Link } from 'react-router-dom';

const AA_IS_ADMIN = 'isAdmin';
const AA_IS_USER = 'isUser';

function AccessTableTrackee({
  toAdmin, toProfile,
  isAdminReady, isAdmin,
  isUserReady, isUser,
}) {
  function adminLink() {
    if (!isAdminReady) {
      return 'loading...';
    }
    if (!isAdmin) {
      return 'no';
    }
    return (
      <Fragment>
        yes:
        {' '}
        <Link to={toAdmin()}>main admin tab</Link>
        {', '}
        <Link to={toAdmin({ detail: 'foo' })}>foo admin tab</Link>
      </Fragment>
    );
  }

  function userLink() {
    if (!isUserReady) {
      return 'loading...';
    }
    if (!isUser) {
      return 'no';
    }
    return (
      <Fragment>
        yes:
        {' '}
        <Link to={toProfile()}>profile</Link>
      </Fragment>
    );
  }

  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h4>Access</h4>
        </div>
      </div>
      <div className="row">
        <div className="col-md-12">
          <p>
            isAdmin:
            {' '}
            {adminLink()}
          </p>
          <p>
            isUser:
            {' '}
            {userLink()}
          </p>
        </div>
      </div>
    </Fragment>
  );
}

AccessTableTrackee.propTypes = {
  toAdmin: PropTypes.func.isRequired,
  toProfile: PropTypes.func.isRequired,
  isAdminReady: PropTypes.bool.isRequired,
  isAdmin: PropTypes.bool.isRequired,
  isUserReady: PropTypes.bool.isRequired,
  isUser: PropTypes.bool.isRequired,
};

function AccessTableTracker({
  toAdmin,
  toProfile,
  testAccess,
}) {
  const isAdmin = testAccess(AA_IS_ADMIN);
  const isUser = testAccess(AA_IS_USER);
  return {
    toAdmin,
    toProfile,
    isAdminReady: (isAdmin !== null),
    isAdmin: !!isAdmin,
    isUserReady: (isUser !== null),
    isUser: !!isUser,
  };
}

const AccessTable = withTracker(AccessTableTracker)(AccessTableTrackee);

AccessTable.propTypes = {
  toAdmin: PropTypes.func.isRequired,
  toProfile: PropTypes.func.isRequired,
  testAccess: PropTypes.func.isRequired,
};

export {
  AccessTable,
};
