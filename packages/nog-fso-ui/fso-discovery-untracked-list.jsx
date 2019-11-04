/* eslint-disable react/forbid-prop-types */
/* eslint-disable react/no-multi-comp */

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import Blaze from 'meteor/gadicc:blaze-react-component';
import { withTracker } from 'meteor/react-meteor-data';
import { Random } from 'meteor/random';
import { ReactiveVar } from 'meteor/reactive-var';
import {
  AA_FSO_DISCOVER_ROOT,
  AA_FSO_ENABLE_DISCOVERY_PATH,
  KeyUntrackedGlobalPath,
} from 'meteor/nog-fso';
import {
  CondensedRepoBreadcrumbs,
} from './breadcrumbs.jsx';


function ErrorAlert({ error, onClickClose }) {
  if (!error.message) {
    return null;
  }

  return (
    <div className="row">
      <div className="col-md-12">
        <div className="alert alert-danger" role="alert">
          <button
            type="button"
            className="close"
            aria-label="Close"
            onClick={onClickClose}
          ><span aria-hidden="true">&times;</span>
          </button>
          <strong>{error.title}:</strong> {error.message}
        </div>
      </div>
    </div>
  );
}

ErrorAlert.propTypes = {
  error: PropTypes.object.isRequired,
  onClickClose: PropTypes.func.isRequired,
};

// `InitRepoAtPathForm` is a button together with a text input to initialize a
// repo at a global path, which must be below `props.globalRoot`.
//
// State:
//
//  - `path` tracks the HTML text input.
//  - `sanePath` is a cleaned version of the text input, with leading and
//    trailing spaces removed and backslashes replaced by slashes.
//  - `isAdded` controls whether the link to the new repo is displayed.  It is
//    set to `true` when `callInitRepo()` succeeded and reset to `false` when
//    `path` changes.
//  - `error` is an optional error message.  It is set if `callInitRepo()`
//    fails and can be cleared by the user.
//
class InitRepoAtPathForm extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      path: '',
      sanePath: '',
      isAdded: false,
      error: null,
    };
    this.handleChange = this.handleChange.bind(this);
    this.handleSubmit = this.handleSubmit.bind(this);
    this.clearError = this.clearError.bind(this);
  }

  handleChange(ev) {
    this.setState({
      path: ev.target.value,
      sanePath: ev.target.value.trim().replace(/\\/g, '/'),
      isAdded: false,
    });
  }

  handleSubmit(ev) {
    ev.preventDefault();

    if (!this.isValidPath()) {
      this.setState({
        error: 'The path must be below the root and must not end with slash.',
      });
      return;
    }

    const { registryName, callInitRepo } = this.props;
    const { sanePath } = this.state;

    this.setState({ error: null });
    callInitRepo({ registryName, globalPath: sanePath }, (err) => {
      if (err) {
        this.setState({ error: err.message });
      } else {
        this.setState({ isAdded: true });
      }
    });
  }

  clearError() {
    this.setState({ error: null });
  }

  isValidPath() {
    const { globalRoot } = this.props;
    const { sanePath } = this.state;
    return sanePath.startsWith(`${globalRoot}/`) && !sanePath.endsWith('/');
  }

  render() {
    const { makeHrefLsPath, makeHrefRepo } = this.props;
    const { path, sanePath, isAdded } = this.state;
    const { handleChange, handleSubmit, clearError } = this;

    const error = {
      title: 'Failed to initialize repo',
      message: this.state.error,
    };

    function htmlPath() {
      if (!isAdded) {
        return null;
      }
      return (
        <p>
          <i className="fa fa-check" aria-hidden="true" />
          {' '}
          <a href={makeHrefRepo(sanePath)}>
            <i className="fa fa-database" />
          </a>
          {' '}
          <CondensedRepoBreadcrumbs
            path={sanePath}
            makeHrefLsPath={makeHrefLsPath}
            makeHrefRepo={makeHrefRepo}
          />
        </p>
      );
    }

    const classHasError = this.isValidPath() ? '' : 'has-error';

    return (
      <form
        className="form-inline"
        onSubmit={handleSubmit}
      >
        <button
          type="submit"
          className="btn btn-default"
        >
          Initialize Repo at Path
        </button>
        {' '}
        <div className={`form-group ${classHasError}`}>
          <label
            className="control-label"
            htmlFor="globalPath"
          >
            Global Path
          </label>
          {' '}
          <input
            id="globalPath"
            type="text"
            className="form-control"
            size="80"
            value={path}
            placeholder="/path/to/repo"
            onChange={handleChange}
          />
        </div>
        {htmlPath()}
        <ErrorAlert error={error} onClickClose={clearError} />
      </form>
    );
  }
}

