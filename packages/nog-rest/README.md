# Package `nog-rest`

`nog-rest` implements server-side routing for a REST API.  It uses
signature-based authentication if the package `nog-auth` is available.

Routes are registered with `NogBlob.actions(prefix, actions)` (see details
below).

Access check of actions should be implemented with `NogAccess.checkAccess()`
from package `nog-access`.

Links to resources should be returned as JSON objects with an `href` member and
other alternative identifiers, such as an `id` or a `sha1`.  Each resource
should contain a self reference in member `_id`.

Usage example:

```{.coffee}
if Meteor.isServer
  actions = NogBlob.api.blobs.actions()
  NogRest.actions '/api/blobs', actions
```

With `NogBlob.api.blobs.actions()`, for example, implemented as follows:

```{.coffee}
class BlobsApi
  constructor: (opts) ->
    @blobs = opts.blobs

  actions: () ->
    [
      { method: 'GET', path: '/:blob', action: @get_blob }
      { method: 'GET', path: '/:blob/content', action: @get_blob_content }
    ]

  # Use `=>` to bind the actions to access this instance's state.
  get_blob: (req) =>
    {params, baseUrl} = req
    params = _.pick params, 'blob'
    check params, { blob: isSha1 }
    action = 'nog-blob/GET-blob'
    NogAccess.checkAccess req.auth?.user, action, req.params
    blob = @blobs.findOne params.blob
    if not blob?
      nogthrow ERR_BLOB_NOT_FOUND, {blob: params.blob}
    res = _.pick blob, 'size', 'status', 'confirmations', 'sha1'
    res._id =
      id: blob._id,
      href: Meteor.absoluteUrl(baseUrl[1..] + '/' + blob._id)
    res.content =
      href: share.getSignedDownloadUrl
        sha1: params.blob
        filename: params.blob + '.dat'
    res

  ...
```

Example response JSON:

```{.json}
{
    "data": {
        "_id": {
            "href": "http://localhost:3000/api/blobs/31968d2e8b58e29e63851cb4b340216026f11f69",
            "id": "31968d2e8b58e29e63851cb4b340216026f11f69"
        },
        "confirmations": [
            {
                "date": "2015-04-27T10:02:31.313Z",
                "message": "..."
            }
        ],
        "content": {
            "href": "https://..."
        },
        "sha1": "31968d2e8b58e29e63851cb4b340216026f11f69",
        "size": 11,
        "status": "available"
    },
    "statusCode": 200
}
```


## `NogRest.actions(prefix, actions)` (server)

`NogBlob.actions(prefix, actions)` adds routes that start with `prefix`.
`actions` is an array of `{method: String, path: String, action: callback}`.
`prefix` and `path` use Express-style syntax as describe at
<https://github.com/component/path-to-regexp>.

The action `callback(req)` receives an HTTP request object, parsed as usual:
URL query in `req.params`, parsed JSON body in `req.body`.  In addition to the
usual fields, `req.auth.user` contains a Meteor user if the request signature
has been verified by `nog-auth`.  `req.baseUrl` contains the part of the URL
that was matched by `prefix`.  It can be used in an action callback to create
URLs that use the same prefix:

    href = Meteor.absoluteUrl(baseUrl[1..] + '/' + blob._id)

The action callback either returns a `result` object or throws an error.

A `result` will be send via HTTP with status code 200 and a JSON body:

```{.coffee}
{
  "statusCode": 200
  "data": result
}
```

If `result` contains a field `statusCode`, its value will be used instead for
the HTTP code and in the JSON body:

```{.coffee}
{
  "statusCode": result.statusCode
  "data": _.omit(result, 'statusCode')
}
```

As a special case, the callback can return a redirect `result`:

```{.coffee}
{
  statusCode: 307
  location: "https://..."
}
```

It will be translated to the expected HTTP redirect.

An error will be translated to an HTTP error status code and a JSON body such
as:

```{.coffee}
{
  "errorCode": "ERR_MATCH",
  "message": "Match error: not a sha1 in field blob",
  "statusCode": 422
}
```

## `NogRest.configure(opts)` (server)

`configure()` updates the active configuration with the provided `opts`:

 - `checkRequestAuth` (`Function`, default: `NogAuth.checkRequestAuth`): The
   authentication hook (see below).

### `NogRest.checkRequestAuth(req)` (server)

`checkRequestAuth(req)` is expected to add `req.auth.user` with the
authenticated user or to throw if the authentication fails.  `req` is a HTTP
request object.
