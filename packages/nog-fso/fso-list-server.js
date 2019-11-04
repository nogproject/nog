import crypto from 'crypto';
import { check, Match } from 'meteor/check';
import { Random } from 'meteor/random';
import {
  AA_FSO_LIST_REPOS,
  AA_FSO_LIST_REPOS_RECURSIVE,
  CollNameListingErrors,
  CollNameListingNodes,
  KeyListingErrorMessage,
  KeyListingErrorSeverity,
  KeyPath,
  PubNameListing,
  Severity,
} from './fso-list.js';
import {
  KeyName,
  makeCollName,
} from './collections.js';
import { makePubName } from './fso-pubsub.js';

function nameHashId(name) {
  const s = crypto.createHash('sha1').update(name, 'utf8').digest('base64');
  // Shorten and replace confusing characters.
  return s.substr(0, 20).replace(/[=+/]/g, 'x');
}

// From
// <https://github.com/sindresorhus/escape-string-regexp/blob/master/index.js>
const reOperatorsRgx = /[|\\{}()[\]^$+*?.]/g;

function escapeRegExp(s) {
  return s.replace(reOperatorsRgx, '\\$&');
}

// The level listing is computed on the fly from the full list of repo paths
// that match the path prefix.  This works with the repo list alone, without
// additional state.  The Mongo query is simple and can be loaded from an
// index.  It should be fast even for a large number of repos.  But it
// obviously does not scale to huge numbers of repos.  If we observe practical
// performance issues, we could switch to a stateful approach that maintains
// some kind of directory nodes, similar to a filesystem.
//
// Directories are indicated by node paths that end with a slash.  Repo paths
// do not have a trailing slash.  See `FsoListNodeC` in `./fso-list-client.js`.
function lsReposOneLevel({ repos, path }) {
  const nodes = new Set();
  const pathNoSlashPat = (path === '/') ? '' : escapeRegExp(path);
  const sel = {
    // Match repo with the exact path and repos below.
    [KeyName]: { $regex: `^${pathNoSlashPat}($|/)` },
  };
  const fields = {
    [KeyName]: true,
  };
  // Send nodes for the exact path and repos and directories one level below.
  // Example:
  //
  // ```
  // foo
  // foo/bar/
  // foo/bar
  // foo/baz
  // ```
  //
  // But not `foo/`.
  const levelPathRgx = new RegExp(`^${pathNoSlashPat}($|/[^/]+/?)`);
  repos.find(sel, { fields }).forEach((repo) => {
    const m = repo.path().match(levelPathRgx);
    const levelPath = m[0];
    nodes.add(levelPath);
  });
  return nodes;
}

// List uses client-only collections, see `fso-list-client.js`.
function createCollections() {
  return {};
}

function isValidPath(path) {
  return path === '/' || (path.startsWith('/') && !path.endsWith('/'));
}

function publishListingFunc({
  namespace, testAccess, repos,
}) {
  const listingNodesCollName = makeCollName(namespace, CollNameListingNodes);
  const errCollName = makeCollName(namespace, CollNameListingErrors);

  return function publishListing(opts) {
    check(opts, {
      path: String,
      recursive: Boolean,
      nonce: Match.Optional(String),
    });

    // Silently ignore invalid paths.  A correct client should not send them.
    const { path } = opts;
    if (!isValidPath(path)) {
      this.ready();
      return null;
    }

    const addWarning = (msg) => {
      const id = Random.id();
      this.added(errCollName, id, {
        [KeyListingErrorMessage]: msg,
        [KeyListingErrorSeverity]: Severity.Warning,
      });
    };

    const publishOneLevel = () => {
      for (const nodePath of lsReposOneLevel({ repos, path })) {
        const id = nameHashId(nodePath);
        this.added(listingNodesCollName, id, {
          [KeyPath]: nodePath,
        });
      }
      this.ready();
      return null;
    };

    const publishRecursive = () => {
      const pathNoSlashPat = (path === '/') ? '' : escapeRegExp(path);
      const sel = {
        // Match a repo with the exact path and repos below.
        [KeyName]: { $regex: `^${pathNoSlashPat}($|/)` },
      };
      const fields = {
        [KeyName]: true,
      };
      const limit = 100;
      let n = 0;
      repos.find(sel, { fields, limit }).forEach((repo) => {
        const id = nameHashId(repo.path());
        this.added(listingNodesCollName, id, {
          [KeyPath]: repo.path(),
        });
        n += 1;
      });
      if (n >= limit) {
        addWarning(
          `Partial listing. Displaying only ${n} repos. ` +
          'Consider disabling recursive listing.',
        );
      }
      this.ready();
      return null;
    };

    // If the user does not have the basic permission, silently ignore
    // the subscription.
    const euid = this.userId;
    if (!testAccess(euid, AA_FSO_LIST_REPOS, { path })) {
      this.ready();
      return null;
    }

    // If the user wants to list recursively, but does not have the permission
    // to do so, send a warning and a shallow listing instead.
    let publish;
    if (opts.recursive) {
      if (testAccess(euid, AA_FSO_LIST_REPOS_RECURSIVE, { path })) {
        publish = publishRecursive;
      } else {
        addWarning(
          'You are not allowed to list recursively. ' +
          'Displaying single level instead.',
        );
        publish = publishOneLevel;
      }
    } else {
      publish = publishOneLevel;
    }

    return publish();
  };
}

function registerPublications({
  publisher, namespace, testAccess, repos,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }
  defPub(PubNameListing, publishListingFunc({
    namespace, testAccess, repos,
  }));
}

function createFsoListModuleServer({
  namespace, checkAccess, testAccess, publisher, repos,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(checkAccess, Function);
  check(publisher, Match.ObjectIncluding({ publish: Function }));

  registerPublications({
    publisher, namespace, testAccess, repos,
  });

  const module = {
    ...createCollections({ namespace }),
  };
  return module;
}

export {
  createFsoListModuleServer,
};
