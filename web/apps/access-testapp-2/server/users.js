import { Meteor } from 'meteor/meteor';
import { check } from 'meteor/check';
import { Accounts } from 'meteor/accounts-base';
import { Roles } from 'meteor/alanning:roles';

function createUser({
  username, password, roles,
}) {
  Meteor.users.remove({ username });
  const uid = Accounts.createUser({ username, password });
  if (roles) {
    Roles.addUsersToRoles(uid, roles);
  }
  console.log(
    '[access-testapp-2] Created user.',
    'username', username,
    'roles', roles,
  );
}

const createUsersSetting = {
  key: 'public.tests.passwords.user',
  val: 'test1234',
  help: `
\`passwords.user\` is the password that is used for testing users.
`,
  match: String,
};

function createUsers() {
  const password = Meteor.settings.public.tests.passwords.user;
  check(password, String);

  createUser({
    username: '__testing__user',
    password,
    roles: ['users'],
  });

  createUser({
    username: '__testing__admin',
    password,
    roles: ['users', 'admins'],
  });

  createUser({
    username: '__testing__guest',
    password,
    roles: null,
  });
}

export {
  createUsers,
  createUsersSetting,
};
