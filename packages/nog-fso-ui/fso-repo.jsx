/* eslint-disable react/forbid-prop-types */
/* eslint-disable react/no-multi-comp */

import { Random } from 'meteor/random';
import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import Blaze from 'meteor/gadicc:blaze-react-component';
import { withTracker } from 'meteor/react-meteor-data';
import {
  KeyName,
  KeyRoute,
} from 'meteor/nog-fso';
import { FsoFilesContainer } from './fso-files.jsx';
import { MetadataForm } from './metadata-form.jsx';
import { Markdown } from './markdown.jsx';
import { RepoBreadcrumbs } from './breadcrumbs.jsx';
import { TarttTars } from './tartt-tars.jsx';

const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_READ_REPO_TREE = 'fso/read-repo-tree';
const AA_FSO_REFRESH_REPO = 'fso/refresh-repo';
const AA_FSO_INIT_REPO = 'fso/init-repo';
const AA_FSO_WRITE_REPO = 'fso/write-repo';

function ErrorsAlert({ errors, onClickClose }) {
  if (errors.length === 0) {
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
          {errors.map(err => (
            <p key={err.id}><strong>{err.title}:</strong> {err.message}</p>
          ))}
        </div>
      </div>
    </div>
  );
}

ErrorsAlert.propTypes = {
  errors: PropTypes.arrayOf(PropTypes.shape({
    id: PropTypes.string.isRequired,
    title: PropTypes.string.isRequired,
    message: PropTypes.string.isRequired,
  })).isRequired,
  onClickClose: PropTypes.func.isRequired,
};

// Deprecated: `MetaForm` is going to be replaced by `MetadataForm`.
class MetaForm extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      keywords: props.meta.get('keywords') || '',
    };
    this.handleClickSaveMeta = this.handleClickSaveMeta.bind(this);
    this.handleChangeValue = this.handleChangeValue.bind(this);
  }

  handleClickSaveMeta(ev) {
    ev.preventDefault();
    const { onSubmitSaveMeta, repoId, repoPath } = this.props;
    const meta = new Map(this.props.meta);
    Object.entries(this.state).forEach(([k, v]) => {
      if (v) {
        meta.set(k, v);
      }
    });
    onSubmitSaveMeta({ repoId, repoPath, meta });
  }

  handleChangeValue(ev) {
    ev.preventDefault();
    this.setState({
      [ev.target.name]: ev.target.value,
    });
  }

  render() {
    const { handleClickSaveMeta, handleChangeValue } = this;
    const { disabled } = this.props;
    const { keywords } = this.state;
    return (
      <div className="row">
        <div className="col-md-12">
          <form className="form-inline">
            <button
              type="submit"
              className="btn btn-primary"
              onClick={handleClickSaveMeta}
              disabled={disabled}
            >Save
            </button>
            <div className="form-group">
              <label htmlFor="keywordsInput">Keywords</label>
              <input
                type="text"
                className="form-control"
                id="keywordsInput"
                placeholder="keywords, ..."
                disabled={disabled}
                name="keywords"
                value={keywords}
                onChange={handleChangeValue}
              />
            </div>
          </form>
        </div>
      </div>
    );
  }
}

// `maxDateObject()` takes a list of objects `{ date, ... }` and returns the
// object with the largest `date`.  It returns an empty object if no `date` is
// larger than `Date(0)`.
function maxDateObject(objs) {
  let ret = {};
  let lastDate = new Date(0);
  for (const o of objs) {
    if (o.date && o.date > lastDate) {
      lastDate = o.date;
      ret = o;
    }
  }
  return ret;
}

function fmtLastChange(repo) {
  const { what, date, author } = maxDateObject([
    {
      what: 'stat',
      date: repo.statDate(),
      author: repo.statAuthor(),
    },
    {
      what: 'content',
      date: repo.contentDate(),
      author: repo.contentAuthor(),
    },
    {
      what: 'metadata',
      date: repo.metaDate(),
      author: repo.metaAuthor(),
    },
  ]);
  if (!author || !date) {
    return 'unknown';
  }
  return `updated ${what}, recorded ${date.toUTCString()} by ${author}`;
}

function fmtLastRefresh(repo) {
  const { date } = maxDateObject([
    { date: repo.statRequestTime() },
    { date: repo.refreshContentRequestTime() },
  ]);
  if (!date) {
    return 'unknown';
  }
  return date.toUTCString();
}

