# Developer Walkthrough (deprecated)
By Steffen Prohaska
<!--@@VERSIONINC@@-->

The walkthrough below contains step-by-step instructions that demonstrate how
to create an app from scratch using the nog packages.

```{.bg-warning}
2015-08-20 Warning: The information may be outdated.
```

## Blob Upload with REST API and Access Check

Create a new Meteor app with our default settings, commit it to a separate repo,
and check that clickme works (note that Velocity tests are running):

    cd examples/local
    ../../tools/bin/meteor-create tmp-blob-testapp
    cd tmp-blob-testapp

    git init
    git add . && git commit -m init

    meteor

    open http://localhost:3000

Keep `meteor` running, open a second console and continue.  Configure access to
the nog packages and add `nog-blob`:

    cd examples/local/tmp-blob-testapp
    ln -s ../../../packages .
    meteor add nog-blob

The app is now crashing, because the configuration, such as the AWS access key,
is missing.  Create `_private/settings-localhost-test.json`:

    echo '/_private/' >>.gitignore
    mkdir _private
    touch _private/settings-localhost-test.json

Configure the access key and the S3 bucket in
`_private/settings-localhost-test.json`:

```{.json}
{
    "AWSAccessKeyId": "<key-id>",
    "AWSSecretAccessKey": "<secrect-key>",
    "AWSBucketRegion": "<region>",
    "upload": {
        "bucket": "<s3-bucket>",
        "loglevel": 1
    }
}
```

Here `<region>` is the region where the bucket is stored, which is decided when
the bucket is created, e.g. `eu-central-1` (Frankfurt), or `us-east-1` (N.
Virginia). See [AWS Regions and Availability
Zones](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html)
for a complete list.

Restart the app with settings:

    meteor run --settings _private/settings-localhost-test.json

Commit:

    git add . && git commit -m 'add nog-blob'

Add a file input and wire it to upload a file and display upload progress.

In `client/templates/upload.tpl.jade`:

```{.jade}
form
  legend
    h4 upload files
  .form-group
    input.js-upload-files(type='file' name='files[]' multiple)
  +uploadHeading
  hr
  each uploads
    +uploadItem
```

Add it to the layout in `client/layout.tpl.jade`:

```{.jade}
//...
  .container-fluid
    +upload
//...
```

Handle events in `client/templates/upload.coffee`:

```{.coffee}
Template.upload.events
  'change .js-upload-files': (ev) ->
    ev.preventDefault()
    for f in ev.target.files
      console.log f
      NogBlob.uploadFile f, (err, res) ->
        if err
          return console.error err
        console.log 'upload ok'

Template.upload.helpers
  uploads: -> NogBlob.files.find()
```

Commit:

    git add . && git commit -m 'add file input'

Add an error display:

    meteor add nog-error

In `client/layout.tpl.jade`:

```{.jade}
//...
    +header
    +errorDisplay
//...
```

Commit:

    git add . && git commit -m 'add error display'

Add a collection to store the recently uploaded objects and display them:

In `tmp-blob-testapp.coffee`:

```{.coffee}
@Objects = new Mongo.Collection 'objects'

Meteor.methods
  'addObject': (doc) ->
    check doc,
      name: String
      blob: String
    if Meteor.isServer
      doc.createDate = new Date()
      Objects.insert doc

if Meteor.isServer
  Meteor.publish 'recentObjects', ->
    Objects.find {}, {sort: {createDate: -1}, limit: 10}

if Meteor.isClient
  Meteor.subscribe 'recentObjects'
```

Change `client/templates/upload.coffee` to add an object when the upload
completed:

```{.coffee}
      # ...
      NogBlob.uploadFile f, (err, res) ->
        # ...
        Meteor.call 'addObject', {
          name: res.filename
          blob: res.sha1
        }, (err, res) ->
          if err
            return console.error err
          console.log 'ok, added object'
```

Add a template to display the objects:

In `client/templates/recentObjects.tpl.jade`:

```{.jade}
.row
  .col-md-12
    h4 recent objects
each objects
  .row
    .col-md-2
      +aBlobHref
    .col-md-4 #{createDate}
    .col-md-4 #{blob}
```

