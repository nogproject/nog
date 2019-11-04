import { defSetting } from 'meteor/nog-settings';
import { Meteor } from 'meteor/meteor';
import { Match, check } from 'meteor/check';


const matchDev = Match.Where((x) => {
  check(x, String);
  return x === 'dev';
});

defSetting({
  key: 'public.announcements',
  val: [],
  help: `
\`announcements\` is a list of strings that are displayed below the page
header.  Each string is placed in a separate box.  The primary purpose is to
display temporary status messages, like a planned maintenance window.
`,
  match: [String],
});

defSetting({
  key: 'oauthSecretKey',
  val: null,
  help: `
\`oauthSecretKey\` is the secret for encrypting OAuth access tokens before
storing them to MongoDB.  Encryption is mandatory in production.  Valid keys
can be created with:

    node -p 'require("crypto").randomBytes(16).toString("base64")'

`,
  match: Meteor.isDevelopment ? Match.Maybe(String) : String,
});

defSetting({
  key: 'from',
  val: 'root@localhost',
  help: `
\`from\` is the sender email address for emails sent by the application.
`,
  match: String,
});

defSetting({
  key: 'adminEmails',
  val: ['root@localhost'],
  help: `
\`adminEmails\` is a list of email addresses to which notifications about new
user registrations will be sent.
`,
  match: [String],
});

defSetting({
  key: 'public.searchNumResults',
  val: 10,
  help: `
\`searchNumResults\` is the maximum number of displayed result items during nog
search and the increment for the load-more option.
`,
  match: Number,
});

const matchLdapSettings = Match.Where((x) => {
  check(x, {
    service: String,
    url: String,
    groupDn: String,
    userDn: String,
    autoRegisterGroups: [String],
  });
  return true;
});

defSetting({
  key: 'ldap',
  val: [],
  help: `
\`ldap\` controls how user group information is retrieved from LDAP.  It is a
list of configuration objects that corresponding to account services, like
 \`gitzib\`.

    Meteor.settings.ldap: [
      {
        service: 'gitexample',
        url: 'ldap://ldap.example.com',
        groupDn: 'ou=group,dc=example,dc=com',
        userDn: 'ou=People,dc=example,dc=com',
        autoRegisterGroups: ['users'],
      },
    ]

Accounts with an LDAP group listed in \`autoRegisterGroups\` will be
automatically assigned role \`users\`.
`,
  match: [matchLdapSettings],
});

const matchWellknownAccount = Match.Where((x) => {
  check(x, {
    username: String,
    accountType: String,
    aka: {
      github: Match.Optional(String),
      gitimp: Match.Optional(String),
      gitzib: Match.Optional(String),
    },
  });
  return (
    !!x.accountType.match(/^(password|github|gitimp|gitzib)$/) &&
    Object.keys(x.aka).length > 0
  );
});

const matchWellknownAccountsSettings = [matchWellknownAccount];

defSetting({
  key: 'wellknownAccounts',
  val: [],
  help: `
\`wellknownAccounts\` is a list of account aliases.  If an alias is detected
during sign in, it will be added to the primary account.  Examples:

    Meteor.settings.wellknownAccounts: [
      {
        username: 'sprohaska', accountType: 'password',
        aka: { github: 'sprohaska' },
      },
    ]

    Meteor.settings.wellknownAccounts: [
      {
        username: 'sprohaska', accountType: 'github',
        aka: { gitzib: 'ziblogin' },
      },
    ]

Use the special value \`dev\` to use the dev settings.
`,
  match: Match.OneOf(matchDev, matchWellknownAccountsSettings),
});

console.log('[app] Completed default settings definitions.');
