/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';

const AA_IS_USER = 'isUser';
const AA_IS_GUEST = 'isGuest';

function ProfileTrackee({
  isUser, isGuest, user,
}) {
  function colContent() {
    if (!user) {
      return (
        <Fragment>
          <p>Log in to access your profile.</p>
        </Fragment>
      );
    }

    const { username } = user;

    if (isGuest) {
      return (
        <Fragment>
          <h4>
            Guest Profile
            {' '}
            {username}
          </h4>
          <p>Wait for an admin to confirm your account.</p>
        </Fragment>
      );
    }

    if (isUser) {
      return (
        <Fragment>
          <h4>
            User Profile
            {' '}
            {username}
          </h4>
          <p>Details</p>
        </Fragment>
      );
    }

    return (
      <Fragment>
        <p>Log in to access your profile.</p>
      </Fragment>
    );
  }

  return (
    <div className="row">
      <div className="col-md-12">
        {colContent()}
      </div>
    </div>
  );
}

ProfileTrackee.propTypes = {
  isUser: PropTypes.bool.isRequired,
  isGuest: PropTypes.bool.isRequired,
  user: PropTypes.object,
};
ProfileTrackee.defaultProps = {
  user: null,
};

function ProfileTracker({
  testAccess,
  currentUser,
}) {
  const isUser = !!testAccess(AA_IS_USER);
  const isGuest = !!testAccess(AA_IS_GUEST);
  const user = currentUser();
  return {
    isUser, isGuest,
    user,
  };
}

const Profile = withTracker(ProfileTracker)(ProfileTrackee);

Profile.propTypes = {
  testAccess: PropTypes.func.isRequired,
  currentUser: PropTypes.func.isRequired,
};

export {
  Profile,
};
