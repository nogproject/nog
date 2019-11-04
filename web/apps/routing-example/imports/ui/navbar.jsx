/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { Link } from 'react-router-dom';

function Navbar({
  navLinks,
}) {
  return (
    <Fragment>
      <nav className="navbar navbar-expand-md navbar-light bg-light">
        <span className="navbar-brand">Routing Example App</span>
        <button
          className="navbar-toggler"
          type="button"
          data-toggle="collapse"
          data-target="#navbarSupportedContent"
          aria-controls="navbarSupportedContent"
          aria-expanded="false"
          aria-label="Toggle navigation"
        >
          <span className="navbar-toggler-icon" />
        </button>
        <div className="collapse navbar-collapse" id="navbarSupportedContent">
          <ul className="navbar-nav mr-auto">
            {
              navLinks.map(l => (
                <li className="nav-item" key={l.key}>
                  <Link className="nav-link" to={l.to}>
                    {l.name}
                  </Link>
                </li>
              ))
            }
          </ul>
        </div>
      </nav>
    </Fragment>
  );
}

Navbar.propTypes = {
  navLinks: PropTypes.array.isRequired,
};

export {
  Navbar,
};
