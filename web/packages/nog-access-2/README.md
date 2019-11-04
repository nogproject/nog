# Package `nog-access-2`

## Introduction

Package `nog-access-2` provides access control similar to the legacy package
`nog-access`.

See app `access-testapp-2` for an example.

The design is inspired by AWS policies:

 - <http://docs.aws.amazon.com/IAM/latest/UserGuide/PoliciesOverview.html>,
 - <http://docs.aws.amazon.com/IAM/latest/UserGuide/AccessPolicyLanguage_EvaluationLogic.html>.

Access is determined from a list of statements such as:

```javascript
const statements = [
  {
    principal: 'role:users',
    action: 'nog-blob/upload',
    effect: 'allow',
  },
  {
    principal: /^username:[^:]+$/,
    action: 'nog-content/create-repo'
    effect: (opts) => {
      const username = opts.principal.split(':')[1]
      if (username === opts.ownerName) {
        return 'allow';
      }
      return 'ignore';
    },
  }
];
```

The Meteor user is mapped to a list of principals and each principal is then
tested against the access statements.  Access is granted if any of the
statements has the effect `allow` and no statement has the effect `deny`.

The mapping of a user to principals is controlled by `principalPlugins` as
explained for `createAccessModuleServer()` below.  The default mapping is:

* A signed-in user that has roles is mapped to principals `['role:<role.0>',
  ..., 'username:<username>', 'userid:<userid>', 'ldapgroup:<group.0>', ...]`,
* A signed-in user that has no role assigned is mapped to principals
  `['guests', ..., 'username:<username>', ...]`.
* A signed-out connection is mapped to principal `['anonymous']`.

In access statements, `principal` can be a string (exact match) or a regular
expression.

`effect` can be a function `(opts) -> {effect: 'access' | 'deny' | 'ignore',
reason: String}` that is evaluated on the `opts` that are passed to the access
check functions.  The Meteor user object is available as `opts.user` if a user
is signed in.  The original `opts` that are passed to `checkAccess()` must not
contain a field `user`.

The list of access control statements can be built with `addStatements()`.
Statements should be added only during startup.

## `nog-access-2.createAccessModuleServer()` (server)

`createAccessModuleServer()` creates the server module.  See example below.

The list `checkUserPlugins` configures additional access checks that are based
on information on the user object, for example `scopesV2` when accessing the
API with a JWT bearer token.

The list `principalPlugins` controls how a user is mapped to a list of
principals:

* `principalPluginRoles` maps roles, which can be maintained with package
  `alanning:roles`, to `role:<role>` principals.
* `principalPluginUsername` adds a principal `username:<username>`.
* `principalPluginUserid` adds a principal `userid:<id>`.
* `principalPluginLdapgroups` adds principals `ldapgroup:<group>`, based on the
  Meteor user `services` fields.

Example:

```javascript
import {
  createAccessModuleServer,
  checkUserPluginScopesV2,
  principalPluginRoles,
  principalPluginUsername,
  principalPluginUserid,
  principalPluginLdapgroups,
  StatementsIsRoleX,
} from 'meteor/nog-access-2';

const NogAccess = createAccessModuleServer({
  namespace: NsAccess,
  users: Meteor.users,
  checkUserPlugins: [
    checkUserPluginScopesV2,
  ],
  principalPlugins: [
    principalPluginRoles,
    principalPluginUsername,
    principalPluginUserid,
    principalPluginLdapgroups,
  ],
});
NogAccess.addStatements(StatementsIsRoleX);
// ...

const NogOther = createOtherModuleServer({
  checkAccess: NogAccess.checkAccess,
  testAccess: NogAccess.testAccess,
});
```

## `nog-access-2.createAccessModuleClient()` (client)

Example:

```javascript
const NogAccess = createAccessModuleClient({
  namespace: NsAccess,
  userId: Meteor.userId,
});

Meteor.startup(() => {
  renderApp({
    testAccess: NogAccess.testAccess,
    // ...
  });
});
```

## `NogAccess.checkAccess(euid, action, opts)` (server)

`checkAccess(euid, action, opts)` works similar to `Meteor.check()`.  `euid`
can be a user object or a user ID or `null`.  `checkAccess()` throws if access
is denied.  If it returns without throwing, access is granted.  `checkAccess()`
can be used for access control in Meteor methods, publish functions, and REST
request handlers.

## `NogAccess.testAccess(euid, action, opts)` (server)

`testAccess(euid, action, opts)` works similar to `checkAccess()` but returns
`true` (access) or `false` (deny) instead of throwing and exception.

## `NogAccess.testAccess(action, [opts], [callback])` (client)

`testAccess(action, opts)` on the client works similar to `testAccess()` on the
server, but it uses the current user, and it returns `null` if the access test
result is not yet available, because the response from the server is pending.
The result is reactively updated when the server call completes.  If `callback`
is provided, it is called with the result when it becomes available.

## `NogAccess.addStatement(statement)` (server)

`addStatement()` adds a statement to the access control list.

## `NogAccess.addStatements(statements)` (server)

`addStatements()` adds a list of statements to the access control list.
