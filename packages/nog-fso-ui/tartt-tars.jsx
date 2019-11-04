/* eslint-disable react/forbid-prop-types */

import React from 'react';
import PropTypes from 'prop-types';
import { withTracker } from 'meteor/react-meteor-data';
import {
  KeyRepoId,
  KeyTime,
} from 'meteor/nog-fso';

function Trackee({
  isReady, head, tars,
}) {
  if (!isReady) {
    return <div>Loading...</div>;
  }
  if (!head) {
    return <div>No archives.</div>;
  }

  const items = tars.map((t) => {
    const icon = t.tarType() === 'Full' ? 'archive' : 'file-archive-o';
    return (
      <tr key={t.id()}>
        <td><i className={`fa fa-${icon}`} /></td>
        <td>{t.tarType()}</td>
        <td>{t.time().toUTCString()}</td>
      </tr>
    );
  });

  return (
    <div className="row">
      <div className="col-md-12">
        <div className="panel panel-default">
          <div className="panel-heading">
            <h3 className="panel-title">Archives</h3>
          </div>
          <div className="panel-body">
            <p>
              <small>
                Last archive update
                {' '}recorded {head.authorDate().toUTCString()}
                {' '}by {head.author()}
              </small>
            </p>
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

Trackee.propTypes = {
  isReady: PropTypes.bool.isRequired,
  head: PropTypes.object,
  tars: PropTypes.arrayOf(PropTypes.object).isRequired,
};

Trackee.defaultProps = {
  head: null,
};

function Tracker({
  repoId, repoPath,
  tarttHeads, repoTars, subscribeTartt,
}) {
  const sub = subscribeTartt({ path: repoPath });
  const isReady = sub.ready();
  return {
    isReady,
    head: tarttHeads.findOne(repoId),
    tars: repoTars.find(
      { [KeyRepoId]: repoId },
      { sort: { [KeyTime]: -1 } },
    ).fetch(),
  };
}

const TarttTars = withTracker(Tracker)(Trackee);

TarttTars.propTypes = {
  repoId: PropTypes.string.isRequired,
  repoPath: PropTypes.string.isRequired,
  tarttHeads: PropTypes.object.isRequired,
  repoTars: PropTypes.object.isRequired,
  subscribeTartt: PropTypes.func.isRequired,
};

export {
  TarttTars,
};