In `client/templates/recentObjects.coffee`:

```{.coffee}
Template.recentObjects.helpers
  objects: ->
    Objects.find {}, {sort: {createDate: -1}}
```

Add the template to `client/layout.tpl.jade`:

```{.jade}
// ...
  .container-fluid
    +recentObjects
```

Commit:

    git add . && git commit -m 'add recent objects'

Add a REST API for the blobs:

    meteor add nog-rest

In `tmp-blob-testapp.coffee`:

```{.coffee}
if Meteor.isServer
  NogRest.actions '/api/blobs', NogBlob.api.blobs.actions()
```

Add test it:

    curl \
      http://localhost:3000/api/blobs/31968d2e8b58e29e63851cb4b340216026f11f69 |
      python -m json.tool


Commit:

    git add . && git commit -m 'add REST API'

Add REST API authentication:

    meteor add nog-auth

The app is crashing, because it requires a master key for encrypting API keys
when storing them in MongoDB.  Add a master key to
`_private/settings-localhost-test.json`.  You may use the following commands to
generate random ids and secrets:

    head -c 100 /dev/random | openssl dgst -sha256 | head -c 20   # id
    head -c 100 /dev/random | openssl dgst -sha256 | head -c 40   # secret

In `_private/settings-localhost-test.json`:

```{.json}
{
    "NogAuthMasterKeys": [
        { "keyid": "<primary-key-id>", "secretkey": "<secret>" },
        { "keyid": "<old-key-id>", "secretkey": "<secret>" }
    ]
}
```

The first key is the primary key.  Old keys can be provided to support key
rotation.  `nog-auth` will re-encrypt all keys with the primary key when the app
restarts.

Curl should now report an error for unauthenticated requests.

Fix it by creating a fake user.

    meteor add accounts-password

Provide a testing password in `_private/settings-localhost-test.json`:

```{.json}
{
    "tests": {
      "passwords": {
          "user": "d43d7d4833e52ca24bc98dd604c231e47e1d2542"
      }
    }
}
```

Create a fake user in `tmp-blob-testapp.coffee`:

```{.coffee}
if Meteor.isServer then Meteor.startup ->
  password = Meteor.settings.tests?.passwords?.user
  check password, String

  username = '__testing__user'
  olduser = Meteor.users.findOne {username},
    fields: {'services.nogauth.keys': 1}
  keys = olduser?.services?.nogauth?.keys
  Meteor.users.remove {username}
  uid = Accounts.createUser {username, password}
  if keys?
    Meteor.users.update {username},
      { $set: { 'services.nogauth.keys': keys } }
    console.log "Kept previous API key with id: #{keys[0].keyid}"
  else
    key = NogAuth.createKey uid
    console.log 'New testing API key:'
    console.log "export NOG_KEYID=#{key.keyid}"
    console.log "export NOG_SECRETKEY=#{key.secretkey}"
```

The app will print the key to the console.  Configure it and confirm that curl
works with a signed request:

    export NOG_KEYID=<copied>
    export NOG_SECRETKEY=<copied>

    curl $(
      ../../../tools/bin/sign-req GET \
      http://localhost:3000/api/blobs/31968d2e8b58e29e63851cb4b340216026f11f69
    ) | python -m json.tool

Commit:

    git add . && git commit -m 'add auth'

The last step is to restrict upload rights by requiring a logged in user.

    meteor add nog-access

The app should now refuse to upload, and curl should report an error (404).  Fix
this, by assigning a role to the testing user:

    meteor add alanning:roles

In `tmp-blob-testapp.coffee`:

```{.coffee}
  # ...
  Roles.addUsersToRoles uid, ['users']
```

Login from the browser console to upload:

    Meteor.loginWithPassword(
      {username: '__testing__user'},
      '<password-from-setting>'
    );
    Meteor.user();

Commit:

    git add . && git commit -m 'add access control'

To configure an upload size limit, add the following to
`_private/settings-localhost-test.json`:

```{.json}
{
    "public": {
        "upload": {
            "uploadSizeLimit": <limit-in-bytes>
        }
    }
}
```
