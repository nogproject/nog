/* eslint-disable react/forbid-prop-types */

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import Blaze from 'meteor/gadicc:blaze-react-component';
import { withTracker } from 'meteor/react-meteor-data';
import {
  AA_FSO_DISCOVER,
  KeyGlobalRootPath,
} from 'meteor/nog-fso';

function Item({ root, makeCandidateListHref }) {
  const globalRoot = root.globalRootPath();
  const registry = root.registryName();
  const href = makeCandidateListHref({ registry, globalRoot });
  return (
    <tr>
      <td>
        <i className="fa fa-folder-o" />
        {' '}
        <a href={href}>{globalRoot}/</a>
      </td>
    </tr>
  );
}

Item.propTypes = {
  root: PropTypes.object.isRequired,
  makeCandidateListHref: PropTypes.func.isRequired,
};

function ErrAlert({ message }) {
  return (
    <div className="alert alert-warning" role="alert">
      <span>{message} </span>
      <strong>The list may be incomplete. Consider reloading later.</strong>
    </div>
  );
}

ErrAlert.propTypes = {
  message: PropTypes.string.isRequired,
};

function List({
  roots, isReady, makeCandidateListHref,
  errors,
}) {
  function content() {
    if (!isReady) {
      return <span>Loading...</span>;
    }

    const items = roots.map(r => (
      <Item
        key={r.id()}
        root={r}
        makeCandidateListHref={makeCandidateListHref}
      />
    ));

    const errorAlerts = errors.map(e => (
      <ErrAlert key={e.id()} message={e.message()} />
    ));

    return (
      <Fragment>
        {errorAlerts}
        <p>{roots.length} roots</p>
        <table className="table table-condensed table-striped">
          <tbody>
            {items}
          </tbody>
        </table>
      </Fragment>
    );
  }

  return (
    <div className="row">
      <div className="col-md-12">
        {content()}
      </div>
    </div>
  );
}

List.propTypes = {
  isReady: PropTypes.bool.isRequired,
  roots: PropTypes.arrayOf(PropTypes.object).isRequired,
  makeCandidateListHref: PropTypes.func.isRequired,
  errors: PropTypes.arrayOf(PropTypes.object).isRequired,
};

function ListTracker({
  prefix, roots, subscribeRoots, makeCandidateListHref,
  discoveryErrors,
}) {
  const sub = subscribeRoots({ prefix });
  const isReady = sub.ready();
  const sort = { [KeyGlobalRootPath]: 1 };
  return {
    isReady,
    roots: roots.find({}, { sort }).fetch(),
    errors: discoveryErrors.find().fetch(),
    makeCandidateListHref,
  };
}

const ListContainer = withTracker(ListTracker)(List);

ListContainer.propTypes = {
  prefix: PropTypes.string.isRequired,
  roots: PropTypes.object.isRequired,
  subscribeRoots: PropTypes.func.isRequired,
  discoveryErrors: PropTypes.object.isRequired,
  makeCandidateListHref: PropTypes.func.isRequired,
};

function Layout({ nogFso, prefix, makeCandidateListHref }) {
  const {
    subscribeRoots, roots,
    discoveryErrors,
  } = nogFso;

  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h4>Discover Untracked Directories</h4>
        </div>
      </div>
      <ListContainer
        prefix={prefix}
        roots={roots}
        subscribeRoots={subscribeRoots}
        discoveryErrors={discoveryErrors}
        makeCandidateListHref={makeCandidateListHref}
      />
    </Fragment>
  );
}

Layout.propTypes = {
  nogFso: PropTypes.shape({
    subscribeRoots: PropTypes.func.isRequired,
    roots: PropTypes.object.isRequired,
    discoveryErrors: PropTypes.object.isRequired,
  }).isRequired,
  prefix: PropTypes.string.isRequired,
  makeCandidateListHref: PropTypes.func.isRequired,
};

function Gate({
  isReady, mayAccess, nogFso, prefix, makeCandidateListHref,
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
      prefix={prefix}
      makeCandidateListHref={makeCandidateListHref}
    />
  );
}

Gate.propTypes = {
  isReady: PropTypes.bool.isRequired,
  mayAccess: PropTypes.bool.isRequired,
  nogFso: PropTypes.object.isRequired,
  prefix: PropTypes.string.isRequired,
  makeCandidateListHref: PropTypes.func.isRequired,
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

function getPrefixParam(router) {
  const path = router.getParam('prefix');
  if (path == null) {
    return '/';
  }
  return ensureLeadingSlash(path);
}

function GateTracker({ router, routes, nogFso }) {
  function makeCandidateListHref({
    registry, globalRoot,
  }) {
    return router.path(routes.fsoUntrackedList, {
      registry,
      globalRoot: trimSlashes(globalRoot),
    });
  }

  const prefix = getPrefixParam(router);
  const ok = nogFso.testAccess(AA_FSO_DISCOVER, { path: prefix });
  return {
    isReady: (ok != null),
    mayAccess: !!ok,
    nogFso,
    prefix,
    makeCandidateListHref,
  };
}

const FsoRootListGateContainer = withTracker(GateTracker)(Gate);

FsoRootListGateContainer.propTypes = {
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    fsoUntrackedList: PropTypes.string.isRequired,
  }).isRequired,
  nogFso: PropTypes.shape({
    testAccess: PropTypes.func.isRequired,
  }).isRequired,
};

export {
  FsoRootListGateContainer,
};
