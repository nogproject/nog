// Check peer versions before importing anything else to report version
// problems before they are reported as import errors.
import './package-peer-versions.js';

import { createComputer } from './common.js';

// `createExampleModuleServer()` illustrates how to create a module instance
// with dependency injection.
function createExampleModuleServer({
  serverName,
}) {
  let name = 'server';
  if (serverName) {
    name += ` ${serverName}`;
  }

  const comp = createComputer({ name });
  return {
    sum: comp.sum,
    serverSum: comp.nameSum,
  };
}

// `NogExample` is the default module instance that is used for exporting
// functions `sum` and `serverSum` without requiring dependency injection.
const NogExample = createExampleModuleServer({ serverName: null });
const { sum, serverSum } = NogExample;

export {
  createExampleModuleServer,
  sum,
  serverSum,
};