InitRepoAtPathForm.propTypes = {
  registryName: PropTypes.string.isRequired,
  globalRoot: PropTypes.string.isRequired,
  callInitRepo: PropTypes.func.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
};

class IgnoredItem extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      error: null,
    };

    this.handleClickEnablePath = this.handleClickEnablePath.bind(this);
    this.clearError = this.clearError.bind(this);
  }

  handleClickEnablePath(ev, depth) {
    ev.preventDefault();
    const {
      untracked,
      registryName, globalRoot,
      mayEnablePath, callEnableDiscoveryPath, refreshListing,
    } = this.props;
    const globalPath = untracked.globalPath();

    if (!mayEnablePath) {
      const andSubdirs = depth > 0 ? ' and its subdirectories' : '';
      const error = (
        'You do not have the permission to enable paths. ' +
        `Contact an admin to enable ${globalPath}${andSubdirs}.`
      );
      this.setState({ error });
      return;
    }

    this.setState({ error: null });
    callEnableDiscoveryPath({
      registryName, globalRoot, depth, globalPath,
    }, (err) => {
      if (err) {
        this.setState({ error: err.message });
      } else {
        refreshListing();
        console.log(`Enabled \`${globalPath}\`.`);
      }
    });
  }

  clearError() {
    this.setState({ error: null });
  }

  render() {
    const { untracked } = this.props;
    const { handleClickEnablePath, clearError } = this;

    const path = untracked.globalPath();
    const error = {
      title: 'Failed to enable',
      message: this.state.error,
    };

    return (
      <tr>
        <td>
          <button
            type="button"
            className="btn btn-xs btn-default"
            onClick={ev => handleClickEnablePath(ev, 0)}
            data-toggle="tooltip"
            title="Enable path without subdirectories"
            disabled={false}
          >
            Enable Path
          </button>
          {' '}
          <button
            type="button"
            className="btn btn-xs btn-default"
            onClick={ev => handleClickEnablePath(ev, 1)}
            data-toggle="tooltip"
            title="Enable path and direct subdirectories"
            disabled={false}
          >
            +Subdirectories
          </button>
          {' '}
          <span>{path}</span>
          <ErrorAlert error={error} onClickClose={clearError} />
        </td>
      </tr>
    );
  }
}

IgnoredItem.propTypes = {
  untracked: PropTypes.object.isRequired,
  registryName: PropTypes.string.isRequired,
  globalRoot: PropTypes.string.isRequired,
  callEnableDiscoveryPath: PropTypes.func.isRequired,
  mayEnablePath: PropTypes.bool.isRequired,
  refreshListing: PropTypes.func.isRequired,
};

