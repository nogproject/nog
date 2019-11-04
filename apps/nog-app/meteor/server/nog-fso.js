import path from 'path';
import { Meteor } from 'meteor/meteor';
import { NogCluster } from 'meteor/nog-cluster';
import { NogAccess } from 'meteor/nog-access';
import { NogRest } from 'meteor/nog-rest';
import { createBearerJwtAuthn } from 'meteor/nog-jwt';
import {
  compileFsoPermissions,
  connectFsoGitNogGrpc,
  connectFsoRegGrpc,
  createAuthorizationCallCreds,
  createFsoModuleServer,
  createFsoTokenProvider,
  observeRegistry,
} from 'meteor/nog-fso';
import { NsFso, NsSuggest } from '../imports/namespace.js';
import { NogCatalog } from 'meteor/nog-catalog';
import { createFsoCatalogPlugin } from 'meteor/nog-catalog-fso';
import { createSuggestModuleServer } from 'meteor/nog-suggest';
import { createFsoCatalogUpdater } from './nog-fso-catalog.js';

function parseSettings() {
  const settings = Meteor.settings.fso;
  if (settings !== 'dev') {
    return settings;
  }

  const appdir = process.env.PWD;
  const devSettings = {
    registries: [{
      name: 'example',
      addr: 'localhost:7550',
      cert: path.join(appdir, '_private/nogappd-devcert/combined.pem'),
      ca: path.join(appdir, '_private/nogappd-devcert/ca.pem'),
      registries: ['exreg'],
    }],
    gitNogs: [{
      name: 'example',
      addr: 'localhost:7554',
      cert: path.join(appdir, '_private/nogappd-devcert/combined.pem'),
      ca: path.join(appdir, '_private/nogappd-devcert/ca.pem'),
      registries: ['exreg'],
    }],
    gitlabs: [{
      name: 'localhost',
      ui: 'http://localhost:10180',
    }],
    jwt: {
      issuer: 'nogapp',
      ou: 'nogfsoiam',
      ca: path.join(appdir, '_private/nogappd-devcert/ca.pem'),
      cert: path.join(appdir, '_private/nogappd-devcert/jwt-iss.combined.pem'),
      testingJtis: ['devjwt'],
      domains: [
        { service: 'gittest', jwtXcrd: 'EXAMPLE' },
      ],
    },
    homes: [
      {
        description: 'everything',
        principals: [
          'username:sprohaska',
        ],
        links: [
          { route: 'help', path: '/example/nog/pub/doc' },
          { route: 'listing', path: '/' },
          { route: 'untracked', path: '/' },
          { route: 'catalog', path: '/example/nog/pub/catalog' },
          { route: 'syssug', path: '/sys/sug/default' },
          { route: 'syssug', path: '/sys/sug/g/visual' },
        ],
      },
      {
        description: 'org',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        links: [
          { route: 'help', path: '/example/nog/org/doc' },
          { route: 'listing', path: '/example/orgfs/srv/' },
          { route: 'listing', path: '/example/orgfs/org/' },
          { route: 'untracked', path: '/example/orgfs/srv/' },
          { route: 'untracked', path: '/example/orgfs/org/' },
          { route: 'catalog', path: '/example/nog/org/catalog' },
          { route: 'syssug', path: '/sys/sug/default' },
          { route: 'syssug', path: '/sys/sug/g/org' },
        ],
      },
    ],
    permissions: [
      // It can be useful to temporarily disable `AllowInsecureEverything`
      // during development when testing access checks.
      {
        rule: 'AllowInsecureEverything',
        usernames: [
          'sprohaska',
        ],
      },
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
      {
        rule: 'AllowPrincipalsPathEqualOrPrefix',
        path: '/',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        actions: [
          'fso/home',
        ],
      },
      { // Allow ag-alice to use `default` and `g/org` suggestion namespaces.
        rule: 'AllowPrincipalsPathPattern',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        actions: [
          'sys/read',
        ],
        pathPattern: '/sys/sug/(default|g/org)',
      },
      { // Allow ag-alice to read documentation.
        rule: 'AllowPrincipalsPathEqualOrPrefix',
        path: '/example/nog/org',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        actions: [
          'fso/read-repo',
          'fso/read-repo-tree',
        ],
      },
      {
        rule: 'AllowPrincipalsPathEqualOrPrefix',
        path: '/example',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        actions: [
          'fso/discover',
        ],
      },
      {
        rule: 'AllowLdapGroups2FromPath',
        pathPattern: '/example/orgfs/srv/:group1/:group2/(.*)?',
        ldapPrefix1: 'srv_',
        ldapPrefix2: 'ou_',
        actions: [
          'fso/discover-root',
          'fso/init-repo',
          'fso/list-repos',
          'fso/list-repos-recursive',
          'fso/read-repo',
          'fso/read-repo-tree',
          'fso/refresh-repo',
          'fso/write-repo',
        ],
      },
      {
        rule: 'AllowPrincipalsPathPattern',
        pathPattern: '/example/orgfs/srv/:device?',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        actions: [
          'fso/list-repos',
        ],
      },
      {
        rule: 'AllowLdapGroupFromPath',
        pathPattern: '/example/orgfs/org/:group/(.*)?',
        ldapPrefix: 'ou_',
        actions: [
          'fso/discover-root',
          'fso/init-repo',
          'fso/list-repos',
          'fso/list-repos-recursive',
          'fso/read-repo',
          'fso/read-repo-tree',
          'fso/refresh-repo',
          'fso/write-repo',
        ],
      },
      {
        rule: 'AllowPrincipalsPathPattern',
        pathPattern: '/example/orgfs/org',
        principals: [
          'ldapgroup:ou_ag-alice',
        ],
        actions: [
          'fso/list-repos',
        ],
      },
    ],
  };
  console.log('[fso] Using dev settings.', devSettings);
  return devSettings;
}

