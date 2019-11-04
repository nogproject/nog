import { _ } from 'meteor/underscore';
import React from 'react';
import PropTypes from 'prop-types';

// `splitAbspath()` splits an absolute path.  The path must start with slash
// and end without slash.
function splitAbspath(p) {
  const parts = p.split('/');
  return parts.slice(1);
}

function PrefixBreadcrumbs({
  path, makeHrefLsPath,
}) {
  if (path === '/') {
    return '/';
  }

  const htmlParts = [
    (
      <a key="/" href={makeHrefLsPath('/')}>
        /
      </a>
    ),
  ];

  const parts = splitAbspath(path);
  const prefix = [];
  for (const name of _.initial(parts)) {
    prefix.push(name);
    const prefixPath = `/${prefix.join('/')}/`;
    const href = makeHrefLsPath(prefixPath);
    htmlParts.push(
      (
        <span key={prefixPath}>
          {' '}<a href={href}>{name}</a>{' /'}
        </span>
      ),
    );
  }
  htmlParts.push(
    (
      <span key={path}>
        {' '}{_.last(parts)}{' /'}
      </span>
    ),
  );

  return htmlParts;
}

PrefixBreadcrumbs.propTypes = {
  path: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
};

function RepoBreadcrumbs({
  path, makeHrefLsPath,
}) {
  const htmlParts = [
    (
      <a key="/" href={makeHrefLsPath('/')}>
        /
      </a>
    ),
  ];

  const parts = splitAbspath(path);
  const dirs = _.initial(parts);
  const basename = _.last(parts);
  const prefix = [];
  for (const name of dirs) {
    prefix.push(name);
    const prefixPath = `/${prefix.join('/')}/`;
    const href = makeHrefLsPath(prefixPath);
    htmlParts.push(
      (
        <span key={prefixPath}>
          {' '}<a href={href}>{name}</a>{' /'}
        </span>
      ),
    );
  }
  htmlParts.push(
    (
      <span key={path}>
        {' '}{basename}
      </span>
    ),
  );

  return htmlParts;
}

RepoBreadcrumbs.propTypes = {
  path: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
};

function CondensedRepoBreadcrumbs({
  path, makeHrefLsPath, makeHrefRepo,
}) {
  const parts = splitAbspath(path);
  const dirs = _.initial(parts);
  const basename = _.last(parts);
  const prefix = [];
  const htmlParts = [];
  for (const name of dirs) {
    prefix.push(name);
    const prefixPath = `/${prefix.join('/')}/`;
    const href = makeHrefLsPath(prefixPath);
    htmlParts.push(
      (
        <span key={prefixPath}>
          {'/'}<a href={href}>{name}</a>
        </span>
      ),
    );
  }
  const href = makeHrefRepo(path);
  htmlParts.push(
    (
      <span key={path}>
        {'/ '}<a href={href}><strong>{basename}</strong></a>
      </span>
    ),
  );

  return htmlParts;
}

CondensedRepoBreadcrumbs.propTypes = {
  path: PropTypes.string.isRequired,
  makeHrefLsPath: PropTypes.func.isRequired,
  makeHrefRepo: PropTypes.func.isRequired,
};

export {
  CondensedRepoBreadcrumbs,
  PrefixBreadcrumbs,
  RepoBreadcrumbs,
};
