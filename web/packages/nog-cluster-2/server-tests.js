/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import { createClusterModuleServer } from 'meteor/nog-cluster-2';

describe('nog-cluster-2 server', function () {
  const namespace = { coll: 'nogclustertest' };

  it('creates server module', function () {
    const NogCluster = createClusterModuleServer({
      namespace,
    });
    expect(NogCluster).to.have.property('IdPartition');
  });
});
