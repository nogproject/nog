{
  createContentCollections
  createContentStore
} = NogContent

{
  synchroTreeReposDiffStream,
  synchroTreeReposDiff3Stream,
} = require './nog-sync-diff.coffee'

{ createContentFaker } = require './nog-sync-store-tests.coffee'


createTestContentStore = ->
  rnd = Random.id()
  users = new Mongo.Collection "#{rnd}.users"
  contentColls = createContentCollections {
    namespace: {coll: "#{rnd}_content"}
  }
  contentStoreOpts = _.extend contentColls, {
    users
    blobs: null
    reposSets: null
    checkAccess: ->
  }
  contentStore = createContentStore contentStoreOpts

  contentStore.dropCollections = ->
    for c in ['repos', 'commits', 'trees', 'objects', 'users']
      dropCollection this[c]

  return contentStore


dropCollection = (coll) ->
  # Insert a fake document to avoid Mongo 'ns not found'.
  coll.insert {}
  coll._dropCollection()


createPrefixTreeFaker = () ->
  contentFaker = createContentFaker()
  return {
    contentFaker
    ownerName: null
    repoName: null

    createRepo:  (opts) ->
      { contentStore } = opts
      euid = null
      @contentFaker.insertFakeUsers { users: contentStore.users }
      @contentFaker.createFakeContent { euid, store: contentStore }
      @ownerName = @contentFaker.fakeUserDocs.owner.username
      @repoName = @contentFaker.spec.repo.name

    insertBaseTree: (opts) ->
      { contentStore } = opts
      content = {
        name: 'repos'
        meta: {}
        entries: [
          {
            name: 'repos:aa'
            meta: {}
            entries: [
              {
                name: 'repos:aaaa'
                meta: {}
                entries: [
                  {
                    name: 'repo:alice:foo-' + Random.id()
                    meta: {}
                    entries: []
                  }
                ]
              }
            ]
          }
        ]
      }
      @content = content
      sha = @_createTree contentStore
      return sha

    insertThreeLeafTree: (opts) ->
      { contentStore } = opts
      content = {
        name: 'repos'
        meta: {}
        entries: [
          {
            name: 'repos:55'
            meta: {}
            entries: [
              {
                name: 'repos:5555'
                meta: {}
                entries: [
                  {
                    name: 'repo:alice:foo-' + Random.id()
                    meta: {}
                    entries: []
                  }
                ]
              }
              {
                name: 'repos:55aa'
                meta: {}
                entries: [
                  {
                    name: 'repo:alice:foo-' + Random.id()
                    meta: {}
                    entries: []
                  }
                ]
              }
            ]
          }
          {
            name: 'repos:aa'
            meta: {}
            entries: [
              {
                name: 'repos:aaaa'
                meta: {}
                entries: [
                  {
                    name: 'repo:alice:foo-' + Random.id()
                    meta: {}
                    entries: []
                  }
                ]
              }
            ]
          }
        ]
      }
      @content = content
      sha = @_createTree contentStore
      return sha

    changeLeaf: (opts) ->
      { contentStore } = opts
      leaf = @content.entries[0].entries[0].entries[0]
      leaf.meta = _.extend {}, leaf.meta, {rnd: Random.id()}
      sha = @_createTree contentStore
      return sha

    addLeaf: (opts) ->
      { contentStore, token } = opts
      @content.entries[0].entries[0].entries.push {
        name: 'repo:bob:bar-' + Random.id()
        meta: {}
        entries: []
      }
      sha = @_createTree contentStore
      return sha

    getLeaf: (i) ->
      i ?= 0
      _.clone @content.entries[0].entries[0].entries[i]

    insertInvalidTree: (opts) ->
      { contentStore } = opts
      content = {
        name: 'repos'
        meta: {}
        entries: [
          {
            name: 'invalid:aa'
            meta: {}
            entries: []
          }
        ]
      }
      @content = content
      sha = @_createTree contentStore
      return sha

    _createTree: (contentStore) ->
      ownerName = @ownerName
      repoName = @repoName
      content = @content
      euid = null
      sha = contentStore.createTree euid, {ownerName, repoName, content}

  }


