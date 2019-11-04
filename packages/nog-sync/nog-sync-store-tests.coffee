{ createSyncStore } = require './nog-sync-store.coffee'

{
  createContentCollections
  createContentStore
} = NogContent

NULL_SHA1 = '0000000000000000000000000000000000000000'


crypto = require('crypto')
sha1_hex = (d) -> crypto.createHash('sha1').update(d, 'utf8').digest('hex')


dropCollections = (colls) ->
  for c in colls
    dropCollection c


dropCollection = (coll) ->
  # Insert a fake document to avoid Mongo 'ns not found'.
  coll.insert {}
  coll._dropCollection()


createContentFaker = ->
  return {
    rnd: Random.id().toLowerCase()

    insertFakeUsers: (opts) ->
      {users} = opts

      # See <http://docs.meteor.com/#/full/meteor_users> for user document format.
      @fakeUserDocs =
        owner: {_id: "fakeUserId-#{@rnd}", username: "fakeUser-#{@rnd}"}

      for k, doc of @fakeUserDocs
        users.insert doc

      @users = users

    cleanup: ->
      if @users?
        n = 0
        for k, doc of @fakeUserDocs
          n += @users.remove doc
          console.log "[test] ContentFaker cleanup removed #{n} Meteor users."

    createFakeContent: (opts) ->
      {euid, store} = opts
      euid ?= null
      ownerName = @fakeUserDocs.owner.username

      spec =
        repo:
          name: 'repo-' + Random.id()
        object:
          name: 'fake object ' + Random.id()
          blob: null
          meta: {}
        tree:
          name: 'fake tree ' + Random.id()
          entries: []
          meta: {}
        commit:
          subject: 'fake commit ' + Random.id()
          message: 'Lorem ipsum dolor...'
          meta: {}
          parents: []
      {repo, object, tree, commit} = spec
      repoName = repo.name
      repo._id = store.createRepo euid, {
        repoFullName: [ownerName, repoName].join('/')
      }
      repo.owner = ownerName
      object._id = store.createObject euid, {
        ownerName, repoName, content: object
      }
      tree.entries = [{type: 'object', sha1: object._id}]
      tree._id = store.createTree euid, {
        ownerName, repoName, content: tree
      }
      commit.tree = tree._id
      commit._id = store.createCommit euid, {
        ownerName, repoName, content: commit
      }
      store.updateRef euid, {
        ownerName, repoName,
        refName: 'branches/master', new: commit._id, old: null
      }

      @store = store
      @spec = spec

    commitFakeContent: (opts) ->
      opts ?= {}
      {euid} = opts
      euid ?= null

      store = @store
      ownerName = @fakeUserDocs.owner.username
      {repo, tree, commit} = @spec
      repoName = repo.name

      tree.name = 'fake tree ' + Random.id()
      delete tree._id
      tree._id = store.createTree euid, {
        ownerName, repoName, content: tree
      }
      commit.tree = tree._id
      oldCommitId = commit._id
      delete commit._id
      commit.parents = [oldCommitId]
      commit._id = store.createCommit euid, {
        ownerName, repoName, content: commit
      }
      store.updateRef euid, {
        ownerName, repoName,
        refName: 'branches/master', new: commit._id, old: oldCommitId
      }

    amendFakeContent: ->
      euid = null

      store = @store
      ownerName = @fakeUserDocs.owner.username
      {repo, tree, commit} = @spec
      repoName = repo.name

      tree.name = 'fake tree ' + Random.id()
      delete tree._id
      tree._id = store.createTree euid, {
        ownerName, repoName, content: tree
      }
      commit.tree = tree._id
      oldCommitId = commit._id
      delete commit._id
      commit._id = store.createCommit euid, {
        ownerName, repoName, content: commit
      }
      store.updateRef euid, {
        ownerName, repoName,
        refName: 'branches/master', new: commit._id, old: oldCommitId
      }

    commitBlob: ->
      euid = null

      store = @store
      ownerName = @fakeUserDocs.owner.username

      {repo, object, tree, commit} = @spec
      repoName = repo.name

      blobSha = sha1_hex(Random.id())
      blob = {
        _id: blobSha,
        sha1: blobSha,
        status: 'available',
        size: 999
      }
      store.blobs.insert blob
      @spec.blob = blob

      object.blob = blobSha
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
      oldCommitId = commit._id
      delete commit._id
      commit._id = store.createCommit euid, {
        ownerName, repoName, content: commit
      }
      store.updateRef euid, {
        ownerName, repoName,
        refName: 'branches/master', new: commit._id, old: oldCommitId
      }

  }


