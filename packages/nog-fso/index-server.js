// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';

import { check, Match } from 'meteor/check';

import {
  connectFsoGitNogGrpc,
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
} from './grpc.js';
import { observeRegistry } from './observe.js';
import {
  KeyFsoId,
  KeyId,
  KeyName,
  KeyRegistryId,
} from './collections.js';
import { createCollectionsServer } from './collections-server.js';
import { createPublications } from './fso-pub.js';
import { createMethodsServer } from './fso-methods-server.js';
import { defMethodCalls } from './fso-methods.js';
import { createBroadcast } from './broadcast.js';
import { createFsoDiscoverModuleServer } from './fso-discovery-server.js';
import { createFsoTreeModuleServer } from './fso-tree-server.js';
import { createFsoListModuleServer } from './fso-list-server.js';
import { createFsoIoModuleServer } from './fso-io-server.js';
import { createTarttModuleServer } from './tartt-server.js';
import {
  compileFsoPermissions,
  matchFsoPermissions,
} from './access-fso.js';
import {
  createFsoHomeModuleServer,
  matchFsoHomes,
} from './fso-home-server.js';
import { createFsoTokenProvider } from './fso-jwt.js';
import { createAuthApiActions } from './fso-jwt-auth.js';

function createFsoModuleServer({
  namespace, checkAccess, testAccess, publisher,
  registryConns, gitNogConns, gitlabs, homes,
  rpcTokenProvider, rpcSysCallCreds,
  catalogUpdater,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(checkAccess, Function);
  check(publisher, Match.ObjectIncluding({ publish: Function }));
  check(registryConns, [{ registry: String, conn: Object }]);
  check(gitNogConns, [{ registry: String, conn: Object }]);
  check(homes, matchFsoHomes);
  check(rpcTokenProvider, Match.ObjectIncluding({ fsoToken: Function }));
  check(
    rpcSysCallCreds, Match.Any, // type grpc.credentials.CallCredentials
  );
  check(catalogUpdater, Match.Optional(
    Match.ObjectIncluding({ update: Function })),
  );

  const rpcAuthorization = rpcTokenProvider.fsoTokenFunc();

  const { registries, repos } = createCollectionsServer({ namespace });

  const broadcast = createBroadcast({
    registryConns,
    sysCallCreds: rpcSysCallCreds,
  });

  const {
    enableDiscoveryPath,
  } = createFsoDiscoverModuleServer({
    namespace, checkAccess, testAccess, publisher, registryConns,
    rpcAuthorization,
  });

  createFsoTreeModuleServer({
    namespace, checkAccess, testAccess, publisher, registryConns, broadcast,
    registries, repos, rpcAuthorization,
  });

  createFsoListModuleServer({
    namespace, checkAccess, testAccess, publisher, repos,
  });

  createFsoHomeModuleServer({
    namespace, checkAccess, testAccess, publisher, homes,
  });

  const {
    openRepo,
  } = createFsoIoModuleServer({
    registries, repos, checkAccess, registryConns, rpcAuthorization,
  });

  createPublications({
    namespace, testAccess, publisher, registries, repos, openRepo,
    registryConns, gitNogConns, gitlabs, broadcast, rpcAuthorization,
  });

  const {
    tarttHeads, repoTars,
  } = createTarttModuleServer({
    namespace, testAccess, checkAccess, publisher, openRepo,
  });

  const module = {
    registries, repos, tarttHeads, repoTars, broadcast,
    openRepo,
    enableDiscoveryPath,
    ...createMethodsServer({
      checkAccess, registries, repos, registryConns, gitNogConns, gitlabs,
      rpcAuthorization, rpcTokenProvider,
      catalogUpdater,
    }),
    apiActions: createAuthApiActions({
      checkAccess, testAccess, repos, rpcTokenProvider,
    }),
  };
  // Register Meteor methods without assigning `callX()` functions to `module`.
  // Server code should call the real functions, not via a Meteor method.
  defMethodCalls(module, { namespace });
  return module;
}

export {
  KeyId,
  KeyFsoId,
  KeyName,
  KeyRegistryId,
  compileFsoPermissions,
  connectFsoGitNogGrpc,
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
  createFsoModuleServer,
  createFsoTokenProvider,
  matchFsoHomes,
  matchFsoPermissions,
  observeRegistry,
};
