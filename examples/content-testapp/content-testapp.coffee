if Meteor.isServer
  Migrations.add
    version: 1
    up: NogContent.migrations.addOwnerId

  optForceMigration = false
  if optForceMigration
    Migrations._collection.update {_id: 'control'}, {$set: {locked: false}}

  Meteor.startup ->
    Migrations.migrateTo 'latest'


if Meteor.isClient
  Session.setDefault('counter', 0)


ensureUser = (username, password) ->
  olduser = Meteor.users.findOne {
      username
    }, {
      fields: {'services.nogauth.keys': 1}
    }
  keys = olduser?.services?.nogauth?.keys
  Meteor.users.remove {username}
  uid = Accounts.createUser {username, password}
  Roles.addUsersToRoles uid, ['users']
  if keys?
    Meteor.users.update {
        username
      }, {
        $set: {'services.nogauth.keys': keys}
      }
    console.log "
        Kept previous API key for user #{username}, key id: #{keys[0].keyid}
      "
  else
    key = NogAuth.createKey uid, {keyOwnerId: uid}
    console.log "New testing API key for user #{username}:"
    console.log "export NOG_KEYID=#{key.keyid}"
    console.log "export NOG_SECRETKEY=#{key.secretkey}"

if Meteor.isServer
  Meteor.startup ->
    password = Meteor.settings.public?.tests?.passwords?.user
    check password, String

    ensureUser '__testing__user', password
    ensureUser 'fred', password

    username = '__testing__guest'
    Meteor.users.remove {username}
    uid = Accounts.createUser {username, password}

if Meteor.isServer
  # Enable blob hrefs for testing, although nog-blob is not used.
  NogContent.api.repos.useBlobHrefs = true
  NogRest.actions '/api/repos', NogContent.api.repos.actions_v1()

if Meteor.isServer
  Meteor.publish 'repos', ->
    user = Meteor.users.findOne @userId
    NogContent.repos.find {owner: user?.username}
