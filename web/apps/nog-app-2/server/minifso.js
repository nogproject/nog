import path from 'path';
import { Meteor } from 'meteor/meteor';
import { Match, check } from 'meteor/check';
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
import {
  compileFsoPermissions,
  matchFsoPermissions,
} from 'meteor/nog-fso-authz';
import {
  createReadyJwtsModuleServer,
  matchReadyJwt,
} from 'meteor/nog-ready-jwts';

const AA_FSO_READ_REGISTRY = 'fso/read-registry';
const AA_READ_UNIX_DOMAIN = 'uxd/read-unix-domain';

const matchDev = Match.Where((x) => {
  check(x, String);
  return x === 'dev';
});

const matchRegistry = Match.Where((x) => {
  check(x, {
    name: String,
    addr: String,
    cert: String,
    ca: String,
    registries: [String],
    domains: Match.Optional([String]),
  });
  return true;
});

const matchJwt = Match.Where((x) => {
  check(x, {
    issuer: String,
    cert: String,
    ca: String,
    ou: String,
    domains: Match.Optional([{ service: String, jwtXcrd: String }]),
  });
  return true;
});

const matchMinifso = Match.Where((x) => {
  check(x, {
    registries: [matchRegistry],
    jwt: Match.Optional(matchJwt),
    permissions: matchFsoPermissions,
    readyJwts: [matchReadyJwt],
  });
  return true;
});

const matchDevMinifso = Match.OneOf(matchDev, matchMinifso);

const minifsoSetting = {
  key: 'minifso',
  val: {
    registries: [],
    permissions: [],
  },
  help: `
\`minifso\` configures the file system observer mini registry.  Use the string
\`dev\` to activate the localhost dev setup.  Production example:

    Meteor.settings.minifso: {
      registries: [
        {
          name: 'example',
          addr: 'nogfsoregd.example.org:7550',
          cert: '/example/ssl/certs/nogappd/combined.pem',
          ca: '/example/ssl/certs/nogappd/ca.pem',
          registries: ['exreg', 'exreg2'],
          domains: ['EXDOM'];
        }
      ],
      jwt: {
        // Signing and Verification.
        issuer: 'nogapp',
        // Signing.
        cert: '/example/ssl/certs/nogappd/jwt-iss.combined.pem',
        // Verification.
        ou: 'nogfsoiam',
        ca: '/example/ssl/certs/nogappd/ca.pem',
        // Mapping of services to JWT domains for LDAP username and groups.
        domains: [
          { service: 'gitzib', jwtXcrd: 'ZIB' },
        ],
      },
      permissions: [ // See details: 'Rule' in packages/nog-fso/access-fso.js.
        {
          rule: 'AllowInsecureEverything',
          usernames: ['alovelace'],
        },
        {
          rule: 'AllowPrincipalsPathPrefix',
          pathPrefix: '/example/share/',
          principals: ['username:alovelace'],
          actions: ['fso/read', 'fso/write', 'fso/init']
        },
        {
          rule: 'AllowPrincipalsPathPattern],
          pathPattern: '/example/orgfs/srv/:device?/:ou?',
          principals: [ 'ldapgroup:ag-alice' ],
          actions: [ 'fso/list-repos' ],
        },
        {
          rule: 'AllowLdapGroupFromPath',
          pathPattern: '/example/exinst/data/:micro/:group/:path*',
          actions: ['fso/read', 'fso/write']
        },
      ],
      readyJwts: [
        {
          title: 'admin token',
          description: (
            'An admin token provides full access.'
          ),
          path: '/sys/jwts/example/admin/fsoadmin-x-ex904-dev-exuniverse',
          subuser: 'fsoadmin-ex904-dev',
          expiresIn: 24 * 60 * 60,
          scopes: [
            { action: 'api', path: '/' },
            { action: '*', name: '*', path: '*' },
          ],
        },
      ],
    }

`,
  match: matchDevMinifso,
};

// `username` should be backward compatibility with `nog-app`.  Specifically,
// LDAP Bob's Meteor username is `sprohaska` for backward compatibility with
// `nog-app` and `gen-devjwts`, so that the same testing JWT can be used in all
// apps.
function createTestingUser() {
  const bob = {
    username: 'sprohaska',
    services: {
      gittest: {
        username: 'bob',
        ldapgroups: ['org_ag-alice', 'org_ag-bob', 'srv_tem-505'],
      },
    },
  };

  // Keep the user, so that login services can be added and survive restarts.
  const bobActual = Meteor.users.findOne({ username: bob.username });
  if (bobActual) {
    console.log('[nog-app-2] Kept testing user:', bobActual);
    return;
  }

  Meteor.users.insert(bob);
  console.log('[nog-app-2] Added testing user:', bob);
}