MetaForm.propTypes = {
  repoId: PropTypes.string.isRequired,
  repoPath: PropTypes.string.isRequired,
  onSubmitSaveMeta: PropTypes.func.isRequired,
  disabled: PropTypes.bool.isRequired,
  meta: PropTypes.object.isRequired,
};

function Details({
  repoName, isReady, repo,
  disableRefreshReason, onSubmitRefreshStat, onSubmitRefreshContent,
  disabledReinitReason, onReinitSubdirTracking,
  disableSaveReason, onSubmitSaveMeta, onExternalChange,
  metaSubmission,
  onUpdateCatalogs,
  filesContainer,
  nogSuggest, sugnss,
  errors, onClearErrors,
  tarttHeads, repoTars, subscribeTartt,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }

  if (!repo) {
    return <div>Unknown repo {repoName}.</div>;
  }

  function summaryText() {
    const s = repo.filesSummary();
    return `${s.nFiles} files or directory summaries, ${s.nDirs} directories`;
  }

  function statRequestTimeText() {
    const t = repo.statRequestTime();
    if (t) {
      return t.toUTCString();
    }
    return 'unknown';
  }

  function refreshContentRequestTimeText() {
    const t = repo.refreshContentRequestTime();
    if (t) {
      return t.toUTCString();
    }
    return 'unknown';
  }

  function errorAlert() {
    const msg = repo.errorMessage();
    if (!msg) {
      return null;
    }
    return <div className="alert alert-warning" role="alert">{msg}</div>;
  }

  function metaText() {
    const m = Array.from(repo.metadata());
    if (m.length === 0) {
      return 'None';
    }
    return m.map(([k, v]) => `${k}: ${JSON.stringify(v)}`).join(', ');
  }

  function handleClickRefreshStat(ev) {
    ev.preventDefault();
    onSubmitRefreshStat({
      repoId: repo.id(),
      repoPath: repo.path(),
    });
  }

  function handleClickRefreshContent(ev) {
    ev.preventDefault();
    onSubmitRefreshContent({
      repoId: repo.id(),
      repoPath: repo.path(),
    });
  }

  function handleClickReinit(ev, subdirTracking) {
    ev.preventDefault();
    onReinitSubdirTracking({
      repoId: repo.id(),
      repoPath: repo.path(),
      subdirTracking,
    });
  }

  function handleClickCloseErrors(ev) {
    ev.preventDefault();
    onClearErrors();
  }

  function fmtBy(author, date) {
    if (!author || !date) {
      return null;
    }
    return `${date.toUTCString()} by ${author}`;
  }

  // It seems useful to display disabled buttons and explain why during
  // preview.  We should reconsider later whether to hide the buttons without
  // explanation.
  const buttons = () => {
    const disabled = disableRefreshReason ? 'disabled' : null;
    const disabledReinit = disabledReinitReason ? 'disabled' : null;
    return (
      <div className="row">
        <div className="col-md-8">
          <p>
            <button
              type="button"
              className="btn btn-primary btn-xs"
              disabled={disabled}
              onClick={handleClickRefreshStat}
            >Refresh and Commit Stat
            </button>
            {' '}
            <button
              type="button"
              className="btn btn-primary btn-xs"
              disabled={disabled}
              onClick={handleClickRefreshContent}
            >Refresh and Commit Content
            </button>
          </p>
          <p className="help-block">{disableRefreshReason}</p>
        </div>
        <div className="col-md-4">
          <p className="text-right">
            <button
              type="button"
              className="btn btn-warning btn-xs"
              disabled={disabledReinit}
              onClick={ev => handleClickReinit(ev, 'enter-subdirs')}
            >Track Directory Details
            </button>
            {' '}
            <button
              type="button"
              className="btn btn-warning btn-xs"
              disabled={disabledReinit}
              onClick={ev => handleClickReinit(ev, 'bundle-subdirs')}
            >Track Directory Summaries
            </button>
            {' '}
            <button
              type="button"
              className="btn btn-warning btn-xs"
              disabled={disabledReinit}
              onClick={ev => handleClickReinit(ev, 'ignore-most')}
            >Track Toplevel
            </button>
          </p>
          <p className="help-block text-right">{disabledReinitReason}</p>
        </div>
      </div>
    );
  };

  const metaForm = () => {
    if (disableSaveReason) {
      return (
        <div className="row">
          <div className="col-md-12">
            <p className="help-block">{disableSaveReason}</p>
          </div>
        </div>
      );
    }
    const metaCommit = {
      commitId: repo.metaCommitId(),
      author: repo.metaAuthor(),
      date: repo.metaDate(),
    };
    return (
      <MetadataForm
        metaIsUpdating={repo.metaIsUpdating()}
        metaIsSaving={metaSubmission.isSaving}
        repoId={repo.id()}
        repoPath={repo.path()}
        onExternalChange={onExternalChange}
        onSubmitSaveMeta={onSubmitSaveMeta}
        committedMetaCommit={metaSubmission.commit}
        onUpdateCatalogs={onUpdateCatalogs}
        receivedMetaCommit={metaCommit}
        receivedMetadata={repo.metadata()}
        nogSuggest={nogSuggest}
        sugnss={sugnss}
      />
    );
  };

  function readme() {
    const text = repo.readme();
    if (!text) {
      return null;
    }

    return (
      <div className="row">
        <div className="col-md-12">
          <div className="panel panel-default">
            <div className="panel-heading">
              <h3 className="panel-title">README</h3>
            </div>
            <div className="panel-body">
              <Markdown source={text} />
            </div>
          </div>
        </div>
      </div>
    );
  }

  function statStatus() {
    const st = repo.statStatus();
    if (!st) {
      return (
        <small>Stat status unknown.</small>
      );
    }

    const tsTxt = st.ts.toGMTString();

    if (repo.statStatusIsClean()) {
      return (
        <small>Stat is up to date, last checked {tsTxt}.</small>
      );
    }

    const counts = [];
    const { nNew, nModified, nDeleted } = st;
    [
      { n: nNew, what: 'new' },
      { n: nModified, what: 'modified' },
      { n: nDeleted, what: 'deleted' },
    ].forEach(({ n, what }) => {
      if (n > 0) {
        counts.push(`${n} ${what}`);
      }
    });

    const changes = st.changes.map(({ path, status }) => {
      let statusTxt;
      switch (status) {
        case '?': statusTxt = '     new'; break;
        case 'M': statusTxt = 'modified'; break;
        case 'D': statusTxt = ' deleted'; break;
        default: statusTxt = ' unknown'; break;
      }
      return `${statusTxt}:  ${path}`;
    });
    if (!st.changesIsComplete) {
      changes.push(`...`);
    }

    return (
      <Fragment>
        <small>
          <a
            role="button"
            data-toggle="collapse"
            href="#collapseStatDetails"
            aria-expanded="false"
            aria-controls="collapseStatDetails"
          >
            Uncommitted stat changes:
          </a>
          {' '}{counts.join(', ')}, last checked {tsTxt}.
        </small>
        <div className="collapse" id="collapseStatDetails">
          <pre className="small">{changes.join('\n')}</pre>
        </div>
      </Fragment>
    );
  }

  return (
    <Fragment>
      <div className="row">
        <div className="col-md-12">
          {errorAlert()}
        </div>
      </div>
      <div className="row">
        { repo.filesSummaryExists() ? (
          <div className="col-xs-12">
            {summaryText()}
          </div>
        ) : null }
        <div className="col-xs-12">
          { repo.gitNogCommitExists() ? (
            <small>
              Last change {fmtLastChange(repo)},{' '}
              last refresh {fmtLastRefresh(repo)}
            </small>
          ) : null }
        </div>
        { repo.statStatusExists() ? (
          <div className="col-xs-12">
            {statStatus()}
          </div>
        ) : null }
      </div>
      {buttons()}
      <ErrorsAlert
        errors={errors}
        onClickClose={handleClickCloseErrors}
      />
      {repo.metadataExists() && repo.gitNogCommitExists() ? metaForm() : null}
      {repo.readmeExists() ? readme() : null}
      {filesContainer()}
      <div className="row">
        <div className="col-md-12">
          <h3>
            <a
              data-toggle="collapse"
              href="#technicalDetails"
              aria-expanded="false"
              aria-controls="technicalDetails"
            >
              Technical...
            </a>
          </h3>
          <div
            className="collapse"
            id="technicalDetails"
          >
            <dl>
              <dt>FSO ID</dt>
              <dd>{repo.fsoIdString()}</dd>
              { repo.gitNogCommitExists() ? (
                <Fragment>
                  <dt>Content Summary</dt>
                  { repo.filesSummaryExists() ? (
                    <dd>{summaryText()}</dd>
                  ) : null }
                  <dt>Refresh Stat Last Requested</dt>
                  <dd>{statRequestTimeText()}</dd>
                  <dt>Refresh Content Last Requested</dt>
                  <dd>{refreshContentRequestTimeText()}</dd>

                  <dt>GitNog Commit</dt>
                  <dd>{repo.gitNogCommitId()}</dd>
                  <dt>Stat Last Modified</dt>
                  <dd>{fmtBy(repo.statAuthor(), repo.statDate())}</dd>
                  <dt>Sha Last Modified</dt>
                  <dd>{fmtBy(repo.shaAuthor(), repo.shaDate())}</dd>
                  <dt>Content Last Modified</dt>
                  <dd>{fmtBy(repo.contentAuthor(), repo.contentDate())}</dd>
                  <dt>Meta Last Modified</dt>
                  <dd>{fmtBy(repo.metaAuthor(), repo.metaDate())}</dd>
                </Fragment>
              ) : null }
              { repo.metadataExists() ? (
                <Fragment>
                  <dt>Meta</dt>
                  <dd>{metaText()}</dd>
                </Fragment>
              ) : null }
            </dl>
            <TarttTars
              repoId={repo.id()}
              repoPath={repoName}
              tarttHeads={tarttHeads}
              repoTars={repoTars}
              subscribeTartt={subscribeTartt}
            />
          </div>
        </div>
      </div>
    </Fragment>
  );
}

