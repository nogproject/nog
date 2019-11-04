import { createCluster } from './cluster.js';
import { clusterOptSingleInstanceModeSetting } from './cluster-settings.js';

function createClusterModuleServer({
  namespace, optSingleInstanceMode, optGlobalReadOnly,
}) {
  return createCluster({
    namespace, optSingleInstanceMode, optGlobalReadOnly,
  });
}

export {
  clusterOptSingleInstanceModeSetting,
  createClusterModuleServer,
};
