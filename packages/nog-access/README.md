# Package `nog-access`

`nog-access` provides access control that is inspired by AWS:

 - <http://docs.aws.amazon.com/IAM/latest/UserGuide/PoliciesOverview.html>,
 - <http://docs.aws.amazon.com/IAM/latest/UserGuide/AccessPolicyLanguage_EvaluationLogic.html>.

Access is determined from a list of statements such as:

```{.coffee}
{
  principal: 'role:users'
  action: 'nog-blob/upload'
  effect: 'allow'
}
{
  principal: /// ^ username : [^:]+ $ ///
  action: 'nog-content/create-repo'
  effect: (opts) ->
    userName = opts.principal.split(':')[1]
    if userName is opts.ownerName
      'allow'
    else
      'ignore'
}
{
  principal: 'role:users'
  action: 'nog-blob/upload'
  effect: (opts) ->
    if not config.uploadSizeLimit
      'ignore'
    else if config.uploadSizeLimit is 0
      'ignore'
    else if not opts?.size?
      'ignore'
    else if opts.size <= config.uploadSizeLimit
      'allow'
    else
      {
        effect: 'deny'
        reason: "
            Upload is larger than the size limit of #{config.uploadSizeLimit}
            Bytes.
          "
      }
}
```

Access control uses the roles package `alanning:roles`.  The current user is
expanded to an array of principals `['username:<username>', 'userid:<userid>',
'role:<role.0>', 'ldapgroup:<group.0>', '...]`, and each expanded principal is
tested against the access statements.  Access is granted if any of the
statements has the `effect: 'allow'` and no statement has the `effect: 'deny'`.

Logged-in users that have no role assigned are tested as principals
`['username:<username>', 'userid:<userid>', 'guests']`.

Logged-out connections are tested as principal `['anonymous']`.

`principal` can be a string (exact match) or a regular expression.

`effect` can be a function `(opts) -> {effect: 'access' | 'deny' | 'ignore',
reason: String}` that is evaluated on the `opts` that are passed to the access
check functions.  The Meteor user object is available as `opts.user` if a user
is known.  The original `opts` passed to `checkAccess()` cannot contain a field
`user`.

The list of access control statements can be manipulated with
`removeStatements()` and `addStatement()` (see below).  This should only be
done during startup.  It is not yet clear whether we will maintain most
statements centrally in the default list or use `addStatement()` to add them
if needed.

If the user doc contains `user.scopes`, a special pre-check will be performed
whether the action is permitted.  `opts.scopes` is an array of objects
`{action: String, opts: Object}`.  The access check `action` and `opts` are
compared against each scope.  An equality match for one scope is required for
the access check to proceed to the statement processing phase.  Access is
denied otherwise.

Usage example:

```{.coffee}
NogBlob =
  checkAccess: ->

# Use nog-access if available (weak dependency).
if Meteor.isServer
  if (p = Package['nog-access'])?
    console.log '[nog-blob] using nog-access default policy.'
    NogBlob.checkAccess = p.NogAccess.checkAccess
  else
    console.log '
        [nog-blob] default access control disabled, since nog-access is not
        available.
      '

Meteor.methods
  'startMultipartUpload': (opts) ->
    check opts, {name: String, size: Number, sha1: isSha1}
    if not Meteor.isServer then return
    NogBlob.checkAccess Meteor.user(), 'nog-blob/upload',
      _.pick(opts, 'size')
    ...
```

## `NogAccess.checkAccess(user, action, opts)` (server)

`checkAccess(user, action, opts)` works similar to `Meteor.check()`.  `user` may
be an object or an user id or `null`.  `checkAccess()` throws if access is
denied or simply returns if access is granted.  `checkAccess()` can be used for
access control in Meteor methods, publish functions and REST request handlers.

Example use in method:

```{.coffee}
Meteor.methods
  'startMultipartUpload': (opts) ->
    ...
    NogBlob.checkAccess Meteor.user(), 'nog-blob/upload', opts
    ...
```

