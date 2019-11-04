import React from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router-dom';

function NavBar({
  toHome,
}) {
  return (
    <div className="navbar navbar-default" role="navigation">
      <div className="container-fluid">
        <div className="navbar-header">
          <Link className="navbar-brand" to={toHome()}>Home</Link>
        </div>
      </div>
    </div>
  );
}

NavBar.propTypes = {
  toHome: PropTypes.func.isRequired,
};

export {
  NavBar,
};
