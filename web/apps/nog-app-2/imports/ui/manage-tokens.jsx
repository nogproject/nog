/* global document */
/* eslint-disable react/forbid-prop-types */ // Allow `PropTypes.object`.

import React from 'react';
const {
  Fragment,
  useEffect,
  useState,
  useRef,
} = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import { Mongo } from 'meteor/mongo';
import {
  Link,
  Route,
} from 'react-router-dom';
import moment from 'moment';

function ErrorAlert({
  message,
  details,
}) {
  return (
    <div className="row mt-3">
      <div className="col-md">
        <div className="alert alert-danger" role="alert">
          <strong>{message}</strong>
          {' '}
          {details}
        </div>
      </div>
    </div>
  );
}

ErrorAlert.propTypes = {
  message: PropTypes.string.isRequired,
  details: PropTypes.string.isRequired,
};

function TokenDetailsTrackee({
  isTokenConf,
  id,
  title,
  path,
  description,
  callIssueToken,
}) {
  if (!isTokenConf) {
    return (
      <Fragment>
        No valid token found.
      </Fragment>
    );
  }

  const [createdToken, setCreatedToken] = useState(null);
  const [issueError, setIssueError] = useState(null);
  const tokenInputRef = useRef(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const tokenDivRef = useRef(null);

  const token = (createdToken ? createdToken.token : null);
  const expirationTime = (
    createdToken ? createdToken.expirationTime.toString() : null
  );

  useEffect(() => {
    setCreatedToken(null);
    setIssueError(null);
  }, [id]);

  function handleClickCreateToken(ev) {
    ev.preventDefault();
    const name = tokenInputRef.current.value;
    callIssueToken({ path, name }, (err, res) => {
      if (err) {
        setIssueError(err);
        setCreatedToken(null);
        setTokenCopied(false);
        return;
      }
      setCreatedToken(res);
      setIssueError(null);
      setTokenCopied(false);
    });
  }

  function handleClickCopyToken(ev) {
    ev.preventDefault();

    // We display the token in a <div> for design reasons and use an invisible
    // textarea for selection and clipboard-copy, since <div> has no selection
    // functionality.
    const { textContent } = tokenDivRef.current;
    const textArea = document.createElement('textarea');
    textArea.value = textContent;
    textArea.setAttribute('readonly', '');
    textArea.style = 'position: absolute; left: -1000px; top: -1000px';
    document.body.appendChild(textArea);
    textArea.select();
    const success = document.execCommand('copy');
    setTokenCopied(success);
    document.body.removeChild(textArea);
  }

  function handleClickCloseToken(ev) {
    ev.preventDefault();
    setCreatedToken(null);
  }

  function displayError() {
    if (issueError) {
      return (
        <ErrorAlert
          message="Creating token failed."
          details={issueError.message}
        />
      );
    }
    return null;
  }

  function displayCreatedToken() {
    if (!createdToken) {
      return null;
    }

    const tokenDivStyle = {
      fontFamily: 'monospace',
      wordBreak: 'break-all',
    };

    return (
      <Fragment>
        <div className="row mt-3">
          <div className="col-md">
            <div className="alert alert-warning" role="alert">
              Make sure you save the token.
              {' '}
              You will not be able to access it again.
            </div>
          </div>
        </div>
        <div className="row">
          <div className="col-md-2">
            <strong>Expiration</strong>
          </div>
          <div className="col-md-10">
            {expirationTime}
          </div>
        </div>
        <div className="row mt-3">
          <div className="col-md-2">
            <div className="row">
              <div className="col-md">
                <strong>Token</strong>
              </div>
            </div>
            <div className="row">
              <div className="col-md">
                <div
                  className="btn-group mt-2 sm"
                  role="group"
                >
                  <button
                    type="button"
                    className="btn btn-sm btn-outline-primary"
                    onClick={handleClickCopyToken}
                  >
                    {tokenCopied ? 'Copied' : 'Copy'}
                  </button>
                  <button
                    type="button"
                    className="btn btn-sm btn-outline-primary"
                    onClick={handleClickCloseToken}
                  >
                    Close
                  </button>
                </div>
              </div>
            </div>
          </div>
          <div
            className="col-md-10"
            ref={tokenDivRef}
            style={tokenDivStyle}
          >
            {token}
          </div>
        </div>
      </Fragment>
    );
  }

  return (
    <Fragment>
      <div className="row">
        <div className="col-md">
          <strong>{title}</strong>
        </div>
      </div>
      <div className="row">
        <div className="col-md">
          {description}
        </div>
      </div>
      <div className="row mt-3">
        <div className="col-md">
          Add new token with name:
        </div>
      </div>
      <div className="row">
        <div className="col-md">
          <div className="input-group mb-3">
            <input
              key={id}
              type="text"
              className="form-control"
              placeholder={title}
              defaultValue={title}
              ref={tokenInputRef}
            />
            <div className="input-group-append">
              <button
                className="btn btn-outline-primary"
                type="button"
                onClick={handleClickCreateToken}
              >
                Create Token
              </button>
            </div>
          </div>
        </div>
      </div>
      {displayError()}
      {displayCreatedToken()}
    </Fragment>
  );
}

TokenDetailsTrackee.propTypes = {
  isTokenConf: PropTypes.bool.isRequired,
  id: PropTypes.string,
  title: PropTypes.string,
  path: PropTypes.string,
  description: PropTypes.string,
  callIssueToken: PropTypes.func.isRequired,
};

TokenDetailsTrackee.defaultProps = {
  id: '',
  title: '',
  path: '',
  description: '',
};

function TokenDetailsTracker({
  jwt,
  callIssueToken,
}) {
  const isTokenConf = !!jwt;
  if (isTokenConf) {
    return {
      isTokenConf,
      id: jwt.id(),
      title: jwt.title(),
      path: jwt.path(),
      description: jwt.description(),
      callIssueToken,
    };
  }
  return {
    isTokenConf,
    callIssueToken,
  };
}

const TokenDetails = withTracker(TokenDetailsTracker)(TokenDetailsTrackee);

TokenDetails.propTypes = {
  jwt: PropTypes.object,
  callIssueToken: PropTypes.func.isRequired,
};

TokenDetails.defaultProps = {
  jwt: null,
};

function TokenConfigurationsTrackee({
  parentUrl,
  hasConfiguredTokens,
  readyJwts,
  callIssueToken,
}) {
  const [activeTokenId, setActiveTokenId] = useState('');

  const routes = {
    home: {
      path: `${parentUrl}`,
      to() { return `${parentUrl}`; },
    },
    tokens: {
      path: `${parentUrl}/:id`,
      to({ id } = {}) {
        return `${parentUrl}/${id}`;
      },
    },
  };

  function mainView() {
    if (!hasConfiguredTokens) {
      return (
        <Fragment>
          No token configurations found.
        </Fragment>
      );
    }

    function itemClass(id) {
      let c = 'list-group-item list-group-item-action';
      if (id === activeTokenId) {
        c += ' active';
      }
      return c;
    }

    return (
      <Fragment>
        <div className="row">
          <div className="col-md-3">
            <ul className="list-group">
              {readyJwts.map(t => (
                <Link
                  className={itemClass(t.id())}
                  key={t.id()}
                  to={routes.tokens.to({ id: t.id() })}
                >
                  {t.title()}
                </Link>
              ))}
            </ul>
          </div>
          <div className="col-md-9">
            <Route
              exact
              path={routes.home.path}
              render={() => <p>Select a token configuration on the left.</p>}
            />
            <Route
              strict
              path={routes.tokens.path}
              render={({ match }) => {
                setActiveTokenId(match.params.id);
                return (
                  <TokenDetails
                    jwt={readyJwts.find(t => t.id() === match.params.id)}
                    callIssueToken={callIssueToken}
                  />
                );
              }}
            />
          </div>
        </div>
      </Fragment>
    );
  }

  return (
    <Fragment>
      <div className="row">
        <div className="col-md">
          <h2>
            Create Tokens
          </h2>
          <hr />
        </div>
      </div>
      {mainView()}
    </Fragment>
  );
}

TokenConfigurationsTrackee.propTypes = {
  parentUrl: PropTypes.string.isRequired,
  hasConfiguredTokens: PropTypes.bool.isRequired,
  readyJwts: PropTypes.array.isRequired,
  callIssueToken: PropTypes.func.isRequired,
};

function TokenConfigurationsTracker({
  parentUrl,
  readyJwts,
  callIssueToken,
}) {
  const hasConfiguredTokens = (readyJwts.find().fetch().length > 0);

  return {
    parentUrl,
    hasConfiguredTokens,
    readyJwts: readyJwts.find().fetch(),
    callIssueToken,
  };
}

const TokenConfigurations = withTracker(
  TokenConfigurationsTracker)(TokenConfigurationsTrackee);

TokenConfigurations.propTypes = {
  parentUrl: PropTypes.string.isRequired,
  readyJwts: PropTypes.instanceOf(Mongo.Collection).isRequired,
  callIssueToken: PropTypes.func.isRequired,
};

function TokenListTrackee({
  userId,
  hasTokens,
  tokens,
  callDeleteUserToken,
}) {
  const tableStyle = {
    borderTop: 'none',
  };

  function handleClickDeleteToken(jti, ev) {
    ev.preventDefault();
    callDeleteUserToken({ userId, jti }, (err) => {
      if (err) {
        console.error(err);
      }
    });
  }

  function listTokens() {
    if (!hasTokens) {
      return (
        <Fragment>
          <div className="row">
            <div className="col-md">
              No active tokens found.
            </div>
          </div>
        </Fragment>
      );
    }

    const sortedTokens = tokens.sort((a, b) => b.exp - a.exp);
    const now = new Date();

    function isExpired(token) {
      return token.exp.getTime() < now.getTime();
    }

    function tokenRowClass(token) {
      return isExpired(token) ? 'text-muted' : '';
    }

    return (
      <Fragment>
        <div className="row">
          <div className="col-md">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col" style={tableStyle}>Name</th>
                  <th scope="col" style={tableStyle}>Created At</th>
                  <th scope="col" style={tableStyle}>Expires At</th>
                  <th scope="col" style={tableStyle}>Token ID</th>
                  <th scope="col" style={tableStyle} />
                </tr>
              </thead>
              <tbody>
                {sortedTokens.map(t => (
                  <tr key={t.jti} className={tokenRowClass(t)}>
                    <td>
                      {t.name || 'Unnamed Token'}
                    </td>
                    <td>
                      {moment(t.iat).utc().format('YYYY-MM-DD HH:mm:ss UTC')}
                    </td>
                    <td>
                      {moment(t.exp).utc().format('YYYY-MM-DD HH:mm:ss UTC')}
                      {isExpired(t) ? (
                        ' (expired)'
                      ) : (
                        null
                      )}
                    </td>
                    <td>
                      {t.jti}
                    </td>
                    <td>
                      <button
                        type="button"
                        data-jti={t.jti}
                        className="btn btn-sm btn-outline-danger"
                        onClick={e => handleClickDeleteToken(t.jti, e)}
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </Fragment>
    );
  }

  return (
    <Fragment>
      <div className="row">
        <div className="col-md">
          <h2>
            Your Tokens
          </h2>
          <hr />
        </div>
      </div>
      {listTokens()}
    </Fragment>
  );
}

TokenListTrackee.propTypes = {
  userId: PropTypes.string.isRequired,
  hasTokens: PropTypes.bool.isRequired,
  tokens: PropTypes.array.isRequired,
  callDeleteUserToken: PropTypes.func.isRequired,
};

function TokenListTracker({
  user,
  tokens,
  callDeleteUserToken,
}) {
  const hasTokens = tokens.length > 0;
  const userId = user._id;

  return {
    userId,
    hasTokens,
    tokens,
    callDeleteUserToken,
  };
}

const TokenList = withTracker(TokenListTracker)(TokenListTrackee);

TokenList.propTypes = {
  user: PropTypes.object.isRequired,
  tokens: PropTypes.array.isRequired,
  callDeleteUserToken: PropTypes.func.isRequired,
};

function ManageTokensTrackee({
  user,
  parentUrl,
  isReady,
  readyJwts,
  callIssueToken,
  callDeleteUserToken,
  userTokens,
}) {
  function mainView() {
    if (!isReady) {
      return (
        <div className="row">
          <div className="col-md">
            Loading ...
          </div>
        </div>
      );
    }

    return (
      <Fragment>
        <div className="row mt-5">
          <div className="col-md">
            <TokenConfigurations
              parentUrl={parentUrl}
              readyJwts={readyJwts}
              callIssueToken={callIssueToken}
            />
          </div>
        </div>
        <div className="row mt-5">
          <div className="col-md">
            <TokenList
              user={user}
              callDeleteUserToken={callDeleteUserToken}
              tokens={userTokens}
            />
          </div>
        </div>
      </Fragment>
    );
  }

  return (
    <div className="container-fluid">
      <div className="row">
        <div className="col-md">
          <h1 className="text-center">
            Manage Tokens
          </h1>
        </div>
      </div>
      {mainView()}
    </div>
  );
}

ManageTokensTrackee.propTypes = {
  user: PropTypes.object.isRequired,
  parentUrl: PropTypes.string.isRequired,
  isReady: PropTypes.bool.isRequired,
  readyJwts: PropTypes.instanceOf(Mongo.Collection).isRequired,
  callIssueToken: PropTypes.func.isRequired,
  callDeleteUserToken: PropTypes.func.isRequired,
  userTokens: PropTypes.array.isRequired,
};

function ManageTokensTracker({
  user,
  parentUrl,
  subscribeReadyJwts,
  readyJwts,
  callIssueToken,
  callDeleteUserToken,
  subscribeUserTokens,
  getUserTokens,
}) {
  const subReadyJwts = subscribeReadyJwts();
  const subUserTokens = subscribeUserTokens();

  return {
    user,
    parentUrl,
    isReady: subReadyJwts.ready() && subUserTokens.ready(),
    readyJwts,
    callIssueToken,
    callDeleteUserToken,
    userTokens: getUserTokens(),
  };
}

const ManageTokens = withTracker(ManageTokensTracker)(ManageTokensTrackee);

ManageTokens.propTypes = {
  user: PropTypes.object.isRequired,
  parentUrl: PropTypes.string.isRequired,
  subscribeReadyJwts: PropTypes.func.isRequired,
  readyJwts: PropTypes.instanceOf(Mongo.Collection).isRequired,
  callIssueToken: PropTypes.func.isRequired,
  callDeleteUserToken: PropTypes.func.isRequired,
  subscribeUserTokens: PropTypes.func.isRequired,
  getUserTokens: PropTypes.func.isRequired,
};

export {
  ManageTokens,
};