const settings = parseSettings();

if (settings.registries.length === 0) {
  console.log('[fso] FSO disabled.');
  return;
}

if (settings.allow) {
  console.error(
    `[fso] Warning: Ignored setting \`fso.allow.users\`.  ` +
    'Use `fso.permissions` with rule `AllowInsecureEverything` instead.',
  );
}

if (settings.permissions) {
  try {
    const statements = compileFsoPermissions(settings.permissions);
    for (const s of statements) {
      NogAccess.addStatement(s);
    }
    console.log(`[fso] Installed ${statements.length} access statements.`);
  } catch (err) {
    console.error(
      '[fso] Failed to install access statements.',
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

// `registryConns()` returns an array of registry names and corresponding GRPC
// connections `{ registry, conn }`.
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

// Connections by `g2nd.name`.
const g2nConns = new Map();
// Create connections during startup to confirm that the settings are sane.
// But start observing only after acquiring a cluster lease.
for (const gitNog of settings.gitNogs) {
  g2nConns.set(gitNog.name, connectFsoGitNogGrpc({
    addr: gitNog.addr, certFile: gitNog.cert, caFile: gitNog.ca,
  }));
}

// `gitNogConns()` returns an array of registry names and corresponding GRPC
// GitNog connections `{ registry, conn }`.
function gitNogConns() {
  const rcs = [];
  for (const gn of settings.gitNogs) {
    const conn = g2nConns.get(gn.name);
    for (const registry of gn.registries) {
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

const authn = createBearerJwtAuthn({
  users: Meteor.users,
  issuer: settings.jwt.issuer,
  audience: settings.jwt.issuer,
  ou: settings.jwt.ou,
  ca: settings.jwt.ca,
  testingJtis: settings.jwt.testingJtis || [],
});

NogRest.configure({
  authenticateFromHeader: authn.authenticateFromHeaderFunc(),
});

const AA_BC_READ = 'bc/read';
const AA_FSO_READ_REGISTRY = 'fso/read-registry';
const AA_FSO_READ_REPO = 'fso/read-repo';
const AA_FSO_CONFIRM_REPO = 'fso/confirm-repo';

function allRegistryNames() {
  let names = [];
  for (const reg of settings.registries) {
    names = names.concat(reg.registries);
  }
  return names;
}

const sysCallCredsNogapp = createAuthorizationCallCreds(
  rpcTokenProvider.fsoSysTokenFunc(),
  { username: 'nogapp' },
  {
    subuser: 'nogfso',
    scopes: [
      { action: AA_BC_READ, name: '*' },
      { action: AA_FSO_READ_REGISTRY, names: allRegistryNames() },
      { action: AA_FSO_READ_REPO, path: '/*' },
      { action: AA_FSO_CONFIRM_REPO, path: '/*' },
    ],
  },
);

// `catalogUpdater` updates fso catalogs in the background.  It watches
// broadcast and receives explicit update triggers from `NogFso`.
const catalogUpdater = createFsoCatalogUpdater({
  catalogs: NogCatalog.catalogs,
  updateCatalogFso: NogCatalog.updateCatalogFso,
});

const NogFso = createFsoModuleServer({
  namespace: NsFso,
  checkAccess: NogAccess.checkAccess,
  testAccess: NogAccess.testAccess,
  publisher: Meteor,
  registryConns: registryConns(),
  gitNogConns: gitNogConns(),
  gitlabs: settings.gitlabs,
  homes: settings.homes,
  rpcTokenProvider,
  rpcSysCallCreds: sysCallCredsNogapp,
  catalogUpdater,
});

NogCatalog.registerPlugin(createFsoCatalogPlugin({
  registries: NogFso.registries,
  repos: NogFso.repos,
  checkAccess: NogAccess.checkAccess,
  testAccess: NogAccess.testAccess,
  registryConns: registryConns(),
  rpcAuthorization: rpcTokenProvider.fsoTokenFunc(),
}));

const NogSuggest = createSuggestModuleServer({
  namespace: NsSuggest,
  checkAccess: NogAccess.checkAccess,
  openFsoRepo: NogFso.openRepo,
});

NogCatalog.registerNogSuggest(NogSuggest);

NogFso.broadcast.start();
process.on('exit', () => NogFso.broadcast.stop());

catalogUpdater.startWatchBroadcast(NogFso.broadcast);
process.on('exit', () => catalogUpdater.stopWatchBroadcast(NogFso.broadcast));

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
    const conn = conns.get(regd.name);
    for (const reg of regd.registries) {
      const o = observeRegistry({
        conn,
        sysCallCreds: sysCallCredsNogapp,
        registries: NogFso.registries,
        repos: NogFso.repos,
        registryName: reg,
      });
      obs.push({
        name: `${regd.name}.${reg}`,
        observer: o,
      });
    }
  }

  observers.set(part.begin, obs);

  const names = obs.map(o => o.name);
  console.log(`[app] Started watching fso ${part.selHuman}:`, names);

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
  console.log(`[app] Stopped watching fso ${part.selHuman}:`, names);
}

const partition = new NogCluster.IdPartition({ name: 'fso', max: 2 });
partition.onacquire = (part) => {
  catalogUpdater.startPart(part);
  startPart(part);
};
partition.onrelease = (part) => {
  catalogUpdater.stopPart(part);
  stopPart(part);
};
NogCluster.registerHeartbeat(partition);

NogRest.actions('/api/v1/fso', NogFso.apiActions);