Details.propTypes = {
  repoName: PropTypes.string.isRequired,
  isReady: PropTypes.bool.isRequired,
  repo: PropTypes.object,
  disableRefreshReason: PropTypes.string.isRequired,
  onExternalChange: PropTypes.func.isRequired,
  onSubmitRefreshStat: PropTypes.func.isRequired,
  onSubmitRefreshContent: PropTypes.func.isRequired,
  disabledReinitReason: PropTypes.string.isRequired,
  onReinitSubdirTracking: PropTypes.func.isRequired,
  disableSaveReason: PropTypes.string.isRequired,
  onSubmitSaveMeta: PropTypes.func.isRequired,
  metaSubmission: PropTypes.object.isRequired,
  onUpdateCatalogs: PropTypes.func.isRequired,
  filesContainer: PropTypes.func.isRequired,
  nogSuggest: PropTypes.object.isRequired,
  sugnss: PropTypes.array.isRequired,
  errors: PropTypes.arrayOf(PropTypes.object).isRequired,
  onClearErrors: PropTypes.func.isRequired,
  // Tartt
  tarttHeads: PropTypes.object.isRequired,
  repoTars: PropTypes.object.isRequired,
  subscribeTartt: PropTypes.func.isRequired,
};

Details.defaultProps = {
  repo: null,
};

