import { normalize } from 'path';
import { Writable } from 'stream';
import { Meteor } from 'meteor/meteor';
import { check, Match } from 'meteor/check';
import { createAuthorizationCallCreds } from './grpc.js';
import {
  ERR_FSO,
  ERR_FSO_CLIENT,
  nogthrow,
} from './errors.js';
import {
  AA_FSO_DISCOVER,
  AA_FSO_DISCOVER_ROOT,
  AA_FSO_ENABLE_DISCOVERY_PATH,
  CollNameDiscoveryErrors,
  CollNameRoots,
  CollNameUntracked,
  KeyDiscoveryErrorMessage,
  KeyGlobalRootPath,
  KeyRegistryName,
  KeyUntrackedGlobalPath,
  KeyUntrackedStatus,
  PubNameRoots,
  PubNameUntracked,
  UntrackedStatus,
} from './fso-discovery.js';
import { makeCollName } from './collections.js';
import { makePubName } from './fso-pubsub.js';
import { Random } from 'meteor/random';

const AA_FSO_READ_REGISTRY = 'fso/read-registry';
const AA_FSO_FIND = 'fso/find';

function logerr(msg, ...args) {
  console.error(`[fso] ${msg}`, ...args);
}

// Discovery uses client-only collections, see `fso-discovery-client.js`.
function createCollections() {
  return {};
}

function normpath(p) {
  const n = normalize(p);
  if (n.endsWith('/')) {
    return n.substr(0, n.length - 1);
  }
  return n;
}

// `publishRoots` directly polls the registry to send a list of roots to each
// subscription.
//
// Alternative: Change `process.js`.  Broaden its scope to also maintain a list
// of roots, either in the `registries` MongoDB collection or in a separate
// `roots` collection.  Publish from MongoDB.
//
// Alternative: Add a separate registry watcher that maintains a `roots`
// MongoDB collection.  Publish from MongoDB.
//
function publishRootsFunc({
  namespace, testAccess, registryConns, rpcAuthorization,
}) {
  const rootsCollName = makeCollName(namespace, CollNameRoots);
  const errCollName = makeCollName(namespace, CollNameDiscoveryErrors);

  return function publishRoots(opts) {
    check(opts, { prefix: String });
    const prefix = normpath(opts.prefix);

    // `AA_FSO_DISCOVER` allows listing of roots below `prefix` in general.
    // Individual roots also require `AA_FSO_DISCOVER_ROOT`; see check below.
    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    if (!testAccess(euid, AA_FSO_DISCOVER, { path: prefix })) {
      this.ready();
      return null;
    }

    const pubError = ({ err, registry }) => {
      const errId = Random.id();
      const msg = (
        `Failed to get roots for registry \`${registry}\`: ` +
        `${err.message}.`
      );
      this.added(errCollName, errId, {
        [KeyDiscoveryErrorMessage]: msg,
      });
    };

    // Assuming that roots are rarely added, there is little value in polling
    // at regular intervals.  Instead, rely on users to reload if they believe
    // that there should be more roots.
    registryConns.forEach(({ registry, conn }) => {
      const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        scope: { action: AA_FSO_READ_REGISTRY, name: registry },
      });
      const rpc = conn.registryClient(callCreds);
      try {
        const o = rpc.getRootsSync({ registry });
        o.roots.forEach(({ globalRoot }) => {
          // Test access for individual roots.  We will reconsider the approach
          // if we observe performance problems with a larger number of roots.
          //
          // First filter roots based on `prefix`, which is a simple string
          // operation.  Then `testAccess()`.  The paths are compared with a
          // trailing slash so that the prefix works like a directory, only
          // allowing access to paths below.
          //
          // XXX `prefix` would better be passed to `getRootsSync()`, and
          // `nogfsoregd` would send a filtered list.
          const accessPath = normpath(globalRoot);
          if (!`${accessPath}/`.startsWith(`${prefix}/`)) {
            return;
          }
          if (!testAccess(euid, AA_FSO_DISCOVER_ROOT, { path: accessPath })) {
            return;
          }

          const id = globalRoot; // The name is the id.
          this.added(rootsCollName, id, {
            [KeyGlobalRootPath]: globalRoot,
            [KeyRegistryName]: registry,
          });
        });
      } catch (err) {
        logerr('Failed to get roots.', 'err', err, 'registry', registry);
        pubError({ err, registry });
      }
    });

    this.ready();
    return null;
  };
}

