/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';

const AA_IS_ADMIN = 'isAdmin';

function AdminTrackee({
  isAdminReady, isAdmin,
  detail,
}) {
  function colContent() {
    if (!isAdminReady) {
      return 'loading...';
    }

    if (!isAdmin) {
      return 'Permission denied.';
    }

    if (detail) {
      return `${detail} admin tab.`;
    }
    return 'main admin tab';
  }

  return (
    <div className="row">
      <div className="col-md-12">
        {colContent()}
      </div>
    </div>
  );
}

AdminTrackee.propTypes = {
  isAdminReady: PropTypes.bool.isRequired,
  isAdmin: PropTypes.bool.isRequired,
  detail: PropTypes.string,
};
AdminTrackee.defaultProps = {
  detail: null,
};

function AdminTracker({
  detail,
  testAccess,
}) {
  const isAdmin = testAccess(AA_IS_ADMIN);
  return {
    isAdminReady: (isAdmin !== null),
    isAdmin: !!isAdmin,
    detail,
  };
}

const Admin = withTracker(AdminTracker)(AdminTrackee);

Admin.propTypes = {
  detail: PropTypes.string,
  testAccess: PropTypes.func.isRequired,
};

export {
  Admin,
};