class Item extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      error: null,
      added: false,
    };

    this.handleSubmit = this.handleSubmit.bind(this);
    this.clearError = this.clearError.bind(this);
  }

  handleSubmit(ev) {
    ev.preventDefault();
    const { registryName, untracked, callInitRepo } = this.props;
    const globalPath = untracked.globalPath();
    this.setState({ error: null });
    callInitRepo({ registryName, globalPath }, (err) => {
      if (err) {
        this.setState({ error: err.message });
      } else {
        this.setState({
          added: true,
        });
      }
    });
  }

  clearError() {
    this.setState({ error: null });
  }

  render() {
    const {
      untracked,
      makeHrefLsPath, makeHrefRepo,
    } = this.props;
    const { handleSubmit, clearError } = this;
    const { added } = this.state;

    const path = untracked.globalPath();
    const error = {
      title: 'Failed to initialize',
      message: this.state.error,
    };
    const icon = (added) ? 'check' : 'plus';
    const htmlPath = (added) ? (
      <Fragment>
        <a href={makeHrefRepo(path)}>
          <i className="fa fa-database" />
        </a>
        {' '}
        <CondensedRepoBreadcrumbs
          path={path}
          makeHrefLsPath={makeHrefLsPath}
          makeHrefRepo={makeHrefRepo}
        />
      </Fragment>
    ) : (
      <span>{path}</span>
    );

    return (
      <tr>
        <td>
          <button
            type="submit"
            className="btn btn-xs btn-default"
            onClick={handleSubmit}
            data-toggle="tooltip"
            title="Init Repo"
            disabled={added}
          >
            <i className={`fa fa-${icon}`} aria-hidden="true" />
          </button>
          {' '}
          {htmlPath}
          <ErrorAlert error={error} onClickClose={clearError} />
        </td>
      </tr>
    );
  }
}