// `publishUntracked()` finds untracked repos once.  It should probably poll at
// regular intervals or reactively update the list by some other means.  For
// now, it accepts a nonce to allow clients to force a new subscription, which
// is not ideal in practice, because the new subscription transitions from
// non-ready to ready, which may cause more GUI redrawing than desired.
function publishUntrackedFunc({
  namespace, testAccess, registryConns, rpcAuthorization,
}) {
  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );
  const untrackedCollName = makeCollName(namespace, CollNameUntracked);
  const errCollName = makeCollName(namespace, CollNameDiscoveryErrors);

  return function publishUntracked(opts) {
    check(opts, {
      registry: String,
      globalRoot: String,
      nonce: Match.Maybe(String),
    });
    const { registry, globalRoot } = opts;

    const euid = this.userId ? Meteor.users.findOne(this.userId) : null;
    const accessPath = normpath(opts.globalRoot);
    if (!testAccess(euid, AA_FSO_DISCOVER_ROOT, { path: accessPath })) {
      this.ready();
      return null;
    }

    const conn = connByRegistry.get(registry);
    if (!conn) {
      this.ready();
      return null;
    }

    const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
      scope: { action: AA_FSO_FIND, path: normpath(accessPath) },
    });
    const rpc = conn.discoveryClient(callCreds);

    const addError = (err) => {
      const errId = Random.id();
      const msg = (
        `Failed to find untracked repos for registry \`${registry}\`: ` +
        `${err.message}.`
      );
      this.added(errCollName, errId, {
        [KeyDiscoveryErrorMessage]: msg,
      });
    };

    const addUntracked = (relpath, status) => {
      const path = relpath === '.' ? globalRoot : `${globalRoot}/${relpath}`;
      const id = path;
      this.added(untrackedCollName, id, {
        [KeyUntrackedGlobalPath]: path,
        [KeyRegistryName]: registry,
        [KeyUntrackedStatus]: status,
      });
    };

    const stream = rpc.findUntracked({ registry, globalRoot });

    // Pipe the stream to process response messages in serial.
    const cbStream = new Writable({
      objectMode: true,
      write: Meteor.bindEnvironment((rsp, enc, next) => {
        rsp.candidates.forEach((relpath) => {
          addUntracked(relpath, UntrackedStatus.Candidate);
        });
        rsp.ignored.forEach((relpath) => {
          addUntracked(relpath, UntrackedStatus.Ignored);
        });
        next();
      }),
    });

    stream.on('error', Meteor.bindEnvironment((err) => {
      cbStream.destroy();
      addError(err);
      this.ready();
    }));

    cbStream.on('finish', Meteor.bindEnvironment(() => {
      this.ready();
    }));

    stream.pipe(cbStream);

    return null;
  };
}

function registerPublications({
  publisher, namespace, testAccess, registryConns, rpcAuthorization,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }
  defPub(PubNameRoots, publishRootsFunc({
    namespace, testAccess, registryConns, rpcAuthorization,
  }));
  defPub(PubNameUntracked, publishUntrackedFunc({
    namespace, testAccess, registryConns, rpcAuthorization,
  }));
}

function createMethods({
  checkAccess, registryConns, rpcAuthorization,
}) {
  const connByRegistry = new Map(
    registryConns.map(({ registry, conn }) => [registry, conn]),
  );

  return {
    enableDiscoveryPath(euid, opts) {
      check(opts, {
        registryName: String,
        globalRoot: String,
        depth: Number,
        globalPath: String,
      });
      const {
        registryName, globalRoot,
        depth, globalPath,
      } = opts;

      const accessPath = normpath(globalRoot);
      checkAccess(euid, AA_FSO_ENABLE_DISCOVERY_PATH, { path: accessPath });

      const conn = connByRegistry.get(registryName);
      if (!conn) {
        nogthrow(ERR_FSO, {
          reason: `No registry GRPC for \`${registryName}\`.`,
        });
      }

      const callCreds = createAuthorizationCallCreds(rpcAuthorization, euid, {
        scope: {
          action: AA_FSO_ENABLE_DISCOVERY_PATH,
          path: normpath(accessPath),
        },
      });
      const rpc = conn.registryClient(callCreds);

      try {
        rpc.enableDiscoveryPaths({
          registry: registryName,
          globalRoot,
          depthPaths: [{ depth, path: globalPath }],
        });
      } catch (cause) {
        nogthrow(ERR_FSO_CLIENT, { reason: 'Failed to enable path.', cause });
      }
    },
  };
}

function createFsoDiscoverModuleServer({
  namespace, checkAccess, testAccess, publisher, registryConns,
  rpcAuthorization,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(checkAccess, Function);
  check(publisher, Match.ObjectIncluding({ publish: Function }));
  check(registryConns, [{ registry: String, conn: Object }]);

  registerPublications({
    publisher, namespace, testAccess, registryConns, rpcAuthorization,
  });

  const module = {
    ...createCollections({ namespace }),
    ...createMethods({ checkAccess, registryConns, rpcAuthorization }),
  };
  return module;
}

export {
  createFsoDiscoverModuleServer,
};
