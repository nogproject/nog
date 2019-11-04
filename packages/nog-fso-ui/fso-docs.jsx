/* eslint-disable react/forbid-prop-types */

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import Blaze from 'meteor/gadicc:blaze-react-component';
import { withTracker } from 'meteor/react-meteor-data';
import {
  KeyRepoName,
  KeyTreePath,
} from 'meteor/nog-fso';
import { Markdown } from './markdown.jsx';

const AA_FSO_READ_REPO_TREE = 'fso/read-repo-tree';

function ErrAlert({ message }) {
  return (
    <div className="alert alert-warning" role="alert">
      <span>
        {message} <strong>Reloading may fix transient errors.</strong>
      </span>
    </div>
  );
}

ErrAlert.propTypes = {
  message: PropTypes.string.isRequired,
};

function ContentTrackee({
  isReady, content, errors,
}) {
  if (!isReady) {
    return (
      <div />
    );
  }

  const alerts = errors.map(e => (
    <ErrAlert key={e.id()} message={e.message()} />
  ));

  if (!content) {
    return (
      <div className="row">
        <div className="col-md-12">
          {alerts}
        </div>
      </div>
    );
  }

  const text = content.text();

  return (
    <div className="row">
      <div className="col-md-12">
        {alerts}
        <Markdown source={text} />
      </div>
    </div>
  );
}

ContentTrackee.propTypes = {
  isReady: PropTypes.bool.isRequired,
  content: PropTypes.object,
  errors: PropTypes.arrayOf(PropTypes.object).isRequired,
};
ContentTrackee.defaultProps = {
  content: null,
};

function ContentTracker({
  repoName, treePath,
  content, subscribeTreePathContent,
  treeErrors,
}) {
  const sub = subscribeTreePathContent({ repoName, treePath });
  const isReady = sub.ready();
  return {
    repoName,
    treePath,
    isReady,
    content: content.findOne({
      [KeyRepoName]: repoName,
      [KeyTreePath]: treePath,
    }),
    errors: treeErrors.find().fetch(),
  };
}

const Content = withTracker(ContentTracker)(ContentTrackee);

Content.propTypes = {
  repoName: PropTypes.string.isRequired,
  treePath: PropTypes.string.isRequired,
  content: PropTypes.object.isRequired,
  subscribeTreePathContent: PropTypes.func.isRequired,
  treeErrors: PropTypes.object.isRequired,
};

function Layout({
  repoName, treePath, nogFso,
}) {
  const {
    subscribeTreePathContent, content,
    treeErrors,
  } = nogFso;
  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          <h4>
            Document {treePath} in repo {repoName}
          </h4>
        </div>
      </div>
      <Content
        repoName={repoName}
        treePath={treePath}
        content={content}
        subscribeTreePathContent={subscribeTreePathContent}
        treeErrors={treeErrors}
      />
    </Fragment>
  );
}

Layout.propTypes = {
  repoName: PropTypes.string.isRequired,
  treePath: PropTypes.string.isRequired,
  nogFso: PropTypes.shape({
    content: PropTypes.object.isRequired,
    subscribeTreePathContent: PropTypes.func.isRequired,
    treeErrors: PropTypes.object.isRequired,
  }).isRequired,
};

function GateTrackee({
  isReady, mayAccess,
  repoName, treePath,
  nogFso,
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
      repoName={repoName}
      treePath={treePath}
      nogFso={nogFso}
    />
  );
}

GateTrackee.propTypes = {
  isReady: PropTypes.bool.isRequired,
  mayAccess: PropTypes.bool.isRequired,
  repoName: PropTypes.string.isRequired,
  treePath: PropTypes.string.isRequired,
  nogFso: PropTypes.object.isRequired,
};

function GateTracker({
  router, nogFso,
}) {
  const repoName = `/${router.getParam('repoPath')}`;
  const treePath = router.getParam('treePath') || 'index.md';

  const { testAccess } = nogFso;
  const mayRead = testAccess(AA_FSO_READ_REPO_TREE, { path: repoName });
  const isReady = (mayRead !== null);

  return {
    isReady,
    mayAccess: !!mayRead,
    repoName,
    treePath,
    nogFso,
  };
}

const FsoDocsGate = withTracker(GateTracker)(GateTrackee);

FsoDocsGate.propTypes = {
  router: PropTypes.object.isRequired,
  // `routes` will be used to create links when rendering Markdown.
  routes: PropTypes.shape({
    fsoDocs: PropTypes.string.isRequired,
  }).isRequired,
  nogFso: PropTypes.shape({
    testAccess: PropTypes.func.isRequired,
  }).isRequired,
};

export {
  FsoDocsGate,
};