function getSettingsMinifso() {
  const settings = Meteor.settings.minifso;
  if (settings !== 'dev') {
    return settings;
  }

  createTestingUser();

  const appdir = process.env.PWD;
  function pemPath(p) {
    return path.join(appdir, '_private/nog-app-2-devcert', p);
  }

  const devSettings = {
    registries: [{
      name: 'example',
      addr: 'localhost:7550',
      cert: pemPath('combined.pem'),
      ca: pemPath('ca.pem'),
      registries: ['exreg'],
      domains: ['EXDOM'],
    }],
    jwt: {
      issuer: 'nogapp',
      ou: 'nogfsoiam',
      ca: pemPath('ca.pem'),
      cert: pemPath('jwt-iss.combined.pem'),
      testingJtis: ['devjwt'],
      domains: [
        { service: 'gittest', jwtXcrd: 'EXAMPLE' },
      ],
    },
    permissions: [
      // This rule must be active to allow API access.
      {
        rule: 'AllowPrincipalsPathEqualOrPrefix',
        path: '/',
        principals: [
          'username:sprohaska',
        ],
        actions: [
          'fso/issue-user-token',
          'fso/issue-sys-token',
        ],
      },

      // It can be useful to temporarily disable `AllowInsecureEverything` to
      // test access checks based on `ldapgroups`.  See `createTestingUser()`
      // above.
      {
        rule: 'AllowInsecureEverything',
        usernames: [
          'sprohaska',
        ],
      },

      // Allow issuing ready JWTs, which is not included in
      // `AllowInsecureEverything`.
      {
        rule: 'AllowPrincipalsPathPrefix',
        pathPrefix: '/sys/jwts/example/',
        principals: [
          'username:sprohaska',
          'username:bzfhombe',
        ],
        actions: [
          'fso/issue-ready-jwt',
        ],
      },

      // The rules below are only used if `AllowInsecureEverything` is
      // disabled.
      {
        rule: 'AllowPrincipalsNames',
        names: [
          'main',
          'exreg',
        ],
        principals: [
          'username:sprohaska',
        ],
        actions: [
          'fso/read-main',
          'fso/read-registry',
        ],
      },
      {
        rule: 'AllowLdapGroups2FromPath',
        pathPattern: '/example/orgfs2/srv/:group1/:group2/(.*)?',
        ldapPrefix1: 'srv_',
        ldapPrefix2: 'org_',
        actions: [
          'fso/find',
          'fso/init-repo',
          'fso/list-repos',
          'fso/list-repos-recursive',
          'fso/read-repo',
          'fso/read-repo-tree',
          'fso/refresh-repo',
          'fso/write-repo',
        ],
      },
    ],
    readyJwts: [
      {
        title: 'Example Admin (dev)',
        description: (
          'An Example Admin (dev) token provides full access for development.'
        ),
        path: '/sys/jwts/example/admin/fsoadmin-x-ex904-dev-exuniverse',
        subuser: 'fsoadmin-ex904-dev',
        expiresIn: 24 * 60 * 60,
        scopes: [
          { action: 'api', path: '/' },
          { action: '*', name: '*', path: '*' },
        ],
      },
    ],
  };
  console.log('[nog-app-2] Using minifso dev settings:', devSettings);
  return devSettings;
}

function initMinifso({
  NsFsoMiniRegistry, NsReadyJwts,
  NogAccess, NogCluster,
}) {
  const settings = getSettingsMinifso();

  if (settings.permissions) {
    // Don't catch, so that uncaught errors terminate the process.
    const statements = compileFsoPermissions(settings.permissions);
    NogAccess.addStatements(statements);
    console.log(
      `[nog-app-2] Installed ${statements.length} access statements.`,
    );
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

  function allRegistryNames() {
    let names = [];
    for (const regd of settings.registries) {
      names = names.concat(regd.registries);
    }
    return names;
  }

  // `domainConns()` returns an array of domain names and corresponding GRPC
  // connections `{ domain, conn }`.
  function domainConns() {
    const dcs = [];
    for (const regd of settings.registries) {
      const conn = conns.get(regd.name);
      for (const domain of (regd.domains || [])) {
        dcs.push({ domain, conn });
      }
    }
    return dcs;
  }

  function allDomainNames() {
    let names = [];
    for (const regd of settings.registries) {
      if (regd.domains) {
        names = names.concat(regd.domains);
      }
    }
    return names;
  }

  const rpcTokenProvider = createFsoTokenProvider({
    issuer: settings.jwt.issuer,
    cert: settings.jwt.cert,
    domains: settings.jwt.domains || [],
    users: Meteor.users,
  });

  const sysCallCreds = createAuthorizationCallCreds(
    rpcTokenProvider.fsoSysToken,
    { username: 'nog-app-2' },
    {
      subuser: 'minifso',
      scopes: [
        { action: AA_FSO_READ_REGISTRY, names: allRegistryNames() },
      ],
    },
  );

  const fsoUnixDomains = {
    domainConns: domainConns(),
    sysCallCreds: null,
  };
  if (fsoUnixDomains.domainConns.length > 0) {
    fsoUnixDomains.sysCallCreds = createAuthorizationCallCreds(
      rpcTokenProvider.fsoSysToken,
      { username: 'nog-app-2' },
      {
        subuser: 'accounts',
        scopes: [
          { action: AA_READ_UNIX_DOMAIN, names: allDomainNames() },
        ],
      },
    );
  }

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
    console.log(`[nog-app-2] Started watching fso ${part.selHuman}:`, names);

    // May be useful during testing:
    //
    // ```
    // Meteor.setTimeout(() => { stopPart(part); }, 1000);
    // ```
  }

  function stopPart(part) {
    const obs = observers.get(part.begin);
    observers.delete(part.begin);

    for (const o of obs) {
      o.observer.stop();
    }

    const names = obs.map(o => o.name);
    console.log(`[nog-app-2] Stopped watching fso ${part.selHuman}:`, names);
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

  // Allow user to delete her own tokens
  NogAccess.addStatement(
    {
      principal: /^userid:[^:]+$/,
      action: 'jwt/delete-user-token',
      effect: (opts) => {
        if (opts.userId === opts.principal.split(':')[1]) {
          return 'allow';
        }
        return 'ignore';
      },
    },
  );

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

  return {
    fsoUnixDomains,
  };
}

export {
  minifsoSetting,
  initMinifso,
};
