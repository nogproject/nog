import fs from 'fs';
import { Meteor } from 'meteor/meteor';
import grpc from 'grpc';

import * as _ from './underscore.js';

import {
  Registry,
  Repos,
  Stat,
  GitNog,
  GitNogTree,
  Broadcast,
  Discovery,
  Tartt,
  UnixDomains,
} from './proto.js';

function connectFsoRegGrpc({
  addr, certFile, caFile,
}) {
  const caPem = fs.readFileSync(caFile);
  const certPem = fs.readFileSync(certFile);
  // The same PEM can be passed as key and cert if it contains a key PEM block
  // and a cert PEM block.
  const tlsCreds = grpc.credentials.createSsl(caPem, certPem, certPem);

  // See explanation in <https://github.com/grpc/grpc-node/issues/154> and
  // available options in `grpc_types.h` <http://bit.ly/2GXOTFo>.
  const clientOpts = {
    'grpc.keepalive_time_ms': 2 * 60 * 1000,
  };

  function combinedCreds(callCreds) {
    if (!callCreds) {
      return tlsCreds;
    }
    return grpc.credentials.combineChannelCredentials(
      tlsCreds, callCreds,
    );
  }

  const conn = {
    addr,

    registryClient(callCreds) {
      const c = new Registry(
        addr, combinedCreds(callCreds), clientOpts,
      );
      // Wrap functions as needed.
      c.infoSync = Meteor.wrapAsync(c.info, c);
      c.initRepoSync = Meteor.wrapAsync(c.initRepo, c);
      c.getRootsSync = Meteor.wrapAsync(c.getRoots, c);
      c.getReposSync = Meteor.wrapAsync(c.getRepos, c);
      c.enableDiscoveryPaths = Meteor.wrapAsync(c.enableDiscoveryPaths, c);
      return c;
    },

    reposClient(callCreds) {
      const c = new Repos(
        addr, combinedCreds(callCreds), clientOpts,
      );
      // Wrap functions as needed.
      c.getRepoSync = Meteor.wrapAsync(c.getRepo, c);
      c.postMoveRepoAppAccepted = Meteor.wrapAsync(
        c.postMoveRepoAppAccepted, c,
      );
      return c;
    },

    statClient(callCreds) {
      const c = new Stat(
        addr, combinedCreds(callCreds), clientOpts,
      );
      // Wrap functions as needed.
      c.statSync = Meteor.wrapAsync(c.stat, c);
      c.refreshContentSync = Meteor.wrapAsync(c.refreshContent, c);
      c.reinitSubdirTrackingSync = Meteor.wrapAsync(c.reinitSubdirTracking, c);
      return c;
    },

    gitNogClient(callCreds) {
      const c = new GitNog(
        addr, combinedCreds(callCreds), clientOpts,
      );
      // Wrap functions as needed.
      c.headSync = Meteor.wrapAsync(c.head, c);
      c.summarySync = Meteor.wrapAsync(c.summary, c);
      c.metaSync = Meteor.wrapAsync(c.meta, c);
      c.putMetaSync = Meteor.wrapAsync(c.putMeta, c);
      c.contentSync = Meteor.wrapAsync(c.content, c);
      return c;
    },

    gitNogTreeClient(callCreds) {
      const c = new GitNogTree(
        addr, combinedCreds(callCreds), clientOpts,
      );
      return c;
    },

    broadcastClient(callCreds) {
      const c = new Broadcast(
        addr, combinedCreds(callCreds), clientOpts,
      );
      return c;
    },

    discoveryClient(callCreds) {
      const c = new Discovery(
        addr, combinedCreds(callCreds), clientOpts,
      );
      return c;
    },

    tarttClient(callCreds) {
      const c = new Tartt(
        addr, combinedCreds(callCreds), clientOpts,
      );
      c.tarttHeadSync = Meteor.wrapAsync(c.tarttHead, c);
      c.listTarsSync = Meteor.wrapAsync(c.listTars, c);
      return c;
    },

    unixDomainsClient(callCreds) {
      const c = new UnixDomains(
        addr, combinedCreds(callCreds), clientOpts,
      );
      c.getUnixDomainSync = Meteor.wrapAsync(c.getUnixDomain, c);
      c.getUnixUserSync = Meteor.wrapAsync(c.getUnixUser, c);
      return c;
    },
  };
  return conn;
}

const TimingDefaults = {
  expiresInS: 10 * 60,
  refreshPeriodS: 5 * 60,
  refreshPeriodPct: 50,
};

// `createAuthorizationCallCreds()` returns GRPC `CallCredentials` that manage
// a JWT, issued by `rpcAuthorization(euid, ...)`, in the GRPC metadata
// `authorization`.  The JWT validity duration and refresh period is specified
// in `opts: { expiresInS, refreshPeriodS }`.
function createAuthorizationCallCreds(rpcAuthorization, euid, opts) {
  function parseTiming() {
    if (!opts) {
      return TimingDefaults;
    }

    const { expiresInS } = opts;
    if (!expiresInS) {
      return TimingDefaults;
    }

    let { refreshPeriodS } = opts;
    if (!refreshPeriodS) {
      refreshPeriodS = Math.ceil(
        (TimingDefaults.refreshPeriodPct * expiresInS) / 100,
      );
    }
    return { expiresInS, refreshPeriodS };
  }

  function pickDetails() {
    if (!opts) {
      return {};
    }
    return _.pick(opts, 'subuser', 'scope', 'scopes');
  }

  const { expiresInS, refreshPeriodS } = parseTiming();
  const authDetails = pickDetails();

  let refreshAtMs = 0;
  let token = null;

  // See <https://grpc.io/grpc/node/grpc.credentials.html#~generateMetadata>.
  function gen(params, cb) {
    const now = Date.now();
    if (!token || (now > refreshAtMs)) {
      token = rpcAuthorization(euid, {
        expiresIn: expiresInS,
        ...authDetails,
      });
      refreshAtMs = now + (refreshPeriodS * 1000);
    }
    const md = new grpc.Metadata();
    md.add('authorization', token);
    cb(null, md);
  }

  return grpc.credentials.createFromMetadataGenerator(gen);
}

export {
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
};