describe 'nog-sync', -> describe 'diff', ->

  contentStore = null
  store = null
  faker = null

  before ->
    contentStore = createTestContentStore()
    store = {
      getTree: (sha) -> contentStore.trees.findOne(sha)
    }
    faker = createPrefixTreeFaker()
    faker.createRepo { contentStore }

  after ->
    contentStore.dropCollections()


  onadded = sinon.spy()
  ondeleted = sinon.spy()
  onmodified = sinon.spy()

  resetSpies = ->
    onadded.reset()
    ondeleted.reset()
    onmodified.reset()

  beforeEach ->
    resetSpies()


  it 'synchroTreeReposDiffStream() reports modified', ->
    aSha = faker.insertBaseTree { contentStore }
    aLeaf = faker.getLeaf()
    bSha = faker.changeLeaf { contentStore }
    bLeaf = faker.getLeaf()

    synchroTreeReposDiffStream {
      aSha, bSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(0)
    expect(onmodified).to.have.callCount(1)
    expect(onmodified).to.have.been.calledWith {
      a: sinon.match(aLeaf),
      b: sinon.match(bLeaf),
    }
    expect(ondeleted).to.have.callCount(0)

  it 'synchroTreeReposDiffStream() reports added', ->
    aSha = faker.insertBaseTree { contentStore }
    aLeaf = faker.getLeaf(0)
    bSha = faker.addLeaf { contentStore }
    bLeaf = faker.getLeaf(1)

    synchroTreeReposDiffStream {
      aSha, bSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(1)
    expect(onadded).to.have.been.calledWith {
      b: sinon.match(bLeaf),
    }
    expect(onmodified).to.have.callCount(0)
    expect(ondeleted).to.have.callCount(0)

  it 'synchroTreeReposDiffStream() reports removed', ->
    # a b switched.
    bSha = faker.insertBaseTree { contentStore }
    bLeaf = faker.getLeaf(0)
    aSha = faker.addLeaf { contentStore }
    aLeaf = faker.getLeaf(1)

    synchroTreeReposDiffStream {
      aSha, bSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(0)
    expect(onmodified).to.have.callCount(0)
    expect(ondeleted).to.have.callCount(1)
    expect(ondeleted).to.have.been.calledWith {
      a: sinon.match(aLeaf),
    }

  it 'synchroTreeReposDiffStream() reports multiple changes', ->
    aSha = faker.insertBaseTree { contentStore }
    faker.changeLeaf { contentStore }
    faker.addLeaf { contentStore }
    faker.addLeaf { contentStore }
    bSha = faker.addLeaf { contentStore }

    synchroTreeReposDiffStream {
      aSha, bSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(3)
    expect(onmodified).to.have.callCount(1)
    expect(ondeleted).to.have.callCount(0)

    # a b switched.
    resetSpies()
    synchroTreeReposDiffStream {
      aSha: bSha, bSha: aSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(0)
    expect(onmodified).to.have.callCount(1)
    expect(ondeleted).to.have.callCount(3)

  it 'synchroTreeReposDiffStream() handles fanout', ->
    aSha = faker.insertBaseTree { contentStore }
    aLeaf = faker.getLeaf()
    bSha = faker.insertThreeLeafTree { contentStore }
    bLeaf = faker.getLeaf()

    synchroTreeReposDiffStream {
      aSha, bSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(3)
    expect(onadded).to.have.been.calledWith {
      b: sinon.match(bLeaf),
    }
    expect(onmodified).to.have.callCount(0)
    expect(ondeleted).to.have.callCount(1)
    expect(ondeleted).to.have.been.calledWith {
      a: sinon.match(aLeaf),
    }

    # a b switched.
    resetSpies()
    synchroTreeReposDiffStream {
      aSha: bSha, bSha: aSha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(1)
    expect(onadded).to.have.been.calledWith {
      b: sinon.match(aLeaf),
    }
    expect(onmodified).to.have.callCount(0)
    expect(ondeleted).to.have.callCount(3)
    expect(ondeleted).to.have.been.calledWith {
      a: sinon.match(bLeaf),
    }

  it 'synchroTreeReposDiffStream() handles null sha', ->
    sha = faker.insertThreeLeafTree { contentStore }
    leaf = faker.getLeaf()

    synchroTreeReposDiffStream {
      aSha: null, bSha: sha, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(3)
    expect(onadded).to.have.been.calledWith {
      b: sinon.match(leaf),
    }
    expect(onmodified).to.have.callCount(0)
    expect(ondeleted).to.have.callCount(0)

    # a b switched.
    resetSpies()
    synchroTreeReposDiffStream {
      aSha: sha, bSha: null, store, onadded, ondeleted, onmodified
    }
    expect(onadded).to.have.callCount(0)
    expect(onmodified).to.have.callCount(0)
    expect(ondeleted).to.have.callCount(3)
    expect(ondeleted).to.have.been.calledWith {
      a: sinon.match(leaf),
    }

  it 'synchroTreeReposDiffStream() throws on invalid tree', ->
    aSha = faker.insertBaseTree { contentStore }
    bSha = faker.insertInvalidTree { contentStore }
    fn = -> synchroTreeReposDiffStream {
      aSha, bSha, store, onadded, ondeleted, onmodified
    }
    expect(fn).to.throw '[ERR_PARAM_INVALID]'


describe 'nog-sync', -> describe 'diff3', ->

  contentStore = null
  store = null
  faker = null

  before ->
    contentStore = createTestContentStore()
    store = {
      getTree: (sha) -> contentStore.trees.findOne(sha)
    }
    faker = createPrefixTreeFaker()
    faker.createRepo { contentStore }

  after ->
    contentStore.dropCollections()


  onchanged = sinon.spy()

  resetSpies = ->
    onchanged.reset()

  beforeEach ->
    resetSpies()


  it 'synchroTreeReposDiffStream() reports modified', ->
    baseSha = faker.insertBaseTree { contentStore }
    baseLeaf = faker.getLeaf()
    sha = faker.changeLeaf { contentStore }
    leaf = faker.getLeaf()

    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: baseSha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      a: sinon.match(leaf),
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha, aSha: baseSha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      b: sinon.match(leaf),
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      a: sinon.match(leaf),
      b: sinon.match(leaf),
    }

  it 'synchroTreeReposDiffStream() reports added', ->
    baseSha = faker.insertBaseTree { contentStore }
    baseLeaf = faker.getLeaf(0)
    sha = faker.addLeaf { contentStore }
    leaf = faker.getLeaf(1)

    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: baseSha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      a: sinon.match(leaf),
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha, aSha: baseSha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      b: sinon.match(leaf),
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      a: sinon.match(leaf),
      b: sinon.match(leaf),
    }


  it 'synchroTreeReposDiffStream() reports deleted', ->
    # Roles reversed.
    sha = faker.insertBaseTree { contentStore }
    leaf = faker.getLeaf(0)
    baseSha = faker.addLeaf { contentStore }
    baseLeaf = faker.getLeaf(1)

    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: baseSha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      a: 'D',
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha, aSha: baseSha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      b: 'D',
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      a: 'D',
      b: 'D',
    }

  it 'synchroTreeReposDiffStream() handles null sha', ->
    sha = faker.insertBaseTree { contentStore }
    leaf = faker.getLeaf(0)

    synchroTreeReposDiff3Stream {
      baseSha: null, aSha: sha, bSha: null, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      a: sinon.match(leaf),
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha: null, aSha: sha, bSha: sha, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      a: sinon.match(leaf),
      b: sinon.match(leaf),
    }

    resetSpies()
    synchroTreeReposDiff3Stream {
      baseSha: sha, aSha: null, bSha: null, store, onchanged
    }
    expect(onchanged).to.have.callCount(1)
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(leaf),
      a: 'D',
      b: 'D',
    }

  it 'synchroTreeReposDiff3Stream() handles fanout', ->
    baseSha = faker.insertBaseTree { contentStore }
    baseLeaf = faker.getLeaf()
    sha = faker.insertThreeLeafTree { contentStore }
    leaf = faker.getLeaf()

    synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: sha, store, onchanged
    }

    expect(onchanged).to.have.callCount(4)
    # `leaf` has been added.
    expect(onchanged).to.have.been.calledWith {
      a: sinon.match(leaf),
      b: sinon.match(leaf),
    }

    # `baseLeaf` has been deleted.
    expect(onchanged).to.have.been.calledWith {
      base: sinon.match(baseLeaf),
      a: 'D',
      b: 'D',
    }
    return

  it 'synchroTreeReposDiff3Stream() throws on invalid tree', ->
    baseSha = faker.insertBaseTree { contentStore }
    sha = faker.insertInvalidTree { contentStore }

    fn = -> synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: baseSha, store, onchanged
    }
    expect(fn).to.throw '[ERR_PARAM_INVALID]'

    fn = -> synchroTreeReposDiff3Stream {
      baseSha, aSha: baseSha, bSha: sha, store, onchanged
    }
    expect(fn).to.throw '[ERR_PARAM_INVALID]'

    fn = -> synchroTreeReposDiff3Stream {
      baseSha, aSha: sha, bSha: sha, store, onchanged
    }
    expect(fn).to.throw '[ERR_PARAM_INVALID]'
