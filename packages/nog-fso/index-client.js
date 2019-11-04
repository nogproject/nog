import { check, Match } from 'meteor/check';
import {
  KeyId,
  KeyName,
} from './collections.js';
import {
  AA_FSO_HOME,
  KeyRoute,
} from './fso-home.js';
import { createFsoHomeModuleClient } from './fso-home-client.js';
import {
  AA_FSO_DISCOVER,
  AA_FSO_DISCOVER_ROOT,
  AA_FSO_ENABLE_DISCOVERY_PATH,
  KeyGlobalRootPath,
  KeyUntrackedGlobalPath,
} from './fso-discovery.js';
import {
  createCollectionsClient,
} from './collections-client.js';
import { createSubscribeFuncs } from './fso-sub.js';
import { defMethodCalls } from './fso-methods.js';
import { createFsoDiscoverModuleClient } from './fso-discovery-client.js';
import {
  KeyRepoName,
  KeyTreePath,
} from './fso-tree.js';
import { createFsoTreeModuleClient } from './fso-tree-client.js';
import {
  AA_FSO_LIST_REPOS,
  KeyPath,
} from './fso-list.js';
import { createFsoListModuleClient } from './fso-list-client.js';
import { createTarttModuleClient } from './tartt-client.js';
import {
  KeyRepoId,
  KeyTime,
} from './tartt.js';

function createFsoModuleClient({ namespace, testAccess, subscriber }) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(testAccess, Function);
  check(subscriber, Match.ObjectIncluding({ subscribe: Function }));

  const { repos } = createCollectionsClient({ namespace });

  const {
    roots, subscribeRoots,
    untracked, subscribeUntracked,
    discoveryErrors,
  } = createFsoDiscoverModuleClient({
    namespace, testAccess, subscriber,
  });

  const {
    files, subscribeTree,
    treeErrors,
    content, subscribeTreePathContent,
  } = createFsoTreeModuleClient({
    namespace, testAccess, subscriber,
  });

  const {
    listingNodes, subscribeListing,
    listingErrors,
  } = createFsoListModuleClient({
    namespace, testAccess, subscriber,
  });

  const {
    homeLinks, subscribeHome,
  } = createFsoHomeModuleClient({
    namespace, testAccess, subscriber,
  });

  const {
    tarttHeads, repoTars, subscribeTartt,
  } = createTarttModuleClient({
    namespace, testAccess, subscriber,
  });

  const module = {
    testAccess,
    repos,
    ...createSubscribeFuncs({ namespace, subscriber }),
    ...defMethodCalls(null, { namespace }),
    // Home
    homeLinks, subscribeHome,
    // Discovery
    roots, subscribeRoots,
    untracked, subscribeUntracked,
    discoveryErrors,
    // Tree
    files, subscribeTree,
    treeErrors,
    content, subscribeTreePathContent,
    // Listing
    listingNodes, subscribeListing,
    listingErrors,
    // Tartt
    tarttHeads, repoTars, subscribeTartt,
  };
  return module;
}

export {
  AA_FSO_DISCOVER,
  AA_FSO_DISCOVER_ROOT,
  AA_FSO_ENABLE_DISCOVERY_PATH,
  AA_FSO_HOME,
  AA_FSO_LIST_REPOS,
  KeyGlobalRootPath,
  KeyId,
  KeyName,
  KeyPath,
  KeyRepoId,
  KeyRepoName,
  KeyRoute,
  KeyTime,
  KeyTreePath,
  KeyUntrackedGlobalPath,
  createFsoModuleClient,
};
