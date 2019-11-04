# Package `nog-auth`

`nog-auth` provides signature-based authentication of HTTP requests.

See the [apidoc](./apidoc.md) for a details description of the signature
process.

The Meteor template `{{> nogApiKeys}}` provides a UI to manage keys.  Secret
keys are encrypted before they are stored in MongoDB.  The master keys must be
provided in `Meteor.settings.NogAuthMasterKeys` as an array of key objects
`[{keyid: String, secretkey: String}]`.  The first key is the primary key.  Old
keys can be provided to support key rotation.  `nog-auth` will re-encrypt all
keys with the primary key when on startup.  The following commands may be
useful to generate random ids and secrets:

    head -c 100 /dev/random | openssl dgst -sha256 | head -c 20   # id
    head -c 100 /dev/random | openssl dgst -sha256 | head -c 40   # secret

## `NogAuth.checkRequestAuth(req)` (server)

`checkRequestAuth(req)` authenticates an HTTP request.  It throws if the
authentication fails.  It adds the Meteor user that owns the key as
`req.auth.user` if the request was successfully authenticated.  If the signing
key has `scopes` (see `createKey()`), they will be added as
`req.auth.user.scopes`.

`checkRequestAuth()` is used as the authentication hook in `nog-rest`.

## `NogAuth.signRequest(key, req)` (server)

`signRequest(key, req)` signs an HTTP request object with the `key`, which is an
object `{keyid: String, secretkey: String}`.

## `NogAuth.createKey(user, opts)` (server)

`createKey()` creates a new API key for the user id `opts.keyOwnerId` after
an access check that `user` has permission to create a key.

## `NogAuth.createKeySudo(opts)` (server)

`createKeySudo(opts)` creates a new API key for the user id `opts.keyOwnerId`
without access check.  It returns the key object `{keyid, secretkey}`.  The
secret key is encrypted with the primary master key before it is stored in
MongoDB.

`opts` can contain a comment and scopes.  `opts.comment` will be displayed in
`{{> nogApiKeys}}`.  `opts.scopes` (an array of `{action: String, opts:
Object}`) will be returned by `checkRequestAuth()` as `req.auth.user.scopes`.
The scopes can be used by `NogAccess` to restrict the actions that the key can
be used for.

## `NogAuth.deleteKey(user, opts)` (server)

`deleteKey()` delete the access key with id `opts.keyid` after an access check
that `user` has permission to delete the key.

If `opts.keyOnwerId` (a user id) is present, it will be used as an additional
selector when finding the key.  Since `keyid` is assumed to be unique,
`keyOnwerId` is only a additional safety measure.

## `NogAuth.deleteKeySudo(opts)` (server)

Same as `deleteKey()` but without access check.

## `{{> nogApiKeys}}` (client)

A UI widget to create and delete API keys for the user in the data context.

Example:

```jade
with currentUser
  +nogApiKeys
```

## `NogAuth.configure(opts)` (anywhere)

`configure()` updates the active configuration with the provided `opts`:

 - `onerror` (`Function`, default: `NogError.defaultErrorHandler`) is used to
   report errors.
 - `checkAccess` (`Function`, default `NogAccess.checkAccess` if available) is
   used for access control.

### `NogAuth.onerror(err)` (client)

The hook `onerror(err)` is called with errors on the client.

### `NogAuth.checkAccess(user, action, opts)` (server)

The hook `NogAuth.checkAccess(user, action, opts)` is called to check whether
a user has permissions to manage API keys.  See package `nog-access`,
specifically `NogAccess.checkAccess()`.