Example use in publish function:

```{.coffee}
Meteor.publish 'nog-blob/blobs', (sha1s) ->
  NogAccess.checkAccess @userId, 'nog-blob/upload', {sha1s}
  ...
```

Since an exception is thrown if access is denied, the subscription is terminated
in this case.  Access deny errors will be reported to the client's `onStop()`
subscribe callback.  Example:

```{.coffee}
Meteor.subscribe 'nog-blob/blobs', [],
  onStop: (err) ->
    console.error err
```

The publish function will not be re-run by the server if the logged-in user
changes.  The client code needs to explicitly call `Meteor.subscribe()` again.
If this is a problem, `testAccess()`, which returns `false` if access is denied,
may be the better alternative.  Example:

```{.coffee}
Meteor.publish 'nog-blob/blobs', (sha1s) ->
  if not NogAccess.testAccess @userId, 'nog-blob/upload', {sha1s}
    return null
  ...
```

Example use in REST API handler:

```{.coffee}
NogAccess.checkAccess req.auth?.user, action, opts
```

`req.auth.user` is added by `nog-auth` during signature verification.

## `NogAccess.testAccess(user, action, opts)` (server)

`testAccess(user, action, opts)` works similar to `checkAccess()` but returns
`true` (access) or `false` (deny) instead of throwing and exception.

## `NogAccess.testAccess(action, [opts], [callback])` (client)

`testAccess(action, opts)` on the client works similar to `testAccess()` on the
server but returns `null` if the result is not yet available, because the
response from the server is pending.  The result is reactively updated when the
server call completes.  If `callback` is provided, it is called with the result
when it becomes available.

`testAccess_ready(action, opts)` can be used to test whether the result is
available.

Template helper example:

```{.coffee}
Template.repoView.helpers
  mayModify: ->
    ...
    NogAccess.testAccess 'nog-content/modify', {ownerName, repoName}
```

Flow router middleware example:

```{.coffee}
requireUserOrGuest = (path, next) ->
  NogAccess.testAccess 'isUser', (err, isUser) ->
    NogAccess.testAccess 'isGuest', (err, isGuest) ->
      if isUser or isGuest
        next()
      else
        next('/sign-in')
```

### `{{testAccess action [kwopts]}}` (client)

`{{testAccess action}}` can be used in a template to test access.  It calls
`NogAccess.testAccess(action)`.

`{{testAccess_ready action}}` can be used to test whether the test results is
available.

Example:

```jade
  if testAccess_ready 'nog-blob/upload'
    if testAccess 'nog-blob/upload'
      +uploadView
    else
      | You cannot upload files.
  else
    | loading...
```

Keyword arguments are passed as an `opts` object to `NogAccess.testAccess()`.
Example:

```jade
  if testAccess 'nog-content/modify' ownerName='foo' repoName='bar'
    +repoView
  else
    | You cannot modify the repo.
```

It may be clearer to write a helper function that performs the toggle check if
an `opts` object is needed.

### `{{testAccess_ready action [kwopts]}}` (client)

`{{testAccess_ready action [kwopts]}}` tests whether the test result is
available.


## `NogAccess.configure(opts)` (server)

`configure()` updates `NogAccess.config` with the provided `opts`:

 - `uploadSizeLimit` (`Number >= 0`, default:
   `Meteor.settings.public.upload.uploadSizeLimit` or `0`) limits the allowed
   blob upload size.  Use `0` to disable the limit.

### `NogAccess.config` (server)

The active configuration.

### `NogAccess.addStatement(statement)` (server)

`addStatement(statement)` adds a statement to the access control list.  See the
introduction above for the format of `statement`.

### `NogAccess.removeStatements(selector)` (server)

`removeStatetments(selector)` removes the statements that match `selector` from
the access control list.  `selector` can contain only a single field `action`.
Statements whose action matches (exact string comparison) are removed.
