import { check } from 'meteor/check';
import { Meteor } from 'meteor/meteor';
import { Accounts } from 'meteor/accounts-base';

function createUser({
  username, password, email, ldapUsername, ldapgroups,
}) {
  const user = Meteor.users.findOne({ username });
  if (user) {
    console.log(`[fso-testapp] Kept testing user ${username}.`);
    return user;
  }

  const uid = Accounts.createUser({ username, password, email });
  console.log(`[fso-testapp] Created testing user ${username}.`);
  Meteor.users.update({ username }, {
    $set: {
      'services.gittest.username': ldapUsername,
      'services.gittest.ldapgroups': ldapgroups,
    },
  });
  return Meteor.users.findOne(uid);
}

const publicTestsPasswordsUserSetting = {
  key: 'public.tests.passwords.user',
  val: 'test1234',
  help: `
\`passwords.user\` is the password that is used for testing users.
`,
  match: String,
};

// `createTestingUsers()` creates testing users.
//
// `username` and `ldapUsername` are different, because we want to support
// scenarios where a user have accounts in multiple LDAP domains, and all
// usernames may differ.
//
// `username` should be backward compatibility with `nog-app`.  Specifically,
// Bob's username is `sprohaska` for backward compatibility with `nog-app` and
// `gen-devjwts`, so that the same testing JWT can be used in all apps.
function createTestingUsers() {
  const password = Meteor.settings.public.tests.passwords.user;
  check(password, String);

  const alice = createUser({
    username: 'alovelace',
    password,
    email: 'alice@example.com',
    ldapUsername: 'alice',
    ldapgroups: ['ou_ag-alice', 'srv_lm1'],
  });

  const bob = createUser({
    username: 'sprohaska',
    password,
    email: 'bob@example.com',
    ldapUsername: 'bob',
    ldapgroups: [],
  });

  const testingUsers = {
    alice, bob,
  };
  return testingUsers;
}

export {
  createTestingUsers,
  publicTestsPasswordsUserSetting,
};
