{ createNogSyncMain } = require 'meteor/nog-sync'

{
  createContentCollections
  createContentStore
} = NogContent


dropCollection = (coll) ->
  # Insert a fake document to avoid Mongo 'ns not found'.
  coll.insert {}
  coll._dropCollection()


createTestPeers = ->
  rndAlice = 'testalice' + Random.id().toLowerCase()
  rndBob = 'testbob' + Random.id().toLowerCase()

  createPeerContentStore = (opts) ->
    {rnd} = opts
    contentColls = createContentCollections {
      namespace: {coll: "#{rnd}_content"}
    }
    contentStoreOpts = _.extend contentColls, {
      users: Meteor.users
      blobs: new Mongo.Collection "#{rnd}_blobs"
      reposSets: null
      checkAccess: ->
    }
    contentStore = createContentStore contentStoreOpts
    return contentStore

  aliceOwner = "nogsyncbot#{rndAlice}"
  aliceOpts =
    contentStore: createPeerContentStore {rnd: rndAlice}
    checkAccess: sinon.spy()
    testAccess: sinon.spy -> true
    namespace:
      meth: rndAlice + 'meth'
      coll: rndAlice + 'coll'
    settings:
      sync:
        interval_ms: 1000
        afterSnapWait: { min_ms: 0, max_ms: 0 }
        peers: [rndAlice, rndBob]
        us: rndAlice
        remotes: [
          {
            name: rndBob
            url: Meteor.absoluteUrl('')
            namespace:
              meth: rndBob + 'meth'
              coll: rndBob + 'coll'
          }
        ]

  bobOwner = "nogsyncbot#{rndBob}"
  bobOpts =
    contentStore: createPeerContentStore {rnd: rndBob}
    checkAccess: sinon.spy()
    testAccess: sinon.spy -> true
    namespace:
      meth: rndBob + 'meth'
      coll: rndBob + 'coll'
    settings:
      sync:
        interval_ms: 100
        afterSnapWait: { min_ms: 0, max_ms: 0 }
        peers: [rndAlice, rndBob]
        us: rndBob
        remotes: [
          {
            name: rndAlice
            url: Meteor.absoluteUrl('')
            namespace:
              meth: rndAlice + 'meth'
              coll: rndAlice + 'coll'
          }
        ]

  AliceMain = createNogSyncMain aliceOpts
  BobMain = createNogSyncMain bobOpts

  peers = {
    rndAlice, rndBob, aliceOwner, bobOwner, AliceMain, BobMain,
    aliceOpts, bobOpts
  }

  peers.cleanup = ->
    n = 0
    for username in [@aliceOwner, @bobOwner]
      n += Meteor.users.remove {username}
    console.log "[test] TestPeers cleanup removed #{n} Meteor users."

    for c in ['repos', 'commits', 'trees', 'objects', 'blobs']
      dropCollection @aliceOpts.contentStore[c]
      dropCollection @bobOpts.contentStore[c]

    for c in ['synchros', 'commits', 'trees', 'objects']
      dropCollection @AliceMain.syncStore[c]
      dropCollection @BobMain.syncStore[c]

  return peers


module.exports.createTestPeers = createTestPeers
