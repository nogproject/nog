/* eslint-env mocha */
/* eslint-disable func-names */
import { expect } from 'chai';

import path from 'path';
import { Mongo } from 'meteor/mongo';

import {
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
} from 'meteor/nog-fso-grpc';
import {
  createFsoTokenProvider,
} from 'meteor/nog-jwt-2';

const AA_FSO_READ_REGISTRY = 'fso/read-registry';

const appdir = process.env.PWD;

function pemPath(p) {
  return path.join(appdir, '_private/fso-testapp-devcert', p);
}

// Nogfsoregd must be running, i.e. `dc up nogfsoregd`.
function describeNogFsoGrpcTests() {
  describe('nog-fso-grpc', function () {
    describe('connectFsoRegGrpc()', function () {
      const addr = 'localhost:7550';
      const certFile = pemPath('combined.pem');
      const caFile = pemPath('ca.pem');

      const registryExreg = 'exreg';

      describe(`without JWT, to dev nogfsoregd ${addr}`, function () {
        // Connect without JWT and inspect the error to test that the TLS
        // connection works.
        it(`TLS connection ok, but request fails.`, function () {
          const conn = connectFsoRegGrpc({ addr, certFile, caFile });
          const regd = conn.registryClient(null);
          function fn() {
            return regd.infoSync({ registry: registryExreg });
          }
          expect(fn).to.throw('UNAUTHENTICATED');
          expect(fn).to.throw('missing GRPC authorization metadata');
        });
      });

      describe(`with JWT, to dev nogfsoregd ${addr}`, function () {
        const jwtCertFile = pemPath('jwt-iss.combined.pem');
        const issuer = 'nogapp';
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

        it(`info() ok.`, function () {
          const conn = connectFsoRegGrpc({ addr, certFile, caFile });
          const regd = conn.registryClient(sysCallCreds);
          const reg = regd.infoSync({ registry: registryExreg });
          expect(reg).to.have.property('registry').equal(registryExreg);
          expect(reg).to.have.property('vid').that.is.instanceof(Buffer);
          expect(reg).to.have.property('numRoots').that.is.a('string');
          expect(reg).to.have.property('numRepos').that.is.a('string');
        });
      });
    });
  });
}

export {
  describeNogFsoGrpcTests,
};