function DetailsTracker({
  repoName, repos, subscribeRepo,
}) {
  const sub = subscribeRepo({ repoName });
  const isReady = sub.ready();
  return {
    repoName,
    isReady,
    repo: repos.findOne({ [KeyName]: repoName }),
  };
}

const DetailsContainer = withTracker(DetailsTracker)(Details);

DetailsContainer.propTypes = {
  repoName: PropTypes.string.isRequired,
  repos: PropTypes.object.isRequired,
  subscribeRepo: PropTypes.func.isRequired,
};

class Layout extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      errors: [],
      metaSubmission: {
        isSaving: false,
        commit: {},
      },
    };

    this.clearErrors = this.clearErrors.bind(this);
    this.setConcurrentChangeError = this.setConcurrentChangeError.bind(this);
    this.refreshStat = this.refreshStat.bind(this);
    this.refreshContent = this.refreshContent.bind(this);
    this.reinitSubdirTracking = this.reinitSubdirTracking.bind(this);
    this.saveMeta = this.saveMeta.bind(this);
    this.updateCatalogs = this.updateCatalogs.bind(this);
  }

  // It's unclear why ESLint complains.  The order seems fine.
  // eslint-disable-next-line react/sort-comp
  clearErrors() {
    this.setState({ errors: [] });
  }

  setConcurrentChangeError(err) {
    this.setState(st => ({
      errors: [...st.errors, {
        id: Random.id(),
        title: err.title,
        message: err.message,
      }],
    }));
  }

  refreshStat(opts) {
    const { callUpdateStat } = this.props.nogFso;
    callUpdateStat(opts, (err) => {
      if (err) {
        this.setState(st => ({
          errors: [...st.errors, {
            id: Random.id(),
            title: 'Refresh stat failed',
            message: err.message,
          }],
        }));
      }
    });
  }

  refreshContent(opts) {
    const { callRefreshContent } = this.props.nogFso;
    callRefreshContent(opts, (err) => {
      if (err) {
        this.setState(st => ({
          errors: [...st.errors, {
            id: Random.id(),
            title: 'Refresh content failed',
            message: err.message,
          }],
        }));
      }
    });
  }

  reinitSubdirTracking(opts) {
    const { callReinitSubdirTracking } = this.props.nogFso;
    callReinitSubdirTracking(opts, (err) => {
      if (err) {
        this.setState(st => ({
          errors: [...st.errors, {
            id: Random.id(),
            title: 'Reinit directory tracking failed',
            message: err.message,
          }],
        }));
      }
    });
  }

  saveMeta({ repoId, repoPath, meta }) {
    const { callStoreMeta } = this.props.nogFso;
    const m = {};
    for (const [k, v] of meta.entries()) {
      m[k] = v;
    }
    this.setState({
      metaSubmission: {
        isSaving: true,
        commit: {},
      },
    });
    callStoreMeta({ repoId, repoPath, meta: m }, (err, res) => {
      if (err) {
        this.setState(st => ({
          errors: [...st.errors, {
            id: Random.id(),
            title: 'Save metadata failed',
            message: err.message,
          }],
        }));
      }
      this.setState({
        metaSubmission: {
          isSaving: false,
          commit: {
            author: res.metaAuthor.name,
            date: new Date(res.metaAuthor.date),
            commitId: res.metaCommitId,
          },
        },
      });
    });
  }

  updateCatalogs({ repoId, repoPath }) {
    const { callTriggerUpdateCatalogs } = this.props.nogFso;
    callTriggerUpdateCatalogs({ repoId, repoPath }, (err) => {
      if (err) {
        this.setState(st => ({
          errors: [...st.errors, {
            id: Random.id(),
            title: 'Trigger update catalogs failed',
            message: err.message,
          }],
        }));
      }
    });
  }

  render() {
    const {
      repoName, nogFso, mayAccessTree, mayRefreshRepo, mayReinit, mayWrite,
      nogSuggest, sugnss,
      makeHrefLsPath,
    } = this.props;
    const {
      subscribeRepo, repos, files, treeErrors, subscribeTree,
      tarttHeads, repoTars, subscribeTartt,
    } = nogFso;
    const {
      errors, metaSubmission,
    } = this.state;
    const {
      clearErrors, refreshStat, refreshContent, reinitSubdirTracking, saveMeta,
      setConcurrentChangeError, updateCatalogs,
    } = this;

    const filesContainer = () => {
      if (!mayAccessTree) {
        return (
          <div className="alert alert-info" role="alert">
              File listing access denied.
          </div>
        );
      }

      return (
        <FsoFilesContainer
          repoName={repoName}
          files={files}
          treeErrors={treeErrors}
          subscribeTree={subscribeTree}
        />
      );
    };

    let disableRefreshReason = '';
    if (!mayRefreshRepo) {
      disableRefreshReason = 'You have no permission to refresh.';
    }

    let disableSaveReason = '';
    if (!mayWrite) {
      disableSaveReason = 'You have no permission to change the metadata.';
    }

    const disabledReinitReason = mayReinit ? '' : (
      'You have no permission to reinitialize the repo.'
    );

    return (
      <Fragment>
        <div className="row">
          <div className="col-md-12">
            <h4>
              Repo
              {' '}
              <RepoBreadcrumbs
                path={repoName}
                makeHrefLsPath={makeHrefLsPath}
              />
            </h4>
          </div>
        </div>
        <DetailsContainer
          repoName={repoName}
          repos={repos}
          subscribeRepo={subscribeRepo}
          disableRefreshReason={disableRefreshReason}
          onSubmitRefreshStat={refreshStat}
          onSubmitRefreshContent={refreshContent}
          disabledReinitReason={disabledReinitReason}
          onReinitSubdirTracking={reinitSubdirTracking}
          disableSaveReason={disableSaveReason}
          onExternalChange={setConcurrentChangeError}
          onSubmitSaveMeta={saveMeta}
          metaSubmission={metaSubmission}
          onUpdateCatalogs={updateCatalogs}
          filesContainer={filesContainer}
          nogSuggest={nogSuggest}
          sugnss={sugnss}
          errors={errors}
          onClearErrors={clearErrors}
          tarttHeads={tarttHeads}
          repoTars={repoTars}
          subscribeTartt={subscribeTartt}
        />
      </Fragment>
    );
  }
}

