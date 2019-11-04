/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import path from 'path';
import { Mongo } from 'meteor/mongo';
import { Random } from 'meteor/random';

import {
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
} from 'meteor/nog-fso-grpc';
import {
  createFsoTokenProvider,
} from 'meteor/nog-jwt-2';
import {
  createFsoMiniRegistryModuleServer,
} from 'meteor/nog-fso-mini-registry';

const AA_FSO_READ_REGISTRY = 'fso/read-registry';

const appdir = process.env.PWD;

function pemPath(p) {
  return path.join(appdir, '_private/fso-testapp-devcert', p);
}

function nowRandomId() {
  const now = new Date();
  const nowUnix = Math.floor(now.getTime() / 1000);
  return `${nowUnix}-${Random.id(6)}`;
}

// Nogfsoregd must be running, i.e. `dc up nogfsoregd`.
function describeNogFsoMiniRegistryTests() {
  describe('nog-fso-mini-registry', function () {
    const addr = 'localhost:7550';
    const certFile = pemPath('combined.pem');
    const caFile = pemPath('ca.pem');
    const jwtCertFile = pemPath('jwt-iss.combined.pem');
    const issuer = 'nogapp';

    const testId = nowRandomId();
    const namespace = { coll: `test-${testId}` };

    const registryExreg = 'exreg';
    const fakeUsers = new Mongo.Collection(null);

    const rpcTokenProvider = createFsoTokenProvider({
      issuer,
      cert: jwtCertFile,
      domains: [],
      users: fakeUsers,
    });

    const sysCallCreds = createAuthorizationCallCreds(
      rpcTokenProvider.fsoSysToken,
      { username: 'fso-testapp' },
      {
        subuser: 'minifso',
        scopes: [
          { action: AA_FSO_READ_REGISTRY, names: [registryExreg] },
        ],
      },
    );

    const conn = connectFsoRegGrpc({ addr, certFile, caFile });

    const NogFsoMiniRegistry = createFsoMiniRegistryModuleServer({
      namespace,
      registryConns: [{ registry: registryExreg, conn }],
      rpcSysCallCreds: sysCallCreds,
      logger: { log() {} },
    });

    it('observeRegistry() adds repos.', async function () {
      // The collections start empty.
      expect(NogFsoMiniRegistry.registries.find().count()).to.equal(0);
      expect(NogFsoMiniRegistry.repos.find().count()).to.equal(0);

      // Inspect the registry using direct gRPCs to determine the expected
      // information.
      const regd = conn.registryClient(sysCallCreds);
      const reg = regd.infoSync({ registry: registryExreg });
      const numRepos = Number(reg.numRepos);
      const repos = regd.getReposSync({ registry: registryExreg });
      const repoPaths = new Set(repos.repos.map(repo => repo.globalPath));

      // Then start observe and wait until gRPC stream blocks.
      let obs = null;
      await new Promise((resolve, reject) => {
        try {
          obs = NogFsoMiniRegistry.observeRegistry(registryExreg, {
            testingOnWillBlock: resolve,
          });
        } catch (err) {
          reject(err);
        }
      });
      obs.stop();

      // The collections should now contain the expected registry information.
      expect(NogFsoMiniRegistry.registries.find().count()).to.equal(1);
      expect(NogFsoMiniRegistry.repos.find().count()).to.equal(numRepos);
      expect(repoPaths).to.include.all.keys(
        NogFsoMiniRegistry.repos.find().map(repo => repo.path()),
      );
    });
  });
}

export {
  describeNogFsoMiniRegistryTests,
};
