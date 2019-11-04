{ createTestPeers } = require './nog-sync-peer-tests.coffee'


describe 'nog-sync', ->

  peers = null
  rndAlice = null
  rndBob = null
  aliceOwner = null
  AliceMain = null
  BobMain = null
  euid = null

  before ->
    peers = createTestPeers()
    rndAlice = peers.rndAlice
    rndBob = peers.rndBob
    aliceOwner = peers.aliceOwner
    AliceMain = peers.AliceMain
    BobMain = peers.BobMain

  after ->
    peers.cleanup()
    AliceMain.syncStore.synchros.remove {owner: aliceOwner}


  describe 'config', ->

    it 'maps us to nogsyncbot user', ->
      {config} = AliceMain
      expect(config.ourUsername(config.us)).to.eql "nogsyncbot#{rndAlice}"


  # We do not use a fake user collection, because `ensureSyncUsers()` uses the
  # roles package and it is not obvious how to use a fake collection with it.

  describe 'ensureSyncUsers()', ->

    it 'creates users for all peers', ->
      for peer in [rndAlice, rndBob]
        user = Meteor.users.findOne({username: "nogsyncbot#{peer}"})
        expect(user).to.not.exist

      AliceMain.ensureSyncUsers()

      for peer in [rndAlice, rndBob]
        user = Meteor.users.findOne({username: "nogsyncbot#{peer}"})
        expect(user).to.exist


  describe 'ensureMainSynchro()', ->

    it 'initializes the all synchro', ->
      AliceMain.ensureMainSynchro euid
      synchros = AliceMain.syncStore.synchros
      expect(synchros._name).to.eql "#{rndAlice}coll.synchros"
      expect(synchros.findOne({owner: aliceOwner, name: 'all'})).to.exist


  describe 'startMainSynchro()', ->

    it 'starts event processing', (done) ->

      # The test uses a ping to confirm event processing.  The ping token will
      # be delivered via the syncLoop to the syncStore, which will store it in
      # the synchro doc.
      #
      # First install an observer that detects the change.  It uses `added`,
      # because the selector initially excludes the doc, so it appears added.
      #
      # Then trigger the ping.

      synchros = AliceMain.syncStore.synchros
      token = Random.id()
      pingKey = "_ping.nogsyncbot#{rndAlice}"
      sel = {owner: aliceOwner, name: 'all'}
      sel[pingKey] = {$eq: token}
      fields = {}
      fields[pingKey] = 1
      obs = synchros.find(sel, {fields}).observe({
        added: (doc) ->
          expect(doc._ping?["nogsyncbot#{rndAlice}"]).to.eql token
          obs.stop()
          AliceMain.stopMainSynchro euid
          done()
      })

      AliceMain.startMainSynchro euid
      AliceMain.pingMainSynchro euid, {token}


  describe 'connectRemotes()', ->

    it 'connects and starts observing remotes', (done) ->

      # The test connects from Alice to Bob and checks that a ping from Bob
      # propagates to Alice.

      token = Random.id()

      synchros = AliceMain.syncStore.synchros
      pingKey = "_ping.nogsyncbot#{rndBob}"
      sel = {owner: aliceOwner, name: 'all'}
      sel[pingKey] = {$eq: token}
      fields = {}
      fields[pingKey] = 1
      obs = synchros.find(sel, {fields}).observe({
        added: (doc) ->
          expect(peers.bobOpts.checkAccess).to.have.been.calledWith(
            null, 'nog-sync/get'
          )
          expect(peers.bobOpts.testAccess).to.have.been.calledWith(
            null, 'nog-sync/get'
          )
          expect(doc._ping?["nogsyncbot#{rndBob}"]).to.eql token
          obs.stop()
          BobMain.stopMainSynchro euid
          AliceMain.disconnectRemotes euid
          done()
      })

      peers.aliceOpts.checkAccess.reset()
      peers.aliceOpts.testAccess.reset()
      AliceMain.connectRemotes euid

      BobMain.ensureMainSynchro euid
      BobMain.startMainSynchro euid
      BobMain.pingMainSynchro euid, {token}
