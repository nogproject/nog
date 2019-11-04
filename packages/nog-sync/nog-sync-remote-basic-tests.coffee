{ createTestPeers } = require './nog-sync-peer-tests.coffee'
{ createContentFaker } = require './nog-sync-store-tests.coffee'

NULL_SHA1 = '0000000000000000000000000000000000000000'


createSynchroFaker = ->
  return {
    rnd: Random.id().toLowerCase()

    commitInitial: (opts) ->
      if opts?
        { syncStore, ownerName } = opts
        oldCommitSha = null
      else  # `commitInitial()` must have been called before.
        syncStore ?= @syncStore
        ownerName ?= @spec.synchro.owner
        oldCommitSha = @spec.commit._id

      @syncStore = syncStore

      spec =
        synchro:
          owner: ownerName
          name: 'all'
        object:
          name: 'fake object ' + Random.id()
          blob: null
          meta: {}
        tree:
          name: 'fake tree ' + Random.id()
          entries: []
          meta: {}
        commit:
          _id: oldCommitSha
          subject: 'fake commit ' + Random.id()
          message: 'Lorem ipsum dolor...'
          meta: {}
          parents: []
      { synchro } = spec
      synchro._id = syncStore.synchros.findOne(synchro)._id
      @spec = spec

      @_commit()

    commit: ->
      { object, tree } = @spec
      object.name = 'fake object ' + Random.id()
      tree.name = 'fake tree ' + Random.id()
      @_commit {
        parent: @spec.commit._id
      }

    commitInvalidBlob: ->
      { object } = @spec
      object.blob = 'bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0'
      @_commit()

    _commit: (opts) ->
      opts ?= {}
      { parent } = opts
      parent ?= null

      # Operate on the content store implementation.
      store = @syncStore._syncStore
      spec = @spec
      {synchro, object, tree, commit} = spec
      ownerName = synchro.owner
      repoName = synchro.name
      euid = null

      delete object._id
      object._id = store.createObject euid, {
        ownerName, repoName, content: object
      }

      tree.entries = [{type: 'object', sha1: object._id}]
      delete tree._id
      tree._id = store.createTree euid, {
        ownerName, repoName, content: tree
      }

      commit.tree = tree._id
      if parent?
        commit.parents = [parent]
      else
        commit.parents = []
      old = commit._id ? null
      delete commit._id
      commit._id = store.createCommit euid, {
        ownerName, repoName, content: commit
      }

      store.updateRef euid, {
        ownerName, repoName,
        refName: 'branches/master', new: commit._id, old,
      }
      synchro.master = commit._id

      return

  }


describe 'nog-sync', -> describe 'remote, basic ops', ->

  peers = null
  euid = null
  alice = null
  bob = null
  synchroFaker = null
  remote = null

  before ->
    peers = createTestPeers()
    alice = peers.AliceMain
    bob = peers.BobMain

    alice.ensureSyncUsers()
    alice.ensureMainSynchro euid

    # Connect Alice to Bob, but stop automatic propagation of changes from Bob,
    # so that the tests can explicitly call fetch.
    #
    # Patch `remote` to disable fetching content, because the `synchroFaker`
    # generates trees that are not valid repo prefix trees.

    alice.connectRemotes()
    remote = alice.remotes[peers.rndBob]
    remote.observer.stop()
    remote._fetchContentForReposSnapDiff = ->
      return { nContentCommits: 0, nContentTrees: 0, nContentObjects: 0 }

    bob.ensureSyncUsers()
    bob.ensureMainSynchro euid

    synchroFaker = createSynchroFaker()

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


  getRemoteRef = ->
    synchro = alice.syncStore.synchros.findOne({ name: 'all' })
    return synchro.refs["remotes/#{peers.rndBob}/branches/master"]

  expectLocalEntries = (entries) ->
    { commits, trees, objects } = alice.syncStore
    for ent in entries
      switch ent.type
        when 'commit'
          expect(commits.findOne(ent.sha)).to.exist
        when 'tree'
          expect(trees.findOne(ent.sha)).to.exist
        when 'object'
          expect(objects.findOne(ent.sha)).to.exist


  it 'fetches synchro and updates remote ref', ->
    synchroFaker.commitInitial {
      syncStore: bob.syncStore
      ownerName: peers.bobOwner
    }
    master = synchroFaker.spec.synchro.master
    waitForSubUpdate { master }

    remote.fetch()

    expect(getRemoteRef()).to.eql master
    { spec } = synchroFaker
    expectLocalEntries([
      { type: 'commit', sha: spec.commit._id }
      { type: 'tree', sha: spec.tree._id }
      { type: 'object', sha: spec.object._id }
    ])

  it 'fetches more synchro commits', ->
    entries = []
    pushEntries = ->
      { commit, tree, object } = synchroFaker.spec
      entries.push { type: 'commit', sha: commit._id }
      entries.push { type: 'tree', sha: tree._id }
      entries.push { type: 'object', sha: object._id }

    for i in [0...10]
      synchroFaker.commit()
      pushEntries()

    master = synchroFaker.spec.synchro.master
    waitForSubUpdate { master }

    remote.fetch()

    expect(getRemoteRef()).to.eql master
    expectLocalEntries entries

  it 'rejects blobs in synchro store', ->
    synchroFaker.commitInvalidBlob()
    master = synchroFaker.spec.synchro.master
    waitForSubUpdate { master }

    fn = -> remote.fetch()
    expect(fn).to.throw '[ERR_SYNCHRO_STATE]'


  describe 'transfer sha check', ->

    origCall = null

    beforeEach ->
      # Init before each to ensure fresh transfer in each test.
      synchroFaker.commitInitial()
      master = synchroFaker.spec.synchro.master
      waitForSubUpdate { master }
      origCall = remote.call
      remote.call = _.clone remote.call

    afterEach ->
      remote.call = origCall

    it 'detects commit transfer sha mismatch', ->
      origGet = remote.call.getSynchroCommit
      remote.call.getSynchroCommit = (args...) ->
        content = origGet args...
        content.subject = Random.id()
        return content

      fn = -> remote.fetch()
      expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'

    it 'detects tree transfer sha mismatch', ->
      origGet = remote.call.getSynchroTree
      remote.call.getSynchroTree = (args...) ->
        content = origGet args...
        content.name = Random.id()
        return content

      fn = -> remote.fetch()
      expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'

    it 'detects object transfer sha mismatch', ->
      origGet = remote.call.getSynchroObject
      remote.call.getSynchroObject = (args...) ->
        content = origGet args...
        content.name = Random.id()
        return content
      origGetEntries = remote.call.getSynchroEntries
      remote.call.getSynchroEntries = (args...) ->
        res = origGetEntries args...
        for obj in res.objects
          obj.name = Random.id()
        return res

      fn = -> remote.fetch()
      expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'
