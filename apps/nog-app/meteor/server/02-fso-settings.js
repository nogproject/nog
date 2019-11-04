import { defSetting } from 'meteor/nog-settings';
import { Match, check } from 'meteor/check';
import {
  matchFsoPermissions,
  matchFsoHomes,
} from 'meteor/nog-fso';

const matchDev = Match.Where((x) => {
  check(x, String);
  return x === 'dev';
});

const matchAllow = Match.Where((x) => {
  check(x, {
    users: Match.Maybe([String]),
  });
  return true;
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

const matchGitNog = Match.Where((x) => {
  check(x, {
    name: String,
    addr: String,
    cert: String,
    ca: String,
    registries: [String],
  });
  return true;
});

const matchGitlab = Match.Where((x) => {
  check(x, {
    name: String,
    ui: String,
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

const matchFso = Match.Where((x) => {
  check(x, {
    allow: Match.Optional(matchAllow),
    registries: [matchRegistry],
    gitNogs: [matchGitNog],
    gitlabs: [matchGitlab],
    jwt: Match.Optional(matchJwt),
    homes: matchFsoHomes,
    permissions: matchFsoPermissions,
  });
  return true;
});

const matchDevFso = Match.OneOf(matchDev, matchFso);

defSetting({
  key: 'fso',
  val: {
    registries: [],
    gitNogs: [],
    gitlabs: [],
    homes: [],
    permissions: [],
  },
  help: `
\`fso\` specifies file system observer registries.  Use the string \`dev\` to
activate the localhost dev setup.  Production example:

    Meteor.settings.fso: {
      allow: { // Ignored.  Use 'InsecureEverything' permission instead.
        users: ['alovelace'],
      },
      registries: [
        {
          name: 'example',
          addr: 'nogfsoregd.example.org:7550',
          cert: '/example/ssl/certs/nogappd/combined.pem',
          ca: '/example/ssl/certs/nogappd/ca.pem',
          registries: ['exreg', 'exreg2'],
        }
      ],
      gitNogs: [ // DEPRECATED: \`grep optGitNogRegdOnly\` for details.
        {
          name: 'nogfsog2nd.example.org',
          addr: 'nogfsog2nd.example.org:7554',
          cert: '/example/ssl/certs/nogappd/combined.pem',
          ca: '/example/ssl/certs/nogappd/ca.pem'
          registries: ['exreg', 'exreg2'],
        }
      ],
      gitlabs: [
        {
          name: 'git.example.org',
          ui: 'https://git.example.org',
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
      homes: [
        {
          description: 'org',
          principals: [
            'ldapgroup:ou_ag-alice',
          ],
          links: [
            { route: 'listing', path: '/example/orgfs/srv/' },
            { route: 'listing', path: '/example/orgfs/org/' },
            { route: 'untracked', path: '/example/orgfs/srv/' },
            { route: 'untracked', path: '/example/orgfs/org/' },
          ],
        },
      ],
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
        }
        {
          rule: 'AllowLdapGroupFromPath',
          pathPattern: '/example/exinst/data/:micro/:group/:path*',
          actions: ['fso/read', 'fso/write']
        },
      ]
    }

`,
  match: matchDevFso,
});
