/* eslint-disable react/forbid-prop-types */

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import Blaze from 'meteor/gadicc:blaze-react-component';
import { Random } from 'meteor/random';
import { withTracker } from 'meteor/react-meteor-data';
import { ReactiveVar } from 'meteor/reactive-var';
import {
  AA_FSO_LIST_REPOS,
  KeyId,
  KeyPath,
} from 'meteor/nog-fso';
import {
  PrefixBreadcrumbs,
  CondensedRepoBreadcrumbs,
} from './breadcrumbs.jsx';

function ErrAlert({ error, onClickClose }) {
  function handleClick() {
    onClickClose(error.id());
  }

  return (
    <div className="alert alert-warning" role="alert">
      <button
        type="button"
        className="close"
        aria-label="Close"
        onClick={handleClick}
      ><span aria-hidden="true">&times;</span>
      </button>
      <span>{error.message()} </span>
    </div>
  );
}

ErrAlert.propTypes = {
  error: PropTypes.object.isRequired,
  onClickClose: PropTypes.func.isRequired,
};

function List({
  isReady, topNodes, nodes, makeHrefLsPath, makeHrefRepo,
  isRecursive, toggleRecursive,
  refreshListing,
  errors, clearError,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }

  const errorAlerts = errors.map(e => (
    <ErrAlert
      key={e.id()}
      error={e}
      onClickClose={clearError}
    />
  ));

  const buttons = (
    <p>
      <button
        type="button"
        className="btn btn-default btn-xs"
        onClick={refreshListing}
      >
        Refresh
      </button>
      {' '}
      <small>
        <input
          type="checkbox"
          checked={isRecursive}
          onChange={toggleRecursive}
        />
        {' '}
        recursive
      </small>
    </p>
  );

  const list = () => {
    // Display topNodes first, then subdirs before repos.
    const dirs = nodes.filter(n => n.isDir());
    const repos = nodes.filter(n => n.isRepo());
    return topNodes.concat(dirs).concat(repos).map((n) => {
      const path = n.path();
      if (n.isDir()) {
        const href = makeHrefLsPath(path);
        return (
          <tr key={n.id()}>
            <td>
              <i className="fa fa-folder-o" />
              {' '}
              <a href={href}>{path}</a>
            </td>
          </tr>
        );
      }
      const href = makeHrefRepo(path);
      return (
        <tr key={n.id()}>
          <td>
            <a href={href}><i className="fa fa-database" /></a>
            {' '}
            <CondensedRepoBreadcrumbs
              path={path}
              makeHrefLsPath={makeHrefLsPath}
              makeHrefRepo={makeHrefRepo}
            />
          </td>
        </tr>
      );
    });
  };

  return (
    <div className="row">
      <div className="col-md-12">
        {buttons}
        {errorAlerts}
        <table className="table table-condensed table-striped">
          <tbody>
            {list()}
          </tbody>
        </table>
      </div>
    </div>
  );
}

List.propTypes = {
  isReady: PropTypes.bool.isRequired,
  topNodes: PropTypes.arrayOf(PropTypes.object).isRequired,
  nodes: PropTypes.arrayOf(PropTypes.object).isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
  isRecursive: PropTypes.bool.isRequired,
  toggleRecursive: PropTypes.func.isRequired,
  refreshListing: PropTypes.func.isRequired,
  errors: PropTypes.arrayOf(PropTypes.object).isRequired,
  clearError: PropTypes.func.isRequired,
};

// From
// <https://github.com/sindresorhus/escape-string-regexp/blob/master/index.js>
const reOperatorsRgx = /[|\\{}()[\]^$+*?.]/g;

function escapeRegExp(s) {
  return s.replace(reOperatorsRgx, '\\$&');
}

function trimRightSlash(s) {
  if (s.endsWith('/')) {
    return s.substr(0, s.length - 1);
  }
  return s;
}

// Recursive listing is managed as global state, so that it is preserved when
// navigating to a repo view and back to the listing.  An alternative could be
// to maintain it as `ListTracker` state.  To prepare for that,
// `toggleRecursive()` is passed as a prop, so that children are unaware of the
// state tracking details.
const isRecursiveEnabled = new ReactiveVar(false);

// `ignoredErrorIds` is a list of errors that are ignored.
const ignoredErrorIds = new ReactiveVar([]);

// `nonce` is used to reactively force a new subscription.
const nonce = new ReactiveVar(Random.id());

