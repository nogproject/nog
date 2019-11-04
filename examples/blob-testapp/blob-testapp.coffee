@Objects = new Mongo.Collection 'objects'

if Meteor.isClient
  Session.setDefault('counter', 0)

Meteor.methods
  'setUploadSizeLimit': (uploadSizeLimit) ->
    check uploadSizeLimit, Number
    if Meteor.isServer
      NogAccess.configure {uploadSizeLimit}

  'addObject': (doc) ->
    check doc,
      name: String
      blob: String
    if Meteor.isServer
      doc.createDate = new Date()
      Objects.insert doc

if Meteor.isServer
  Meteor.publish 'objects', ->
    Objects.find({}, {sort: {createDate: -1}, limit: 10})

  crypto = Npm.require 'crypto'
  Meteor.startup ->
    password = Meteor.settings.public?.tests?.passwords?.user
    check password, String

    username = '__testing__user'
    olduser = Meteor.users.findOne {username},
      fields: {'services.nogauth.keys': 1}
    keys = olduser?.services?.nogauth?.keys
    Meteor.users.remove {username}
    uid = Accounts.createUser {username, password}
    Roles.addUsersToRoles uid, ['users']
    if keys?
      Meteor.users.update {username},
        { $set: { 'services.nogauth.keys': keys } }
      console.log "Kept previous API keys: #{(k.keyid for k in keys)}"
    else
      key = NogAuth.createKey uid, {keyOwnerId: uid}
      console.log 'New testing API key:'
      console.log "export NOG_KEYID=#{key.keyid}"
      console.log "export NOG_SECRETKEY=#{key.secretkey}"

    username = '__testing__guest'
    Meteor.users.remove {username}
    uid = Accounts.createUser {username, password}

if Meteor.isServer
  actions = NogBlob.api.blobs.actions()
  NogRest.actions '/api/blobs', actions
  NogRest.actions '/api/repos/:owner/:repo/db/blobs', actions

  # Api `uploads` must be mounted at the same path as `blobs` to return correct
  # hrefs.
  actions = NogBlob.api.uploads.actions()
  NogRest.actions '/api/blobs', actions
  NogRest.actions '/api/repos/:owner/:repo/db/blobs', actions
