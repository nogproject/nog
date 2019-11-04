/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';

function Home() {
  return (
    <div className="row">
      <div className="col-md-12">
        <p>
          Use the buttons below &ldquo;Test Options&rdquo; to log in as
          different users.  Observe how the permission below
          &ldquo;Access&rdquo; change.  Follow links that become available, for
          example, as an admin.
        </p>
      </div>
    </div>
  );
}

export {
  Home,
};
