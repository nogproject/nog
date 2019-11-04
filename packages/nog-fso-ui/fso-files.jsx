/* eslint-disable react/forbid-prop-types */

import React from 'react';
const { Fragment } = React;
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import {
  KeyRepoName,
  KeyTreePath,
} from 'meteor/nog-fso';

// `nbsp()` replaces all spaces by non-breakable ones.  See
// <https://stackoverflow.com/a/24437562>.
function nbsp(s) {
  return s.replace(/ /g, '\u00a0');
}

function ErrAlert({ message }) {
  return (
    <div className="alert alert-warning" role="alert">
      <span>{message} </span>
      <strong>The file list may be incomplete. Consider reloading.</strong>
    </div>
  );
}

ErrAlert.propTypes = {
  message: PropTypes.string.isRequired,
};

function Files({
  isReady, files, errors,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }

  function summaryText() {
    const s = (files.length === 1) ? '' : 's';
    return nbsp(`${files.length} path${s}`);
  }

  function fmtPath(f) {
    let p = f.path();
    if (f.isDir() || f.isGitlink() || f.isNogbundle()) {
      p += '/';
    }
    return p;
  }

  function fileTypeDetail(f) {
    if (f.isRegular()) {
      return nbsp(`${f.size()} B`);
    }
    if (f.isSymlink()) {
      return nbsp(`-> ${f.symlink()}`);
    }
    const details = [];
    if (f.isGitlink()) {
      details.push(`@${f.gitlink().substr(0, 8)}`);
    }
    if (f.isNogbundle() || f.isGitlink()) {
      for (const [name, val] of [
        ['dir', f.dirs()],
        ['file', f.files()],
        ['link', f.links()],
        ['other', f.others()],
      ]) {
        if (val > 0) {
          const s = (val === 1) ? '' : 's';
          details.push(nbsp(`${val} ${name}${s}`));
        }
      }
      if (f.size() > 0) {
        details.push(nbsp(`${f.size()} B`));
      }
    }
    return details.join(', ');
  }

  function metadataCollapseId(f) {
    return `meta${f.id()}`;
  }

  function fmtMetadataLink(f) {
    if (!f.meta()) {
      return '';
    }

    function truncate(s) {
      const max = 60;
      if (s.length < max) {
        return s;
      }
      return `${s.substr(0, max - 4).trim()}${nbsp(' ...')}`;
    }

    const metaKv = f.meta();
    const summary = truncate(
      `Metadata: ${Object.keys(metaKv).sort().join(', ')}`,
    );
    const htmlId = metadataCollapseId(f);
    return (
      <Fragment>
        <a
          role="button"
          data-toggle="collapse"
          href={`#${htmlId}`}
          aria-expanded="false"
        >
          {summary}
        </a>
      </Fragment>
    );
  }

  function fmtMetadataDetails(f) {
    if (!f.meta()) {
      return null;
    }

    function fmtValue(v) {
      if (typeof v === 'string') {
        return v;
      }

      if (Array.isArray(v) && v.every(e => typeof e === 'string')) {
        return v.join(', ');
      }

      return JSON.stringify(v);
    }

    const metaKv = f.meta();
    const htmlId = metadataCollapseId(f);
    return (
      <tr>
        <td colSpan="4">
          <div className="collapse" id={htmlId}>
            <dl>
              {Object.keys(metaKv).sort().map(k => (
                <Fragment key={k}>
                  <dt>{k}</dt>
                  <dd>{fmtValue(metaKv[k])}</dd>
                </Fragment>
              ))}
            </dl>
          </div>
        </td>
      </tr>
    );
  }

  function fmtMtime(f) {
    if (f.isSymlink()) {
      return '';
    }
    return f.mtime().format();
  }

  const items = files.map((f) => {
    if (f.stat()) {
      return (
        <Fragment key={f.id()}>
          <tr>
            <td>{fmtPath(f)}</td>
            <td>{fileTypeDetail(f)}</td>
            <td>{fmtMtime(f)}</td>
            <td>{fmtMetadataLink(f)}</td>
          </tr>
          {fmtMetadataDetails(f)}
        </Fragment>
      );
    }
    return (
      <Fragment key={f.id()}>
        <tr className="warning">
          <td>{f.path()}</td>
          <td>metadata only</td>
          <td />
          <td>{fmtMetadataLink(f)}</td>
        </tr>
        {fmtMetadataDetails(f)}
      </Fragment>
    );
  });

  const errorAlerts = errors.map(e => (
    <ErrAlert key={e.id()} message={e.message()} />
  ));

  return (
    <div className="row">
      <div className="col-md-12">
        <div className="panel panel-default">
          <div className="panel-heading">
            <h3 className="panel-title">{summaryText()}</h3>
          </div>
          <div className="panel-body">
            {errorAlerts}
            <table className="table table-condensed">
              <tbody>
                {items}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}

Files.propTypes = {
  isReady: PropTypes.bool.isRequired,
  files: PropTypes.arrayOf(PropTypes.object).isRequired,
  errors: PropTypes.arrayOf(PropTypes.object).isRequired,
};

function FilesTracker({
  repoName, files, treeErrors, subscribeTree,
}) {
  const sub = subscribeTree({ repoName });
  const isReady = sub.ready();
  const sort = { [KeyTreePath]: 1 };
  return {
    isReady,
    files: files.find({ [KeyRepoName]: repoName }, { sort }).fetch(),
    errors: treeErrors.find().fetch(),
  };
}

const FsoFilesContainer = withTracker(FilesTracker)(Files);

FsoFilesContainer.propTypes = {
  repoName: PropTypes.string.isRequired,
  files: PropTypes.object.isRequired,
  treeErrors: PropTypes.object.isRequired,
  subscribeTree: PropTypes.func.isRequired,
};

export {
  FsoFilesContainer,
};