Item.propTypes = {
  untracked: PropTypes.object.isRequired,
  registryName: PropTypes.string.isRequired,
  callInitRepo: PropTypes.func.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
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
  isReady, mayEnablePath,
  globalRoot, registry, untracked, errors,
  callInitRepo, callEnableDiscoveryPath, refreshListing,
  makeHrefLsPath, makeHrefRepo,
}) {
  const candidates = untracked.filter(u => u.isCandidate());
  const ignored = untracked.filter(u => u.isIgnored());

  function content() {
    if (!isReady) {
      return <span>Loading...</span>;
    }

    const items = candidates.map(u => (
      <Item
        key={u.id()}
        untracked={u}
        registryName={registry}
        callInitRepo={callInitRepo}
        makeHrefLsPath={makeHrefLsPath}
        makeHrefRepo={makeHrefRepo}
      />
    ));
    const ignoredItems = ignored.map(u => (
      <IgnoredItem
        key={u.id()}
        untracked={u}
        registryName={registry}
        globalRoot={globalRoot}
        callEnableDiscoveryPath={callEnableDiscoveryPath}
        mayEnablePath={mayEnablePath}
        refreshListing={refreshListing}
      />
    ));

    const errorAlerts = errors.map(e => (
      <ErrAlert key={e.id()} message={e.message()} />
    ));

    function pluralize(val, noun) {
      switch (val) {
        case 0: return `no ${noun}s`;
        case 1: return `${val} ${noun}`;
        default: return `${val} ${noun}s`;
      }
    }

    const summary = (
      `${pluralize(candidates.length, 'repository candidate')} ` +
      `below ${globalRoot}/`
    );
    const ignoredSummary = `${pluralize(ignored.length, 'path')} ignored`;

    return (
      <Fragment>
        {errorAlerts}
        <p>{summary}</p>
        <table className="table table-condensed table-striped">
          <tbody>
            {items}
          </tbody>
        </table>
        <InitRepoAtPathForm
          registryName={registry}
          globalRoot={globalRoot}
          callInitRepo={callInitRepo}
          makeHrefLsPath={makeHrefLsPath}
          makeHrefRepo={makeHrefRepo}
        />
        <p>{ignoredSummary}</p>
        <table className="table table-condensed table-striped">
          <tbody>
            {ignoredItems}
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
  globalRoot: PropTypes.string.isRequired,
  untracked: PropTypes.arrayOf(PropTypes.object).isRequired,
  errors: PropTypes.arrayOf(PropTypes.object).isRequired,
  registry: PropTypes.string.isRequired,
  callInitRepo: PropTypes.func.isRequired,
  callEnableDiscoveryPath: PropTypes.func.isRequired,
  mayEnablePath: PropTypes.bool.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
  refreshListing: PropTypes.func.isRequired,
};

// `nonce` is used to reactively force a subscription refresh.
const nonce = new ReactiveVar(Random.id());

function ListTracker({
  registry, globalRoot,
  untracked, subscribeUntracked,
  discoveryErrors,
}) {
  const sub = subscribeUntracked({
    registry, globalRoot,
    nonce: nonce.get(),
  });

  function refreshListing() {
    nonce.set(Random.id());
  }

  const isReady = sub.ready();
  const sort = { [KeyUntrackedGlobalPath]: 1 };
  return {
    isReady,
    globalRoot,
    registryName: registry,
    untracked: untracked.find({}, { sort }).fetch(),
    errors: discoveryErrors.find().fetch(),
    refreshListing,
  };
}

const ListContainer = withTracker(ListTracker)(List);

ListContainer.propTypes = {
  registry: PropTypes.string.isRequired,
  globalRoot: PropTypes.string.isRequired,
  untracked: PropTypes.object.isRequired,
  subscribeUntracked: PropTypes.func.isRequired,
  discoveryErrors: PropTypes.object.isRequired,
};

function Layout({
  mayEnablePath, nogFso,
  registry, globalRoot,
  makeHrefLsPath, makeHrefRepo,
}) {
  const {
    untracked, subscribeUntracked,
    discoveryErrors,
    callInitRepo,
    callEnableDiscoveryPath,
  } = nogFso;

  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h4>Discover Untracked Directories</h4>
        </div>
      </div>
      <ListContainer
        registry={registry}
        globalRoot={globalRoot}
        untracked={untracked}
        subscribeUntracked={subscribeUntracked}
        discoveryErrors={discoveryErrors}
        callInitRepo={callInitRepo}
        callEnableDiscoveryPath={callEnableDiscoveryPath}
        mayEnablePath={mayEnablePath}
        makeHrefLsPath={makeHrefLsPath}
        makeHrefRepo={makeHrefRepo}
      />
    </Fragment>
  );
}

Layout.propTypes = {
  nogFso: PropTypes.shape({
    untracked: PropTypes.object,
    subscribeUntracked: PropTypes.func,
    discoveryErrors: PropTypes.object,
  }).isRequired,
  registry: PropTypes.string.isRequired,
  globalRoot: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
  mayEnablePath: PropTypes.bool.isRequired,
};

function Gate({
  isReady, mayAccess, mayEnablePath, nogFso,
  registry, globalRoot,
  makeHrefLsPath, makeHrefRepo,
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
      registry={registry}
      globalRoot={globalRoot}
      makeHrefLsPath={makeHrefLsPath}
      makeHrefRepo={makeHrefRepo}
      mayEnablePath={mayEnablePath}
    />
  );
}

Gate.propTypes = {
  isReady: PropTypes.bool.isRequired,
  mayAccess: PropTypes.bool.isRequired,
  mayEnablePath: PropTypes.bool.isRequired,
  nogFso: PropTypes.object.isRequired,
  registry: PropTypes.string.isRequired,
  globalRoot: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
};

function trimSlashes(s) {
  return s.replace(/^\/*/, '').replace(/\/*$/, '');
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

  const registry = router.getParam('registry');
  const globalRoot = `/${router.getParam('globalRoot')}`;
  const ok = nogFso.testAccess(AA_FSO_DISCOVER_ROOT, { path: globalRoot });
  // `mayEnablePath` only indicates allow or not, without ready or not.
  const mayEnablePath = !!nogFso.testAccess(
    AA_FSO_ENABLE_DISCOVERY_PATH, { path: globalRoot },
  );
  return {
    isReady: (ok != null),
    mayAccess: !!ok,
    mayEnablePath,
    nogFso,
    registry, globalRoot,
    makeHrefLsPath, makeHrefRepo,
  };
}

const FsoUntrackedListGateContainer = withTracker(GateTracker)(Gate);

FsoUntrackedListGateContainer.propTypes = {
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    fsoListing: PropTypes.string.isRequired,
    fsoRepo: PropTypes.string.isRequired,
  }).isRequired,
  nogFso: PropTypes.shape({
    testAccess: PropTypes.func,
  }).isRequired,
};

export {
  FsoUntrackedListGateContainer,
};
