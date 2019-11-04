import { createComputer } from './common.js';

// See `./index-server.js` for comments.
function createExampleModuleClient({
  clientName,
}) {
  let name = 'client';
  if (clientName) {
    name += ` ${clientName}`;
  }

  const comp = createComputer({ name });
  return {
    sum: comp.sum,
    clientSum: comp.nameSum,
  };
}

const NogExample = createExampleModuleClient({ clientName: null });
const { sum, clientSum } = NogExample;

export {
  createExampleModuleClient,
  sum,
  clientSum,
};
