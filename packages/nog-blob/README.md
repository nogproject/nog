# Package `nog-blob`

`nog-blob` implements content-addressable file storage on AWS S3.  The S3 object
key is the sha1 of the file content.  The sha1 is computed in the browser before
uploading the content.  Uploaded objects are stored in the MongoDB collection
`blobs`, which is available as `NogBlob.blobs`.

The REST API is described in [apidoc-blobs](./apidoc-blobs.md) and
[apidoc-upload](./apidoc-upload.md).

## `NogBlob.blobs` (server, subset at client)

A `Mongo.Collection` with information about the uploaded blobs.  `_id` is the
content sha1.

Clients are automatically subscribed to a subset that is relevant for the
current uploads from the client.

## `NogBlob.uploadFile(file, done | opts)` (client)

`uploadFile(file, callbacks)` starts an upload of a web `File` object.  It
returns an id for the client-only collection `NogBlob.files` immediately and
calls `done(err, res)` upon completion.  `res` is an object `{_id: String,
filename: String, size: Number, sha1: String}`.

The second argument can be an object `opts` with functions `done(err, res)`,
and `onwarning(err)`.  It so, `onwarning()` is called for reporting
intermediate warnings instead of the default `NogBlob.onerror()`.  Eventually,
`done()` is called as described in the previous paragraph.

Usage example:

```{.jade}
template(name='upload')
  form
    .form-group
      input#files(type='file' name='files[]' multiple)
```

```{.coffee}
Template.upload.events
  'change #files': (e) ->
    e.preventDefault()
    for f in e.target.files
      id = NogBlob.uploadFile f, (err, res) ->
        if err
          return console.log 'failed to upload file', err
        # Do something with res; for example call server to add it.
        Meteor.call 'addObject',
          name: res.filename
          blob: res.sha1
      Session.set 'currentUpload', id
```

### `NogBlob.files` (client)

A client-only collection that provides a reactive data source to track upload
progress.

### `NogBlob.fileHelpers` (client)

`fileHelpers` is an object with template helper functions that can be used to
implement a UI to display file upload progress.  The helper functions expect
a document from `NogBlob.files` in the data context.

Usage example:

```{.jade}
template(name='currentUpload')
  .row
    if haveUpload
      with file
        .col-md-3
          span #{name}
        .col-md-9
          .progress
            div(
              class="progress-bar {{uploadCompleteClass}}",
              role="progressbar",
              style="width: {{progressWidth}}%"
            )
```

```{.coffee}
currentUpload = -> Session.get('currentUpload')

Template.currentUpload.helpers
  haveUpload: -> currentUpload()?
  file: -> NogBlob.files.findOne currentUpload()

Template.currentUpload.helpers _.pick(
  NogBlob.fileHelpers, 'name', 'progressWidth', 'uploadCompleteClass'
)
```

### `{{> uploadHeading}}` and `{{> uploadItem}}` (client)

`{{> uploadHeading}}` and `{{> uploadItem}}` are templates that illustrated
how to display a list of uploads.  We probably will not use them as is in the
production app.  Either we improve them or we build a custom UI using the
`fileHelpers` described above.

```{.jade}
template(name='upload')
  +uploadHeading
  hr
  each uploads
    +uploadItem
```

```{.coffee}
Template.upload.helpers
  uploads: -> NogBlob.files.find()
```

## `{{> aBlobHref blob=<sha1> name=<filename>}}` (client)

The template `{{> aBlobHref blob=<sha1> name=<filename>}}` inserts an `<a>`
element that when clicked will download the blob from S3 and save it as the
specified filename.

## `NogBlob.configure(opts)` (anywhere)

`configure()` updates the active configuration with the provided `opts`:

 - `onerror` (`Function`, default: `NogError.defaultErrorHandler`) is used to
   report errors.
 - `checkAccess` (`Function`, default `NogAccess.checkAccess` if available) is
   used for access control.
 - `repoSets` (instance of `NogContent.RepoSets` or `false`, default: `false`
   when used without package `nog-content` and `true` when used with
   `nog-content`) is used internally by the package `nog-content` to inject an
   implementation that checks whether a blob is reachable from a repo.  Checks
   are currently only implemented in the `api.*.actions` but not for method
   calls.  See source for details.
 - See source `nog-blob.coffee` for further configuration options.

### `NogBlob.config` (anywhere)

The currently active configuration.

### `NogBlob.onerror(err)` (client)

The hook `onerror(err)` is called with errors on the client.

### `NogBlob.checkAccess(user, action, opts)` (server)

The hook `NogAuth.checkAccess(user, action, opts)` is called to check whether
a user has the necessary upload and download permissions.  See package
`nog-access`.

## `NogBlob.api.blobs.actions()` (server)

`NogBlob.api.blobs.actions()` returns an action array that can be plucked into
`nog-rest` to provide a REST API.

The mount path must contain `:ownerName` and `:repoName` when used with
`repoSets` for repo membership checks, which is the default when package
`nog-content` is part of the app.

The REST API is described in [apidoc-blobs](./apidoc-blobs.md).

Usage examples:

```{.coffee}
if Meteor.isServer
  NogRest.actions '/api/blobs', NogBlob.api.blobs.actions()
```

```{.coffee}
if Meteor.isServer
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.blobs.actions()
```

### `NogBlob.api.uploads.actions()` (server)

`NogBlob.api.uploads.actions()` returns an action array that can be plucked into
`nog-rest` to provide a REST API for uploading blobs.  The `upload.actions()`
must be mounted at the same path as the `blobs.actions()`.

The mount path must contain `:ownerName` and `:repoName` when used with
`repoSets` for repo membership checks, which is the default when package
`nog-content` is part of the app.

The REST API is described in [apidoc-upload](./apidoc-upload.md).

The Python example `blob-testapp/public/tools/bin/test-upload-py` demonstrates
the preferred way of using the REST API for uploading data.

The Bash example `blob-testapp/public/tools/bin/test-upload` also demonstrates
how to upload data, but the example is not ideal.  It ignores the initial
`parts`, and it constructs URLs from identifiers instead of using the provided
`href` fields.

Usage examples:

```{.coffee}
if Meteor.isServer
  NogRest.actions '/api/blobs', NogBlob.api.blobs.actions()
  NogRest.actions '/api/blobs', NogBlob.api.upload.actions()
```

```{.coffee}
if Meteor.isServer
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.blobs.actions()
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.uploads.actions()
```


## `NogBlob.call.*` (anywhere, internal use)

The object `NogBlob.call` provides Meteor methods that are used internally, such
as `NogBlob.call.startMultipartUpload()` or `NogBlob.call.getBlobDownloadURL()`.
