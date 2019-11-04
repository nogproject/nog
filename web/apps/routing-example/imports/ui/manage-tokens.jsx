import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import {
  Link,
  Route,
} from 'react-router-dom';
import { Mongo } from 'meteor/mongo';

function TokenDetails({
  token,
}) {
  if (Object.keys(token).length > 0) {
    return (
      <Fragment>
        <p>
          token ID:
          {' '}
          {token.id}
        </p>
        <p>
          Expiration time:
          {' '}
          {token.expTime}
        </p>
      </Fragment>
    );
  }
  return (
    <Fragment>
      Unknown token.
    </Fragment>
  );
}

TokenDetails.defaultProps = {
  token: {},
};

TokenDetails.propTypes = {
  token: PropTypes.shape({
    expTime: PropTypes.string,
    id: PropTypes.string,
  }),
};

function ManageTokensTrackee({
  parentUrl,
  fakeTokens,
}) {
  const routes = {
    home: {
      path: `${parentUrl}`,
      to() { return `${parentUrl}`; },
    },
    token: {
      path: `${parentUrl}/:id`,
      to({ id } = {}) {
        return `${parentUrl}/${id}`;
      },
    },
  };

  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h1>Manage tokens</h1>
        </div>
      </div>
      <div className="row">
        <div className="col-md-3">
          <p> Tokens </p>
          <ul>
            {fakeTokens.map(t => (
              <li key={t.id}>
                <Link
                  key={t.id}
                  to={routes.token.to({ id: t.id })}
                >
                  {t.id}
                </Link>
              </li>
            ))}
          </ul>
        </div>
        <div className="col-md-9">
          <p> Details </p>
          <Route
            exact
            path={routes.home.path}
            render={() => <p>Select a token on the left</p>}
          />
          <Route
            path={routes.token.path}
            render={({ match }) => (
              <TokenDetails
                token={fakeTokens.find(u => u.id === match.params.id)}
              />
            )}
          />
        </div>
      </div>
    </Fragment>
  );
}

ManageTokensTrackee.propTypes = {
  parentUrl: PropTypes.string.isRequired,
  fakeTokens: PropTypes.arrayOf(PropTypes.shape({
    expTime: PropTypes.string,
    id: PropTypes.string,
  })).isRequired,
};

function ManageTokensTracker({
  parentUrl,
  fakeTokens,
}) {
  const fakeTokensArray = fakeTokens.find().fetch();
  return {
    parentUrl,
    fakeTokens: fakeTokensArray,
  };
}

const ManageTokens = withTracker(ManageTokensTracker)(ManageTokensTrackee);

ManageTokens.propTypes = {
  parentUrl: PropTypes.string.isRequired,
  fakeTokens: PropTypes.instanceOf(Mongo.Collection).isRequired,
};

export {
  ManageTokens,
};