describe 'nog-sync', ->

  rnd = null
  contentFaker = null
  contentStore = null
  syncStoreOpts = null
  syncStore = null
  users = null
  fakeUserDocs = null
  ourPeerName = null
  checkAccess = sinon.spy()
  testAccess = sinon.spy()

  before ->
    rnd = 'test' + Random.id().toLowerCase()
    ourPeerName = 'us-' + Random.id()

    contentFaker = createContentFaker()

    users = new Mongo.Collection "#{rnd}.users"
    contentFaker.insertFakeUsers {users}
    fakeUserDocs = contentFaker.fakeUserDocs

    contentColls = createContentCollections {
      namespace: {coll: "#{rnd}_content"}
    }
    contentStoreOpts = _.extend contentColls, {
      users: users
      blobs: null
      reposSets: null
      checkAccess
    }
    contentStore = createContentStore contentStoreOpts

    syncStoreOpts = {
      namespace: {coll: "#{rnd}_sync"}
      ourPeerName
      contentStore
      users: users
      checkAccess
      testAccess
      caching: {
        maxNElements: 10
        maxAge_s: 100
      }
    }
    syncStore = createSyncStore syncStoreOpts

  after ->
    dropCollections _.values _.pick(
      contentStore, 'repos', 'commits', 'trees', 'objects'
    )
    dropCollections _.values _.pick(
      syncStore, 'synchros', 'commits', 'trees', 'objects'
    )
    dropCollection users

  describe 'SyncStore', ->

    euid = null
    ownerName = null
    synchroName = null
    contentSpec = null

    before ->
      ownerName = fakeUserDocs.owner.username
      synchroName = 'synchro-' + Random.id()
      contentFaker.createFakeContent {
        euid
        store: contentStore
      }
      contentSpec = contentFaker.spec

    findSynchro = ->
      return syncStore.synchros.findOne {
        owner: ownerName
        name: synchroName
      }

    it 'can be instantiated', ->
      expect(syncStore.synchros).to.exist
      expect(syncStore.commits).to.exist
      expect(syncStore.trees).to.exist
      expect(syncStore.objects).to.exist
      expect(syncStore.contentStore).to.exist

    it 'access checks', ->

      repeat20 = (x) -> Array(21).join x

      get = 'nog-sync/get'
      modify = 'nog-sync/modify'
      create = 'nog-sync/create'
      specs = [
        {
          fn: 'getRefs',
          action: get,
          opts: {ownerName: Random.id(), synchroName: Random.id()},
        }
        {
          fn: 'getPing',
          action: get,
          opts: {owner: Random.id()},
        }
        {
          fn: 'getCommitRaw',
          action: get,
          opts: {sha: repeat20('fe')},
        }
        {
          fn: 'getTreeRaw',
          action: get,
          opts: {sha: repeat20('f0')},
        }
        {
          fn: 'getObjectRaw',
          action: get,
          opts: {sha: repeat20('a3')},
        }
        {
          fn: 'getContentCommitRaw',
          action: get,
          opts: {sha: repeat20('12')},
        }
        {
          fn: 'getContentTreeRaw',
          action: get,
          opts: {sha: repeat20('13')},
        }
        {
          fn: 'getContentObjectRaw',
          action: get,
          opts: {sha: repeat20('14')},
        }

        {
          fn: 'updateRef',
          action: modify,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
            refName: 'branches/foo',
            old: null,
            new: null,
          },
        }
        {
          fn: 'pingSynchro',
          action: modify,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
            token: Random.id(),
          },
        }
        {
          fn: 'snapshot',
          action: modify,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
          },
        }
        {
          fn: 'fullSnapshot',
          action: modify,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
          },
        }
        {
          fn: 'createTree',
          action: modify,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
            content: { name: 'a', meta: {}, entries: [] },
          }
        }
        {
          fn: 'createCommit',
          action: modify,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
            content: {
              subject: Random.id(), message: '', parents: [],
              tree: repeat20('8f'),
            },
          }
        }

        {
          fn: 'ensureSynchro',
          action: create,
          opts: {
            ownerName: Random.id(),
            synchroName: Random.id(),
          }
        }

      ]

      for spec in specs
        {fn, opts, action} = spec
        fakeEuid = Random.id()
        checkAccess.reset()
        try
          syncStore[fn] fakeEuid, opts
        catch err
          # Ignore expected error, we are only interested in the check.
          unless (
            err.errorCode in [
              'ERR_SYNCHRO_CONTENT_MISSING',
              'ERR_CONTENT_MISSING',
              'ERR_SYNCHRO_MISSING',
              'ERR_REPO_MISSING',
              'ERR_UNKNOWN_USERNAME',
            ]
          )
            throw err
        expect(checkAccess).to.have.been.calledWith(
          fakeEuid, action, opts
        )

    it 'ensureSynchro() requires existing owner.', ->
      fn = ->
        syncStore.ensureSynchro euid, {
          synchroName
          ownerName: Random.id()
        }
      expect(fn).to.throw '[ERR_UNKNOWN_USERNAME]'

    it 'ensureSynchro() creates a synchro.', ->
      selector = {repos: {owner: 'foobar'}}
      syncStore.ensureSynchro euid, {
        ownerName, synchroName, selector
      }
      synchro = syncStore.synchros.findOne {
        owner: ownerName
        name: synchroName
      }
      expect(synchro).to.exist
      expect(synchro.owner).to.equal ownerName
      expect(synchro.name).to.equal synchroName
      expect(synchro.selector.repos).to.deep.equal selector.repos

    it 'ensureSynchro() updates a synchro.', ->
      selector = {repos: {}}
      syncStore.ensureSynchro euid, {
        ownerName, synchroName, selector
      }
      synchro = syncStore.synchros.findOne {
        owner: ownerName
        name: synchroName
      }
      expect(synchro.selector.repos).to.deep.equal {}

    it 'setOp(prevOp: null)', ->
      ok = syncStore.setOp { ownerName, synchroName, op: 'FROM_NULL' }
      expect(ok).to.be.true
      op = syncStore.getOp { ownerName, synchroName }
      expect(op.op).to.eql 'FROM_NULL'
      ok = syncStore.setOp {
        ownerName, synchroName, prevOp: 'invalid', op: 'x',
      }
      expect(ok).to.be.false
      op = syncStore.getOp { ownerName, synchroName }
      expect(op.op).to.eql 'FROM_NULL'

    it 'setOp(prevOp: *)', ->
      ok = syncStore.setOp {
        ownerName, synchroName, prevOp: '*', op: 'FROM_STAR1',
      }
      expect(ok).to.be.true
      op = syncStore.getOp { ownerName, synchroName }
      expect(op.op).to.eql 'FROM_STAR1'
      ok = syncStore.setOp {
        ownerName, synchroName, prevOp: '*', op: 'FROM_STAR2',
      }
      expect(ok).to.be.true
      op = syncStore.getOp { ownerName, synchroName }
      expect(op.op).to.eql 'FROM_STAR2'

    it 'setOp(prevOp: op)', ->
      ok = syncStore.setOp {
        ownerName, synchroName, prevOp: 'FROM_STAR2', op: 'FROM_PREV',
      }
      expect(ok).to.be.true
      op = syncStore.getOp { ownerName, synchroName }
      expect(op.op).to.eql 'FROM_PREV'

    it 'clearOp(prevOp: *)', ->
      syncStore.setOp { ownerName, synchroName, prevOp: '*', op: 'OP' }
      ok = syncStore.clearOp { ownerName, synchroName, prevOp: '*' }
      expect(ok).to.be.true
      op = syncStore.getOp { ownerName, synchroName }
      expect(op).to.not.exist

    it 'clearOp(prevOp: op)', ->
      syncStore.setOp { ownerName, synchroName, prevOp: '*', op: 'OP' }
      ok = syncStore.clearOp { ownerName, synchroName, prevOp: 'invalid' }
      expect(ok).to.be.false
      op = syncStore.getOp { ownerName, synchroName }
      expect(op.op).to.eql 'OP'
      ok = syncStore.clearOp { ownerName, synchroName, prevOp: 'OP' }
      expect(ok).to.be.true
      op = syncStore.getOp { ownerName, synchroName }
      expect(op).to.not.exist

    it 'snapshot() throws if the synchro is missing.', ->
      fn = -> syncStore.snapshot euid, {ownerName, synchroName: 'unknown'}
      expect(fn).to.throw '[ERR_SYNCHRO_MISSING]'

    it 'snapshot() creates initial sync snapshot.', ->
      refName = 'branches/master'
      syn = findSynchro()
      expect(syn.refs[refName]).to.eql NULL_SHA1

      syncStore.snapshot euid, {ownerName, synchroName}

      syn = findSynchro()
      expect(syn.refs[refName]).to.not.eql NULL_SHA1

    it 'snapshot() keeps sync commit when already up-to-date.', ->
      refName = 'branches/master'
      syn = findSynchro()

      syncStore.snapshot euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[refName]).to.eql syn.refs[refName]

    it 'snapshot() creates new sync commit when a content repo changes', ->
      contentFaker.commitFakeContent()
      refName = 'branches/master'
      syn = findSynchro()

      syncStore.snapshot euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[refName]).to.not.eql syn.refs[refName]


    getReposRootTree = ->
      refName = 'branches/master'
      syn = findSynchro()
      commitSha = syn.refs[refName]
      commit = syncStore.commits.findOne commitSha
      rootTree = syncStore.trees.findOne commit.tree
      return rootTree

    getReposTreeLeaf = ->
      tree = getReposRootTree()
      tree = syncStore.trees.findOne tree.entries[0].sha1  # -> repos
      tree = syncStore.trees.findOne tree.entries[0].sha1  # -> level 'ba'
      tree = syncStore.trees.findOne tree.entries[0].sha1  # -> level 'bafe'
      tree = syncStore.trees.findOne tree.entries[0].sha1  # -> leaf
      return tree

    it 'repos root tree contains root entries: repos', ->
      rootTree = getReposRootTree()
      expect(rootTree.entries).to.have.length 1

      ent = rootTree.entries[0]
      expect(ent.type).to.eql 'tree'
      tree = syncStore.trees.findOne ent.sha1
      expect(tree.name).to.eql 'repos'

    it '
      repos tree contains content repos snapshot 2x2 prefix tree.
    ', ->
      rootTree = getReposRootTree()
      tree = syncStore.trees.findOne rootTree.entries[0].sha1
      repo = contentSpec.repo
      leafName = ['repo', repo.owner, repo.name].join(':')
      shaid = sha1_hex(leafName)
      for level in [0..1]
        ent = tree.entries[0]
        expect(ent.type).to.eql 'tree'
        tree = syncStore.trees.findOne ent.sha1
        name = 'repos:' + shaid[0..(level * 2 + 1)]
        expect(tree.name).to.eql name
      leaf = getReposTreeLeaf()
      expect(leaf.name).to.eql leafName

    it 'repos tree leaf contains content repo info', ->
      repo = contentSpec.repo
      leaf = getReposTreeLeaf()
      leafName = ['repo', repo.owner, repo.name].join(':')
      expect(leaf.name).to.eql leafName
      expect(leaf.meta.nog).to.deep.eql {
        name: repo.name
        owner: repo.owner
        refs: {
          'branches/master': contentSpec.commit._id
        }
        conflicts: {}
      }

    addContentRef = (opts) ->
      {refName} = opts
      repoName = contentSpec.repo.name
      commitSha = contentSpec.commit._id
      contentStore.repos.update {
        owner: ownerName, name: repoName
      }, {
        $set: { "refs.#{refName}": commitSha }
        $currentDate: { mtime: true }
      }

    setNullContentRef = (opts) ->
      {refName} = opts
      repoName = contentSpec.repo.name
      contentStore.repos.update {
        owner: ownerName, name: repoName
      }, {
        $set: { "refs.#{refName}": NULL_SHA1 }
        $currentDate: { mtime: true }
      }

    deleteContentRef = (opts) ->
      {refName} = opts
      repoName = contentSpec.repo.name
      contentStore.repos.update {
        owner: ownerName, name: repoName
      }, {
        $unset: { "refs.#{refName}": '' }
        $currentDate: { mtime: true }
      }

    deleteContentConflict = (opts) ->
      {refName} = opts
      repoName = contentSpec.repo.name
      contentStore.repos.update {
        owner: ownerName, name: repoName
      }, {
        $unset: { "conflicts.#{refName}": '' }
        $currentDate: { mtime: true }
      }

    it 'snapshot() ignores refs other than master', ->
      synRefName = 'branches/master'
      syn = findSynchro()

      addContentRef {refName: 'branches/other'}
      syncStore.snapshot euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[synRefName]).to.eql syn.refs[synRefName]


    setContentRef = (opts) ->
      {refName, sha} = opts
      repoName = contentSpec.repo.name
      commitSha = contentSpec.commit._id
      contentStore.repos.update {
        owner: ownerName, name: repoName
      }, {
        $set: { "refs.#{refName}": sha }
        $currentDate: { mtime: true }
      }

    setContentConflict = (opts) ->
      {refName, shas} = opts
      repoName = contentSpec.repo.name
      commitSha = contentSpec.commit._id
      contentStore.repos.update {
        owner: ownerName, name: repoName
      }, {
        $set: { "conflicts.#{refName}": shas }
        $currentDate: { mtime: true }
      }

    sha111 = '1111111111111111111111111111111111111111'
    sha222 = '2222222222222222222222222222222222222222'

    it 'snapshotAnonC() stores simple master', ->
      synRefName = 'branches/master'
      syn = findSynchro()

      setContentRef {
        refName: "branches/master",
        sha: sha222
      }
      deleteContentConflict { refName: 'branches/master' }
      syncStore.snapshotAnonC euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[synRefName]).to.not.eql syn.refs[synRefName]

      leaf = getReposTreeLeaf()
      commitSha = contentSpec.commit._id
      expect(leaf.meta.nog.refs).to.deep.eql {
        "branches/master": sha222
      }

    it 'snapshotAnonC() stores nothing for missing master', ->
      synRefName = 'branches/master'
      syn = findSynchro()

      deleteContentRef { refName: 'branches/master' }
      deleteContentConflict { refName: 'branches/master' }
      syncStore.snapshotAnonC euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[synRefName]).to.not.eql syn.refs[synRefName]

      leaf = getReposTreeLeaf()
      commitSha = contentSpec.commit._id
      expect(leaf.meta.nog.refs).to.deep.eql {}
      expect(leaf.meta.nog.conflicts).to.deep.eql {}

    it 'snapshotAnonC() stores conflicts as sorted array', ->
      synRefName = 'branches/master'
      syn = findSynchro()

      setContentRef {
        refName: "branches/master",
        sha: sha222
      }
      setContentConflict {
        refName: "branches/master",
        shas: [sha111]
      }
      syncStore.snapshotAnonC euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[synRefName]).to.not.eql syn.refs[synRefName]

      leaf = getReposTreeLeaf()
      commitSha = contentSpec.commit._id
      expect(leaf.meta.nog.refs).to.deep.eql {}
      expect(leaf.meta.nog.conflicts).to.deep.eql {
        "branches/master": [sha111, sha222]
      }

    it 'snapshotAnonC() stores null sha for missing master with conflicts', ->
      synRefName = 'branches/master'
      syn = findSynchro()

      deleteContentRef { refName: "branches/master" }
      setContentConflict {
        refName: 'branches/master',
        shas: [sha111]
      }
      syncStore.snapshotAnonC euid, {ownerName, synchroName}

      syn2 = findSynchro()
      expect(syn2.refs[synRefName]).to.not.eql syn.refs[synRefName]

      leaf = getReposTreeLeaf()
      commitSha = contentSpec.commit._id
      expect(leaf.meta.nog.refs).to.deep.eql {}
      expect(leaf.meta.nog.conflicts).to.deep.eql {
        "branches/master": [NULL_SHA1, sha111]
      }


module.exports.createContentFaker = createContentFaker
