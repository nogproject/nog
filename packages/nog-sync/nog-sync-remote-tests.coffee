{ createTestPeers } = require './nog-sync-peer-tests.coffee'
{ createContentFaker } = require './nog-sync-store-tests.coffee'


describe 'nog-sync', -> describe 'remote, real ops', ->

  peers = null
  euid = null
  alice = null
  bob = null
  bobSyncStore = null
  bobSyncOwner = null
  bobContentStore = null
  bobContentFaker = null
  remote = null

  before ->
    peers = createTestPeers()
    alice = peers.AliceMain
    bob = peers.BobMain
    bobSyncStore = bob.syncStore
    bobSyncOwner = peers.bobOwner
    bobContentStore = peers.bobOpts.contentStore

    alice.ensureSyncUsers()
    alice.ensureMainSynchro euid

    # Connect Alice to Bob, but stop automatic propagation of changes from Bob,
    # so that the tests can explicitly call fetch.
    #
    # By default create fake content at Bob and check that fetch transfers to
    # Alice.

    alice.connectRemotes()
    remote = alice.remotes[peers.rndBob]
    remote.observer.stop()

    bob.ensureSyncUsers()
    bob.ensureMainSynchro euid

    bobContentFaker = createContentFaker()
    bobContentFaker.insertFakeUsers { users: bobContentStore.users }

  after ->
    alice.disconnectRemotes()
    peers.cleanup()


  # `waitForSubUpdate({ master })` spins until the subscription to the remote
  # synchro's master has updated to the expected ref.

  waitForSubUpdate = (opts) ->
    { master } = opts
    getMaster = ->
      remote.remoteSynchros.findOne({ name: 'all' }).refs['branches/master']
    n = 100
    until getMaster() == master and n > 0
      n--
      Meteor._sleepForMs 5
    unless getMaster() == master
      throw new Error('Did not receive the expected update.')

  bobGetSynchroMaster = ->
    synchro = bobSyncStore.synchros.findOne({ name: 'all' })
    return synchro.refs['branches/master']

  aliceGetRemoteRef = ->
    synchro = alice.syncStore.synchros.findOne({ name: 'all' })
    return synchro.refs["remotes/#{peers.rndBob}/branches/master"]

  aliceExpectContentEntries = (entries) ->
    { commits, trees, objects, blobs } = peers.aliceOpts.contentStore
    for ent in entries
      switch ent.type
        when 'commit'
          expect(commits.findOne(ent.sha)).to.exist
        when 'tree'
          expect(trees.findOne(ent.sha)).to.exist
        when 'object'
          expect(objects.findOne(ent.sha)).to.exist
        when 'blob'
          expect(blobs.findOne(ent.sha)).to.exist

  bobSnapshot = ->
    bobSyncStore.snapshot euid, { ownerName: bobSyncOwner, synchroName: 'all' }

  it 'fetches new real content', ->
    bobContentFaker.createFakeContent { euid, store: bobContentStore }
    bobSnapshot()
    master = bobGetSynchroMaster()
    waitForSubUpdate { master }

    remote.fetch()

    expect(aliceGetRemoteRef()).to.eql master
    { spec } = bobContentFaker
    aliceExpectContentEntries([
      { type: 'commit', sha: spec.commit._id }
      { type: 'tree', sha: spec.tree._id }
      { type: 'object', sha: spec.object._id }
    ])

  it 'fetches updated real content', ->
    expected = []
    commit = ->
      bobContentFaker.commitFakeContent()
      { spec } = bobContentFaker
      expected.push { type: 'commit', sha: spec.commit._id }
      expected.push { type: 'tree', sha: spec.tree._id }
      expected.push { type: 'object', sha: spec.object._id }

    commit()
    commit()
    commit()
    bobSnapshot()
    master = bobGetSynchroMaster()
    waitForSubUpdate { master }

    remote.fetch()

    expect(aliceGetRemoteRef()).to.eql master
    aliceExpectContentEntries expected

  it 'adds placeholder blobs', ->
    bobContentFaker.commitBlob()
    bobSnapshot()
    master = bobGetSynchroMaster()
    waitForSubUpdate { master }

    remote.fetch()

    expect(aliceGetRemoteRef()).to.eql master
    { spec } = bobContentFaker
    aliceExpectContentEntries([
      { type: 'blob', sha: spec.blob._id }
    ])


  describe 'content transfer sha check', ->

    origCall = null

    beforeEach ->
      # Change content before each to ensure fresh transfer in each test.
      bobContentFaker.createFakeContent { euid, store: bobContentStore }
      bobSnapshot()
      master = bobGetSynchroMaster()
      waitForSubUpdate { master }

      origCall = remote.call
      remote.call = _.clone remote.call

    afterEach ->
      remote.call = origCall

    it 'detects content commit transfer sha mismatch', ->
      origGet = remote.call.getContentCommit
      remote.call.getContentCommit = (args...) ->
        content = origGet args...
        content.subject = Random.id()
        return content

      fn = -> remote.fetch()
      expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'

    it 'detects content tree transfer sha mismatch', ->
      origGet = remote.call.getContentTree
      remote.call.getContentTree = (args...) ->
        content = origGet args...
        content.name = Random.id()
        return content

      fn = -> remote.fetch()
      expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'

    it 'detects content object transfer sha mismatch', ->
      origGet = remote.call.getContentObject
      remote.call.getContentObject = (args...) ->
        content = origGet args...
        content.name = Random.id()
        return content
      origGetEntries = remote.call.getContentEntries
      remote.call.getContentEntries = (args...) ->
        res = origGetEntries args...
        for obj in res.objects
          obj.name = Random.id()
        return res

      fn = -> remote.fetch()
      expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'
