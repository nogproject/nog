/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React, { useState } from 'react';
import PropTypes from 'prop-types';
const { Fragment } = React;

function Login({
  appTitle,
  loginWithGitimp,
  loginWithGitzib,
}) {
  const [errorMsg, setErrorMsg] = useState('');

  function handleClickLoginGitimp(ev) {
    ev.preventDefault();
    loginWithGitimp((err) => {
      if (err) {
        setErrorMsg('Login failed.');
        return;
      }
      setErrorMsg('');
    });
  }

  function handleClickLoginGitzib(ev) {
    ev.preventDefault();
    loginWithGitzib((err) => {
      if (err) {
        setErrorMsg('Login failed.');
        return;
      }
      setErrorMsg('');
    });
  }

  function loginError() {
    if (!errorMsg) {
      return null;
    }
    return (
      <div className="alert alert-danger" role="alert">
        {errorMsg}
      </div>
    );
  }

  function noteImp() {
    return (
      <div>
        Log in with your ZEDAT account.
        Your ZEDAT account must be registered at the GitLab service of the
        Department of Mathematics and Computer Science:
        {' '}
        <a href="https://git.imp.fu-berlin.de">https://git.imp.fu-berlin.de</a>
      </div>
    );
  }

  function noteZib() {
    return (
      <div>
        Log in with your ZIB account.
        Your ZIB account must be registered at
        {`ZIB's`}
        GitLab service:
        {' '}
        <a href="https://git.zib.de">https://git.zib.de</a>
      </div>
    );
  }

  return (
    <Fragment>
      <div className="container-fluid">
        <div className="row">
          <div className="col">
            <h1 className="text-center">
              Login to
              {' '}
              {appTitle}
            </h1>
          </div>
        </div>
        <div className="row">
          <div className="col-lg-11 mx-auto">
            {loginError()}
          </div>
          <div className="col-lg-5 mx-auto">
            <h2> As a BioSupraMol User </h2>
            <button
              type="button"
              className="btn btn-outline-primary btn-block mb-2"
              onClick={handleClickLoginGitimp}
            >
              Log in with ZEDAT Account
            </button>
            {noteImp()}
          </div>
          <div className="col-lg-5 mx-auto">
            <h2> As a ZIB Member </h2>
            <button
              type="button"
              className="btn btn-outline-primary btn-block mb-2"
              onClick={handleClickLoginGitzib}
            >
              Log in with ZIB Account
            </button>
            {noteZib()}
          </div>
        </div>
      </div>
    </Fragment>
  );
}

Login.propTypes = {
  appTitle: PropTypes.string.isRequired,
  loginWithGitimp: PropTypes.func.isRequired,
  loginWithGitzib: PropTypes.func.isRequired,
};

export {
  Login,
};
