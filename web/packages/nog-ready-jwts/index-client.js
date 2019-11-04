import { check } from 'meteor/check';

import { createCollectionsClient } from './collections-client.js';
import { defSubscribes } from './sub.js';
import { defMethodCalls } from './methods.js';

function createReadyJwtsModuleClient({
  namespace, subscriber,
}) {
  check(namespace, { coll: String, pub: String, meth: String });
  check(subscriber.subscribe, Function);

  const { readyJwts } = createCollectionsClient({ namespace });

  const module = {
    readyJwts,
    ...defSubscribes({ namespace, subscriber }),
    ...defMethodCalls(null, { namespace }),
  };
  return module;
}

export {
  createReadyJwtsModuleClient,
};
