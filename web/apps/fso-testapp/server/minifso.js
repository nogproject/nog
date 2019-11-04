import { Meteor } from 'meteor/meteor';
import { WebApp } from 'meteor/webapp';
import {
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
} from 'meteor/nog-fso-grpc';
import {
  createAuthApiActions,
  createBearerJwtAuthn,
  createFsoTokenProvider,
} from 'meteor/nog-jwt-2';
import { createRestServer } from 'meteor/nog-rest-2';
import {
  createFsoMiniRegistryModuleServer,
} from 'meteor/nog-fso-mini-registry';
import { compileFsoPermissions } from 'meteor/nog-fso-authz';
import { createReadyJwtsModuleServer } from 'meteor/nog-ready-jwts';

import { getSettingsMinifso } from './minifso-settings.js';

const AA_FSO_READ_REGISTRY = 'fso/read-registry';

function initMinifso({
  NsFsoMiniRegistry, NsReadyJwts,
  testingUsers, NogAccess, NogCluster,
}) {
  const settings = getSettingsMinifso();

  if (settings.permissions) {
    try {
      const statements = compileFsoPermissions(settings.permissions);
      NogAccess.addStatements(statements);
      console.log(
        `[fso-testapp] Installed ${statements.length} access statements.`,
      );
    } catch (err) {
      console.error(
        '[fso-testapp] Failed to install access statements.',
        'err', err,
      );
    }
  }

  // Connections by `regd.name`.
  const conns = new Map();
  // Create connections during startup to confirm that the settings are sane.
  // But start observing only after acquiring a cluster lease.
  for (const regd of settings.registries) {
    conns.set(regd.name, connectFsoRegGrpc({
      addr: regd.addr, certFile: regd.cert, caFile: regd.ca,
    }));
  }

  // `registryConns()` returns an array of registry names and corresponding
  // GRPC connections `{ registry, conn }`.
  function registryConns() {
    const rcs = [];
    for (const regd of settings.registries) {
      const conn = conns.get(regd.name);
      for (const registry of regd.registries) {
        rcs.push({ registry, conn });
      }
    }
    return rcs;
  }

  const rpcTokenProvider = createFsoTokenProvider({
    issuer: settings.jwt.issuer,
    cert: settings.jwt.cert,
    domains: settings.jwt.domains || [],
    users: Meteor.users,
  });

  function allRegistryNames() {
    let names = [];
    for (const regd of settings.registries) {
      names = names.concat(regd.registries);
    }
    return names;
  }

  const sysCallCreds = createAuthorizationCallCreds(
    rpcTokenProvider.fsoSysToken,
    { username: 'fso-testapp' },
    {
      subuser: 'minifso',
      scopes: [
        { action: AA_FSO_READ_REGISTRY, names: allRegistryNames() },
      ],
    },
  );

  const NogFsoMiniRegistry = createFsoMiniRegistryModuleServer({
    namespace: NsFsoMiniRegistry,
    registryConns: registryConns(),
    rpcSysCallCreds: sysCallCreds,
  });

  // List of observers by `part.begin`.
  const observers = new Map();

  function nameIsInPart(name, part) {
    const { sel } = part;
    if (sel.$lt && !(name < sel.$lt)) {
      return false;
    }
    return name >= sel.$gte;
  }

  function startPart(part) {
    const regds = settings.registries.filter(s => nameIsInPart(s.name, part));

    const obs = [];
    for (const regd of regds) {
      for (const reg of regd.registries) {
        obs.push({
          name: `${regd.name}.${reg}`,
          observer: NogFsoMiniRegistry.observeRegistry(reg),
        });
      }
    }

    observers.set(part.begin, obs);

    const names = obs.map(o => o.name);
    console.log(`[fso-testapp] Started watching fso ${part.selHuman}:`, names);

    // Useful during testing:
    // Meteor.setTimeout(() => { stopPart(part); }, 1000);
  }

  function stopPart(part) {
    const obs = observers.get(part.begin);
    observers.delete(part.begin);

    for (const o of obs) {
      o.observer.stop();
    }

    const names = obs.map(o => o.name);
    console.log(`[fso-testapp] Stopped watching fso ${part.selHuman}:`, names);
  }

  const partition = new NogCluster.IdPartition({ name: 'minifso', max: 2 });
  partition.onacquire = (part) => {
    startPart(part);
  };
  partition.onrelease = (part) => {
    stopPart(part);
  };
  NogCluster.registerHeartbeat(partition);

  const NogReadyJwts = createReadyJwtsModuleServer({
    namespace: NsReadyJwts,
    publisher: Meteor,
    findOneUserById: id => Meteor.users.findOne(id),
    checkAccess: NogAccess.checkAccess,
    testAccess: NogAccess.testAccess,
    tokenProvider: rpcTokenProvider,
  });

  settings.readyJwts.forEach((spec) => {
    NogReadyJwts.upsertReadyJwt(spec);
  });

  const tokenAlice = NogReadyJwts.issueTokenSudo(testingUsers.alice, {
    path: '/sys/jwts/example/admin/fsoadmin-x-ex904-dev-exuniverse',
  });
  console.log(
    '[fso-testapp] Alice aka alovelace wildcard API bearer token:',
    '# should ERR_ACCESS_DEFAULT_DENY',
    `
curl \
-X POST \
-H 'Authorization: Bearer ${tokenAlice.token}' \
-H 'Content-Type: application/json' \
-d '{ \
"expiresIn": 5443200, \
"subuser": "test", \
"aud": ["fso"], \
"scopes": [{ "actions": ["fso/read-registry"], "names": ["exreg"] }] \
}' \
'http://localhost:3000/api/v1/fso/sysauth'
`);

  const tokenBob = NogReadyJwts.issueToken(testingUsers.bob, {
    path: '/sys/jwts/example/admin/fsoadmin-x-ex904-dev-exuniverse',
  });
  console.log(
    '[fso-testapp] Bob aka sprohaska wildcard API bearer token:',
    '# should issue token',
    `
curl \
-X POST \
-H 'Authorization: Bearer ${tokenBob.token}' \
-H 'Content-Type: application/json' \
-d '{ \
"expiresIn": 5443200, \
"subuser": "test", \
"aud": ["fso"], \
"scopes": [{ "actions": ["fso/read-registry"], "names": ["exreg"] }] \
}' \
'http://localhost:3000/api/v1/fso/sysauth'
`);

  const api = createRestServer({});
  const authn = createBearerJwtAuthn({
    users: Meteor.users,
    issuer: settings.jwt.issuer,
    audience: settings.jwt.issuer,
    ou: settings.jwt.ou,
    ca: settings.jwt.ca,
    testingJtis: settings.jwt.testingJtis || [],
  });
  api.auth.use(authn.middleware);
  api.useActions(createAuthApiActions({
    checkAccess: NogAccess.checkAccess,
    testAccess: NogAccess.testAccess,
    repoResolver: NogFsoMiniRegistry.repoResolver,
    rpcTokenProvider,
  }));
  WebApp.connectHandlers.use('/api/v1/fso', api.app);
  WebApp.connectHandlers.use('/api/fso', api.app);

  return {
    NogFsoMiniRegistry,
    NogReadyJwts,
  };
}

export {
  initMinifso,
};
