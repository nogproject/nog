import path from 'path';
import { Meteor } from 'meteor/meteor';
import { Match, check } from 'meteor/check';
import { matchFsoPermissions } from 'meteor/nog-fso-authz';
import { matchReadyJwt } from 'meteor/nog-ready-jwts';

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

function getSettingsMinifso() {
  const settings = Meteor.settings.minifso;
  if (settings !== 'dev') {
    return settings;
  }

  const appdir = process.env.PWD;
  function pemPath(p) {
    return path.join(appdir, '_private/fso-testapp-devcert', p);
  }

  const devSettings = {
    registries: [{
      name: 'example',
      addr: 'localhost:7550',
      cert: pemPath('combined.pem'),
      ca: pemPath('ca.pem'),
      registries: ['exreg'],
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
      // It can be useful to temporarily disable `AllowInsecureEverything`
      // during development when testing access checks.
      //
      // Username `sprohaska` has ldapUsername `bob`.
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
        rule: 'AllowPrincipalsPathPrefix',
        pathPrefix: '/sys/jwts/example/',
        principals: [
          'username:sprohaska',
        ],
        actions: [
          'fso/issue-ready-jwt',
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
  console.log('[app] Using minifso dev settings:', devSettings);
  return devSettings;
}

export {
  minifsoSetting,
  getSettingsMinifso,
};