Layout.propTypes = {
  repoName: PropTypes.string.isRequired,
  mayAccessTree: PropTypes.bool.isRequired,
  mayRefreshRepo: PropTypes.bool.isRequired,
  mayReinit: PropTypes.bool.isRequired,
  mayWrite: PropTypes.bool.isRequired,
  nogFso: PropTypes.shape({
    repos: PropTypes.object.isRequired,
    subscribeRepo: PropTypes.func.isRequired,
    callUpdateStat: PropTypes.func.isRequired,
    callRefreshContent: PropTypes.func.isRequired,
    callStoreMeta: PropTypes.func.isRequired,
    callReinitSubdirTracking: PropTypes.func.isRequired,
    callTriggerUpdateCatalogs: PropTypes.func.isRequired,
    files: PropTypes.object.isRequired,
    treeErrors: PropTypes.object.isRequired,
    subscribeTree: PropTypes.func.isRequired,
  }).isRequired,
  nogSuggest: PropTypes.object.isRequired,
  sugnss: PropTypes.array.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
};

function Gate({
  repoName, nogFso, nogSuggest, sugnss,
  isReady, mayAccess, mayAccessTree, mayRefreshRepo, mayReinit, mayWrite,
  makeHrefLsPath,
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
      mayAccessTree={mayAccessTree}
      mayRefreshRepo={mayRefreshRepo}
      mayReinit={mayReinit}
      mayWrite={mayWrite}
      nogFso={nogFso}
      nogSuggest={nogSuggest}
      sugnss={sugnss}
      makeHrefLsPath={makeHrefLsPath}
    />
  );
}

