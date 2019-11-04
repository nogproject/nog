/* eslint-disable react/forbid-prop-types */

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import Blaze from 'meteor/gadicc:blaze-react-component';
import { withTracker } from 'meteor/react-meteor-data';
import {
  AA_FSO_HOME,
  KeyPath,
  KeyRoute,
} from 'meteor/nog-fso';

// Access to the V2 home is currently controlled by `AA_FSO_HOME`, because the
// primary goal is to provide access to FSO.
const AA_HOME = AA_FSO_HOME;

function LinkList({
  links, makeHref,
}) {
  const items = links.map((link) => {
    const path = link.path();
    const href = makeHref(path);
    return (
      <li key={link.id()}>
        <a href={href}>{path}</a>
      </li>
    );
  });

  return (
    <ul>{items}</ul>
  );
}

LinkList.propTypes = {
  links: PropTypes.arrayOf(PropTypes.object).isRequired,
  makeHref: PropTypes.func.isRequired,
};

function FsoBrowse({
  isReady, links, makeHref,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }

  return (
    <div className="row">
      <div className="col-md-12">
        <h4>Browse</h4>
        <p className="help-block">
          Browse repositories below the following locations.
        </p>
        <LinkList links={links} makeHref={makeHref} />
      </div>
    </div>
  );
}

FsoBrowse.propTypes = {
  isReady: PropTypes.bool.isRequired,
  links: PropTypes.arrayOf(PropTypes.object).isRequired,
  makeHref: PropTypes.func.isRequired,
};

function FsoBrowseTracker({
  nogFso, makeHrefs,
}) {
  const {
    subscribeHome, homeLinks,
  } = nogFso;
  const sub = subscribeHome();
  const isReady = sub.ready();
  const sel = { [KeyRoute]: 'listing' };
  const sort = { [KeyPath]: 1 };
  const makeHref = makeHrefs.fsoListing;
  return {
    isReady,
    links: homeLinks.find(sel, { sort }).fetch(),
    makeHref,
  };
}

const FsoBrowseContainer = withTracker(FsoBrowseTracker)(FsoBrowse);

FsoBrowseContainer.propTypes = {
  nogFso: PropTypes.object.isRequired,
  makeHrefs: PropTypes.shape({
    fsoListing: PropTypes.func.isRequired,
  }).isRequired,
};

function FsoDiscover({
  isReady, links, makeHref,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }

  return (
    <div className="row">
      <div className="col-md-12">
        <h4>Add</h4>
        <p className="help-block">
          Discover untracked directories below the following locations in order
          to add them as repositories.
        </p>
        <LinkList links={links} makeHref={makeHref} />
      </div>
    </div>
  );
}

FsoDiscover.propTypes = {
  isReady: PropTypes.bool.isRequired,
  links: PropTypes.arrayOf(PropTypes.object).isRequired,
  makeHref: PropTypes.func.isRequired,
};

function FsoDiscoverTracker({
  nogFso, makeHrefs,
}) {
  const {
    subscribeHome, homeLinks,
  } = nogFso;
  const sub = subscribeHome();
  const isReady = sub.ready();
  const sel = { [KeyRoute]: 'untracked' };
  const sort = { [KeyPath]: 1 };
  const makeHref = makeHrefs.fsoRootList;
  return {
    isReady,
    links: homeLinks.find(sel, { sort }).fetch(),
    makeHref,
  };
}

const FsoDiscoverContainer = withTracker(FsoDiscoverTracker)(FsoDiscover);

FsoDiscoverContainer.propTypes = {
  nogFso: PropTypes.object.isRequired,
  makeHrefs: PropTypes.shape({
    fsoListing: PropTypes.func.isRequired,
  }).isRequired,
};

function FsoCatalogs({
  isReady, links, makeHref,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }

  return (
    <div className="row">
      <div className="col-md-12">
        <h4>Catalogs</h4>
        <p className="help-block">
          Access catalogs at the links below.
        </p>
        <LinkList links={links} makeHref={makeHref} />
      </div>
    </div>
  );
}

FsoCatalogs.propTypes = {
  isReady: PropTypes.bool.isRequired,
  links: PropTypes.arrayOf(PropTypes.object).isRequired,
  makeHref: PropTypes.func.isRequired,
};

function FsoCatalogsTracker({
  nogFso, makeHrefs,
}) {
  const {
    subscribeHome, homeLinks,
  } = nogFso;
  const sub = subscribeHome();
  const isReady = sub.ready();
  const sel = { [KeyRoute]: 'catalog' };
  const sort = { [KeyPath]: 1 };
  const makeHref = makeHrefs.fsoCatalog;
  return {
    isReady,
    links: homeLinks.find(sel, { sort }).fetch(),
    makeHref,
  };
}

const FsoCatalogsContainer = withTracker(FsoCatalogsTracker)(FsoCatalogs);

FsoCatalogsContainer.propTypes = {
  nogFso: PropTypes.object.isRequired,
  makeHrefs: PropTypes.shape({
    fsoCatalog: PropTypes.func.isRequired,
  }).isRequired,
};

// XXX The layout should be rearranged, perhaps into tabs like the V1 home.  We
// should add a mechanism to manage per-user favorites.
function Layout({
  nogFso, makeHrefs,
}) {
  return (
    <Fragment>
      <FsoBrowseContainer
        nogFso={nogFso}
        makeHrefs={makeHrefs}
      />
      <FsoDiscoverContainer
        nogFso={nogFso}
        makeHrefs={makeHrefs}
      />
      <FsoCatalogsContainer
        nogFso={nogFso}
        makeHrefs={makeHrefs}
      />
    </Fragment>
  );
}

function Gate({
  isReady, mayAccess, nogFso, makeHrefs,
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
      makeHrefs={makeHrefs}
    />
  );
}

Gate.propTypes = {
  isReady: PropTypes.bool.isRequired,
  mayAccess: PropTypes.bool.isRequired,
  nogFso: PropTypes.object.isRequired,
  makeHrefs: PropTypes.shape({
    fsoListing: PropTypes.func.isRequired,
    fsoRootList: PropTypes.func.isRequired,
    fsoCatalog: PropTypes.func.isRequired,
  }).isRequired,
};

function trimSlashes(s) {
  return s.replace(/^\/*/, '').replace(/\/*$/, '');
}

function GateTracker({ router, routes, nogFso }) {
  const makeHrefs = {
    fsoListing(path) {
      return router.path(routes.fsoListing, {
        path: trimSlashes(path),
      });
    },
    fsoRootList(path) {
      return router.path(routes.fsoRootList, {
        prefix: trimSlashes(path),
      });
    },
    fsoCatalog(path) {
      return router.path(routes.fsoCatalog, {
        repoPath: trimSlashes(path),
      });
    },
  };

  const mayHome = nogFso.testAccess(AA_HOME, { path: '/' });
  const isReady = (mayHome !== null);
  return {
    isReady,
    mayAccess: !!mayHome,
    makeHrefs,
    nogFso,
  };
}

const NogHomeContainer = withTracker(GateTracker)(Gate);

NogHomeContainer.propTypes = {
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    fsoListing: PropTypes.string.isRequired,
    fsoRootList: PropTypes.string.isRequired,
  }).isRequired,
  nogFso: PropTypes.shape({
    testAccess: PropTypes.func.isRequired,
  }).isRequired,
};

export {
  NogHomeContainer,
};