function ListTracker({
  path, subscribeListing, listingNodes,
  listingErrors,
  makeHrefLsPath, makeHrefRepo,
}) {
  const sub = subscribeListing({
    path,
    recursive: isRecursiveEnabled.get(),
    nonce: nonce.get(),
  });

  function toggleRecursive() {
    isRecursiveEnabled.set(!isRecursiveEnabled.get());
  }

  // Instead of clearing the error, we only pretend that it has been cleared by
  // putting it on the ignore list.  The errors will be cleared when the
  // subscription ends.
  function addIgnoredErrorId(id) {
    const ids = ignoredErrorIds.get();
    ids.push(id);
    ignoredErrorIds.set(ids);
  }

  const errors = listingErrors.find({
    [KeyId]: { $nin: ignoredErrorIds.get() },
  }).fetch();

  function refreshListing() {
    nonce.set(Random.id());
  }

  const isReady = sub.ready();
  const selRepoAtPrefix = {
    [KeyPath]: trimRightSlash(path),
  };
  const selBelow = {
    [KeyPath]: { $regex: `^${escapeRegExp(path)}` },
  };
  const sort = { [KeyPath]: 1 };
  return {
    isReady,
    topNodes: listingNodes.find(selRepoAtPrefix).fetch(),
    nodes: listingNodes.find(selBelow, { sort }).fetch(),
    errors,
    clearError: addIgnoredErrorId,
    makeHrefLsPath, makeHrefRepo,
    isRecursive: isRecursiveEnabled.get(),
    toggleRecursive,
    refreshListing,
  };
}

const ListContainer = withTracker(ListTracker)(List);

ListContainer.propTypes = {
  path: PropTypes.string.isRequired,
  listingNodes: PropTypes.object.isRequired,
  subscribeListing: PropTypes.func.isRequired,
  listingErrors: PropTypes.object.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
};

function Layout({
  nogFso, path, makeHrefLsPath, makeHrefRepo,
}) {
  const {
    subscribeListing, listingNodes, listingErrors,
  } = nogFso;

  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h4>
            Repos
            {' '}
            <PrefixBreadcrumbs
              makeHrefLsPath={makeHrefLsPath}
              path={path}
            />
          </h4>
        </div>
      </div>
      <ListContainer
        path={path}
        listingNodes={listingNodes}
        listingErrors={listingErrors}
        subscribeListing={subscribeListing}
        makeHrefLsPath={makeHrefLsPath}
        makeHrefRepo={makeHrefRepo}
      />
    </Fragment>
  );
}

Layout.propTypes = {
  nogFso: PropTypes.shape({
    subscribeListing: PropTypes.func.isRequired,
    listingNodes: PropTypes.object.isRequired,
    listingErrors: PropTypes.object.isRequired,
  }).isRequired,
  path: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
};

function Gate({
  isReady, mayAccess, nogFso, path, makeHrefLsPath, makeHrefRepo,
}) {
  if (!isReady) {
    return (
      <div />
    );
  }
  if (!mayAccess) {
    return (
      <Blaze template="denied" />
    );
  }
  return (
    <Layout
      nogFso={nogFso}
      path={path}
      makeHrefLsPath={makeHrefLsPath}
      makeHrefRepo={makeHrefRepo}
    />
  );
}

Gate.propTypes = {
  isReady: PropTypes.bool.isRequired,
  mayAccess: PropTypes.bool.isRequired,
  nogFso: PropTypes.object.isRequired,
  path: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
};

function ensureLeadingSlash(s) {
  if (s.startsWith('/')) {
    return s;
  }
  return `/${s}`;
}

function trimSlashes(s) {
  return s.replace(/^\/*/, '').replace(/\/*$/, '');
}

function getPathParam(router) {
  const path = router.getParam('path');
  if (path == null) {
    return '/';
  }
  return ensureLeadingSlash(path);
}

function GateTracker({ router, routes, nogFso }) {
  function makeHrefLsPath(path) {
    return router.path(routes.fsoListing, {
      path: trimSlashes(path),
    });
  }

  function makeHrefRepo(repoName) {
    return router.path(routes.fsoRepo, {
      repoName: trimSlashes(repoName),
    });
  }

  const path = getPathParam(router);
  const ok = nogFso.testAccess(AA_FSO_LIST_REPOS, { path });
  return {
    isReady: (ok != null),
    mayAccess: !!ok,
    nogFso,
    path,
    makeHrefLsPath,
    makeHrefRepo,
  };
}

const FsoListingGateContainer = withTracker(GateTracker)(Gate);

FsoListingGateContainer.propTypes = {
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    fsoListing: PropTypes.string.isRequired,
    fsoRepo: PropTypes.string.isRequired,
  }).isRequired,
  nogFso: PropTypes.shape({
    testAccess: PropTypes.func.isRequired,
  }).isRequired,
};

export {
  FsoListingGateContainer,
};
