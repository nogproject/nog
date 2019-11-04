{ createTestPeers } = require './nog-sync-peer-tests.coffee'
{ createContentFaker } = require './nog-sync-store-tests.coffee'

NULL_SHA1 = '0000000000000000000000000000000000000000'


describe 'nog-sync', ->

  peers = null
  euid = null
  aliceOwner = null
  AliceMain = null
  BobMain = null
  rndBob = null
  contentFaker = null

  before ->
    peers = createTestPeers()
    aliceOwner = peers.aliceOwner
    AliceMain = peers.AliceMain
    BobMain = peers.BobMain
    rndBob = peers.rndBob

    AliceMain.ensureSyncUsers()
    AliceMain.ensureMainSynchro euid

    BobMain.ensureSyncUsers()
    BobMain.ensureMainSynchro euid

    contentFaker = createContentFaker()

  after ->
    peers.cleanup()
    contentFaker.cleanup()


  it 'startMainSynchro() starts interval snapshots', (done) ->

    synchros = AliceMain.syncStore.synchros
    sel = {owner: aliceOwner, name: 'all'}
    fields = {}
    fields['refs.branches/master'] = 1
    obs = synchros.find(sel, {fields}).observe {
      changed: (doc) ->
        obs.stop()
        AliceMain.stopMainSynchro euid
        done()
    }

    AliceMain.startMainSynchro euid


  expectAliceFetchesBob = (done) ->
    synchros = AliceMain.syncStore.synchros
    sel = {owner: aliceOwner, name: 'all'}
    fields = {}
    refName = "remotes/#{rndBob}/branches/master"
    fields["refs.#{refName}"] = 1
    obs = synchros.find(sel, {fields}).observeChanges {
      changed: (id, doc) ->
        unless (sha = doc.refs?[refName])? and sha != NULL_SHA1
          return
        obs.stop()
        BobMain.stopMainSynchro euid
        AliceMain.disconnectRemotes euid
        done()
    }


  # Connect Alice to Bob, then start snapshots at Bob and observer whether they
  # propagate to Alice as a remote ref with Bob's peer name (not Bob's peer
  # username).

  it 'connectRemotes() starts fetching remote', (done) ->
    expectAliceFetchesBob done
    AliceMain.connectRemotes euid
    BobMain.startMainSynchro euid


  it 'remote fetches repo updates', (done) ->
    @timeout(3000)
    contentFaker.insertFakeUsers {users: Meteor.users}

    expectAliceFetchesBob ->
      # Test that method access has been checked.
      expect(peers.bobOpts.checkAccess).to.have.been.calledWith(
        null, 'nog-sync/get'
      )
      # Test that publish access has been checked.
      expect(peers.bobOpts.testAccess).to.have.been.calledWith(
        null, 'nog-sync/get'
      )
      done()

    BobMain.startMainSynchro euid
    contentFaker.createFakeContent {
      store: peers.bobOpts.contentStore
    }
    Meteor._sleepForMs 150
    contentFaker.commitFakeContent()
    Meteor._sleepForMs 150
    contentFaker.commitFakeContent()
    Meteor._sleepForMs 150

    peers.aliceOpts.checkAccess.reset()
    peers.aliceOpts.testAccess.reset()
    AliceMain.connectRemotes euid


  getAliceRemoteMaster = ->
    synchro = AliceMain.syncStore.synchros.findOne { name: 'all' }
    return synchro.refs["remotes/#{rndBob}/branches/master"]


  # A partial apply could cause inconsistent state.  Some repo masters may have
  # been updated before apply got interrupted.  If a later pull fetched and
  # merged before completing the interrupted apply, the master for the
  # already-updated repos would not match the base, causing spurious conflicts;
  # similarly for snapshots.

  it 'interrupted apply blocks fetch and snapshot', ->
    BobMain.startMainSynchro euid
    contentFaker.createFakeContent {
      store: peers.bobOpts.contentStore
    }

    # Force fake error.
    AliceMain.syncStore._testingApplyErrorProb = 1
    AliceMain.connectRemotes euid
    Meteor._sleepForMs 200

    oldRef = getAliceRemoteMaster()

    # Trigger change, which is not fetched.
    contentFaker.commitFakeContent()
    Meteor._sleepForMs 200
    expect(getAliceRemoteMaster()).to.be.eql oldRef

    # Snapshots are refused with a pending apply.
    { status } = AliceMain.syncStore.snapshot euid, {
      ownerName: aliceOwner
      synchroName: 'all'
    }
    expect(status).to.eql 'refused'

    # Clear fake error to allow recovery.
    AliceMain.syncStore._testingApplyErrorProb = 0

    # Trigger pull that skips fetch and restarts apply.
    contentFaker.commitFakeContent()
    Meteor._sleepForMs 200

    # Trigger another pull that fetches, since apply has completed.
    contentFaker.commitFakeContent()
    Meteor._sleepForMs 500
    expect(getAliceRemoteMaster()).to.be.not.eql oldRef

    AliceMain.disconnectRemotes euid
    BobMain.stopMainSynchro euid


  # NULL_SHA1s are ignored during fetch; similar to git fetch, which requires
  # an explicit prune before it deletes refs that disappeared at the remote.

  it 'remote fetch ignores remote NULL_SHA1 synchro master', ->
    oldRef = getAliceRemoteMaster()

    $set = {}
    $set['refs.branches/master'] = NULL_SHA1
    BobMain.syncStore.synchros.update { name: 'all' }, { $set }

    AliceMain.connectRemotes euid
    Meteor._sleepForMs 200

    expect(getAliceRemoteMaster()).to.be.eql oldRef

    AliceMain.disconnectRemotes euid
