import { check, Match } from 'meteor/check';

import { createCollectionsServer } from './collections-server.js';
import { createRegistryObserverManager } from './observe.js';
import {
  KeyFsoId,
  KeyPath,
} from './collections.js';

function createFsoMiniRegistryModuleServer({
  namespace, registryConns, rpcSysCallCreds,
  logger = console,
}) {
  check(namespace, { coll: String });
  check(registryConns, [{ registry: String, conn: Object }]);
  check(
    rpcSysCallCreds, Match.Any, // type grpc.credentials.CallCredentials
  );
  check(logger.log, Function);

  function log(msg, ...args) {
    logger.log(`[nog-fso-mini-registry] ${msg}`, ...args);
  }

  const { registries, repos } = createCollectionsServer({ namespace });

  const repoResolver = {
    findRepoFsoId(fsoId) {
      const sel = { [KeyFsoId]: fsoId };
      const fields = { [KeyPath]: true };
      return repos.findOne(sel, { fields });
    },
  };

  const registryObserverManager = createRegistryObserverManager({
    log, registryConns, registries, repos, rpcSysCallCreds,
  });

  const module = {
    registries,
    repos,
    repoResolver,
    observeRegistry: registryObserverManager.observeRegistry,
  };
  return module;
}

export {
  createFsoMiniRegistryModuleServer,
};