Gate.propTypes = {
  isReady: PropTypes.bool.isRequired,
  mayAccess: PropTypes.bool.isRequired,
  mayAccessTree: PropTypes.bool.isRequired,
  mayRefreshRepo: PropTypes.bool.isRequired,
  mayReinit: PropTypes.bool.isRequired,
  mayWrite: PropTypes.bool.isRequired,
  repoName: PropTypes.string.isRequired,
  nogFso: PropTypes.object.isRequired,
  nogSuggest: PropTypes.object.isRequired,
  sugnss: PropTypes.array.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
};

function trimSlashes(s) {
  return s.replace(/^\/*/, '').replace(/\/*$/, '');
}

function GateTracker({
  router, routes, nogFso, nogSuggest, nogHome,
}) {
  function makeHrefLsPath(path) {
    return router.path(routes.fsoListing, {
      path: trimSlashes(path),
    });
  }

  // XXX Maybe add a separate subscription instead of using pseudo home links.
  const { subscribeHome, homeLinks } = nogHome;
  subscribeHome();
  const selSugNs = { [KeyRoute]: 'syssug' };
  const sugnss = homeLinks.find(selSugNs).map(d => d.path());

  const repoName = `/${router.getParam('repoName')}`;
  const { testAccess } = nogFso;
  const mayRead = testAccess(AA_FSO_READ_REPO, { path: repoName });
  const mayTree = testAccess(AA_FSO_READ_REPO_TREE, { path: repoName });
  const mayRefresh = testAccess(AA_FSO_REFRESH_REPO, { path: repoName });
  const mayReinit = testAccess(AA_FSO_INIT_REPO, { path: repoName });
  const mayWrite = testAccess(AA_FSO_WRITE_REPO, { path: repoName });
  const isReady = (
    (mayRead !== null) &&
    (mayTree !== null) &&
    (mayRefresh !== null) &&
    (mayWrite !== null)
  );
  return {
    isReady,
    mayAccess: !!mayRead,
    mayAccessTree: !!mayTree,
    mayRefreshRepo: !!mayRefresh,
    mayReinit: !!mayReinit,
    mayWrite: !!mayWrite,
    repoName,
    router,
    nogFso,
    nogSuggest,
    sugnss,
    makeHrefLsPath,
  };
}

const FsoRepoGateContainer = withTracker(GateTracker)(Gate);

FsoRepoGateContainer.propTypes = {
  router: PropTypes.object.isRequired,
  routes: PropTypes.shape({
    fsoListing: PropTypes.string.isRequired,
    fsoRepo: PropTypes.string.isRequired,
  }).isRequired,
  nogFso: PropTypes.shape({
    testAccess: PropTypes.func.isRequired,
  }).isRequired,
  nogHome: PropTypes.shape({
    subscribeHome: PropTypes.func.isRequired,
    homeLinks: PropTypes.object.isRequired,
  }).isRequired,
  nogSuggest: PropTypes.object.isRequired,
};

export {
  FsoRepoGateContainer,
};
