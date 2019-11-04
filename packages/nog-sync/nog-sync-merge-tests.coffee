{ mergeSynchro } = require './nog-sync-merge.coffee'

{ mergeSynchroAnonC } = require './nog-sync-merge-anonc.js'
{ createTestPeers } = require './nog-sync-peer-tests.coffee'
{ createContentFaker } = require './nog-sync-store-tests.coffee'

NULL_SHA1 = '0000000000000000000000000000000000000000'


describe 'nog-sync', -> describe 'mergeSynchro()', ->

  # The merge tests runs locally at Alice, simulating a remote by using a
  # second synchro.

  euid = null
  peers = null
  syncOwner = null
  syncStore = null
  contentStore = null
  contentFaker = null
  remoteName = null
  ourPeerName = null

  before ->
    peers = createTestPeers()
    alice = peers.AliceMain
    syncOwner = peers.aliceOwner
    syncStore = alice.syncStore
    contentStore = peers.aliceOpts.contentStore
    ourPeerName = peers.rndAlice
    remoteName = peers.rndBob

    alice.ensureSyncUsers()
    alice.ensureMainSynchro euid

    contentFaker = createContentFaker()
    contentFaker.insertFakeUsers { users: contentStore.users }

  after ->
    peers.cleanup()

  snapshot = ->
    syncStore.snapshot euid, { ownerName: syncOwner, synchroName: 'all' }
    return getMaster()

  getMaster = ->
    synchro = syncStore.synchros.findOne { name: 'all' }
    return synchro.refs['branches/master']

  setMaster = (sha) ->
    $set = {}
    $set["refs.branches/master"] = sha
    syncStore.synchros.update { name: 'all' }, { $set }

  setRemote = (sha) ->
    $set = {}
    $set["refs.remotes/#{remoteName}/branches/master"] = sha
    syncStore.synchros.update { name: 'all' }, { $set }

  merge = ->
    return mergeSynchro euid, {
      syncStore,
      ownerName: syncOwner,
      synchroName: 'all',
      branch: 'master',
      remoteName,
    }

  mergeAnonC = ->
    return mergeSynchroAnonC euid, {
      syncStore,
      ownerName: syncOwner,
      synchroName: 'all',
      branch: 'master',
      remoteName,
    }

  it 'handles missing our branch', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    sha = snapshot()
    setMaster(NULL_SHA1)
    setRemote(sha)
    expect(merge).to.throw '[ERR_REF_NOT_FOUND]'


  it 'handles missing remote branch', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    sha = snapshot()
    setRemote(NULL_SHA1)
    expect(merge).to.throw '[ERR_REF_NOT_FOUND]'


  it 'detects already up-to-date, identical commit', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    sha = snapshot()
    setRemote(sha)

    { status, commitSha } = merge()

    expect(status).to.eql 'up-to-date'
    expect(commitSha).to.eql sha
    expect(getMaster()).to.eql sha


  it 'detects already up-to-date, ancestor', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    sha = snapshot()
    setRemote(sha)
    contentFaker.commitFakeContent()
    sha = snapshot()

    { status, commitSha } = merge()

    expect(status).to.eql 'up-to-date'
    expect(commitSha).to.eql sha
    expect(getMaster()).to.eql sha


  it 'fast-forward', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    base = snapshot()
    contentFaker.commitFakeContent()
    sha = snapshot()
    setRemote(sha)
    setMaster(base)

    { status, commitSha } = merge()

    expect(status).to.eql 'fast-forward'
    expect(commitSha).to.eql sha
    expect(getMaster()).to.eql sha


  setContentMaster = (opts) ->
    { repoName, commitSha } = opts
    $set = {}
    $set['refs.branches/master'] = commitSha
    contentStore.repos.update {
      name: repoName
    }, {
      $set,
      $currentDate: { mtime: true }
    }

  getSnapshotLeaves = ->
    { commits, trees } = syncStore
    master = getMaster()
    commit = commits.findOne(master)
    root = trees.findOne(commit.tree)
    pfxTree = trees.findOne(root.entries[0].sha1)
    leaves = {}
    for ent in pfxTree.entries
      for ent1 in trees.findOne(ent.sha1).entries
        for ent2 in trees.findOne(ent1.sha1).entries
          leaf = trees.findOne(ent2.sha1)
          name = leaf.name.split(':')[2]
          refs = leaf.meta.nog.refs
          conflicts = leaf.meta.nog.conflicts
          leaves[name] = {name, refs, conflicts}
    return leaves

  it 'proper merge, no conflicts', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    base = snapshot()
    baseContentSha = contentFaker.spec.commit._id
    contentFaker.commitFakeContent()
    repoName = contentFaker.spec.repo.name
    repoMaster = contentFaker.spec.commit._id
    remote = snapshot()

    setMaster(base)
    setContentMaster({ repoName, commitSha: baseContentSha })

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName2 = contentFaker.spec.repo.name
    repoMaster2 = contentFaker.spec.commit._id
    local = snapshot()
    setRemote(remote)

    { status, commitSha } = merge()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.equal({
      'branches/master': repoMaster
    })
    expect(leaves[repoName].conflicts).to.deep.equal {}
    expect(leaves[repoName2].refs).to.deep.equal({
      'branches/master': repoMaster2
    })
    expect(leaves[repoName2].conflicts).to.deep.equal {}
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)


  removeContentRepos = (sel) ->
    contentStore.repos.find(sel).forEach (repo) ->
      repo._id = "#{repo._id}-#{Random.id()}"
      contentStore.deletedRepos.insert(repo)
      contentStore.deletedRepos.update(
        repo._id, { $currentDate: { mtime: true } }
      )
    contentStore.repos.remove(sel)


  it 'merge unrelated, no conflicts', ->
    removeContentRepos({})

    contentFaker.createFakeContent { euid, store: contentStore }
    setMaster(NULL_SHA1)
    remote = snapshot()
    repoName = contentFaker.spec.repo.name
    repoMaster = contentFaker.spec.commit._id

    removeContentRepos({})

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName2 = contentFaker.spec.repo.name
    repoMaster2 = contentFaker.spec.commit._id
    setMaster(NULL_SHA1)
    local = snapshot()
    setRemote(remote)

    { status, commitSha } = merge()

    expect(status).to.eql 'merge-unrelated'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.equal({
      'branches/master': repoMaster
    })
    expect(leaves[repoName].conflicts).to.deep.equal {}
    expect(leaves[repoName2].refs).to.deep.equal({
      'branches/master': repoMaster2
    })
    expect(leaves[repoName2].conflicts).to.deep.equal {}
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)


  # Tests for conflicting repo changes: modified (M), added (A), deleted (D):
  #
  #  - A a A b, a != b: true conflict
  #  - A a A b, a == b: trivial resolution
  #  - D a D b: always trivial resolution
  #  - M a M b, a != b: true conflict
  #  - M a M b, a == b: trivial resolution
  #  - M a D b: always true conflict
  #  - D a M b: always true conflict
  #
  # Other combinations are logically impossible, e.g. A a M b: A requires that
  # the repo was not present at base, M requires that it was present at base,
  # which is impossible by contradiction.

  it 'handles conflicts A a A b, a == b: trivial resolution', ->
    setMaster(NULL_SHA1)
    base = snapshot()

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    a = contentFaker.spec.commit._id
    local = snapshot()

    setMaster(base)
    # Create unrelated repo to force a modified synchro snapshot tree.
    contentFaker.createFakeContent { euid, store: contentStore }
    remote = snapshot()

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = merge()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {
      'branches/master': a
    }
    expect(leaves[repoName].conflicts).to.deep.eql {}
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles conflicts D a D b: trivial resolution', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    setMaster(NULL_SHA1)
    base = snapshot()

    removeContentRepos({ name: repoName })
    local = snapshot()

    setMaster(base)
    # Create unrelated repo to force a modified synchro snapshot tree.
    contentFaker.createFakeContent { euid, store: contentStore }
    remote = snapshot()

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = merge()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName]).to.not.exist
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles conflicts M a M b, a == b: trivial resolution', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    setMaster(NULL_SHA1)
    base = snapshot()

    contentFaker.commitFakeContent()
    a = contentFaker.spec.commit._id
    local = snapshot()

    setMaster(base)
    # Create unrelated repo to force a modified synchro snapshot tree.
    contentFaker.createFakeContent { euid, store: contentStore }
    remote = snapshot()

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = merge()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {
      'branches/master': a
    }
    expect(leaves[repoName].conflicts).to.deep.eql {}
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'proper merge, anonC conflicts', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    base = snapshot()
    contentFaker.commitFakeContent()
    repoMaster = contentFaker.spec.commit._id
    remote = snapshot()

    setMaster(base)

    contentFaker.amendFakeContent()
    repoMaster2 = contentFaker.spec.commit._id
    local = snapshot()
    setRemote(remote)

    { status, commitSha } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {}
    expect(leaves[repoName].conflicts).to.deep.eql {
      'branches/master': [repoMaster, repoMaster2].sort()
    }
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles conflicts A a A b, a != b: true conflict, anonC', ->
    setMaster(NULL_SHA1)
    base = snapshot()

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    a = contentFaker.spec.commit._id
    local = snapshot()

    setMaster(base)
    contentFaker.amendFakeContent()
    b = contentFaker.spec.commit._id
    remote = snapshot()

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {}
    expect(leaves[repoName].conflicts).to.deep.eql {
      'branches/master': [a, b].sort()
    }
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles conflicts M a M b, a != b: true conflict, anonC', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    setMaster(NULL_SHA1)
    base = snapshot()

    contentFaker.commitFakeContent()
    a = contentFaker.spec.commit._id
    local = snapshot()

    setMaster(base)
    contentFaker.amendFakeContent()
    b = contentFaker.spec.commit._id
    remote = snapshot()

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {}
    expect(leaves[repoName].conflicts).to.deep.eql {
      'branches/master': [a, b].sort()
    }
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles conflicts M a D b: true conflict, anonC', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    setMaster(NULL_SHA1)
    base = snapshot()

    contentFaker.commitFakeContent()
    a = contentFaker.spec.commit._id
    local = snapshot()

    setMaster(base)
    repo = contentStore.repos.findOne({ name: repoName })
    removeContentRepos({ name: repoName })
    remote = snapshot()
    contentStore.repos.insert(repo)
    contentStore.repos.update {
      _id: repo._id
    }, {
      $currentDate: { mtime: true }
    }

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {}
    expect(leaves[repoName].conflicts).to.deep.eql {
      'branches/master': [NULL_SHA1, a].sort()
    }
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles conflicts D a M b: true conflict, anonC', ->
    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    setMaster(NULL_SHA1)
    base = snapshot()

    # a b reversed.
    contentFaker.commitFakeContent()
    b = contentFaker.spec.commit._id
    remote = snapshot()

    setMaster(base)
    removeContentRepos({ name: repoName })
    local = snapshot()

    setMaster(local)
    setRemote(remote)

    { status, commitSha } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {}
    expect(leaves[repoName].conflicts).to.deep.eql {
      'branches/master': [NULL_SHA1, b].sort()
    }
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)

  it 'handles crisscross merge, anonC', ->
    #
    #      --baseA--changeA--mergeA
    #     /       \         /      \
    #  base    ----\--------        ?
    #     \   /     \              /
    #      --baseB---mergeB--------
    #
    removeContentRepos({})
    setMaster(NULL_SHA1)
    base = snapshot()

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    baseACon = contentFaker.spec.commit._id
    baseA = snapshot()

    contentFaker.commitFakeContent()
    changeACon = contentFaker.spec.commit._id
    changeA = snapshot()

    setMaster(base)
    removeContentRepos({})
    contentFaker.createFakeContent { euid, store: contentStore }
    baseBCon = contentFaker.spec.commit._id
    baseB = snapshot()

    setMaster(changeA)
    setRemote(baseB)
    { status, commitSha: mergeA } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {
      'branches/master': changeACon
    }
    expect(leaves[repoName].conflicts).to.deep.eql {}

    setMaster(baseB)
    setRemote(baseA)
    { status, commitSha: mergeB } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {
      'branches/master': baseACon
    }
    expect(leaves[repoName].conflicts).to.deep.eql {}

    setMaster(mergeB)
    setRemote(mergeA)
    { status, commitSha } = mergeAnonC()

    expect(status).to.eql 'merge'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.eql {
      'branches/master': changeACon
    }
    expect(leaves[repoName].conflicts).to.deep.eql {}


  it 'merge unrelated, several repos, no conflict', ->
    n = 10

    # Remove all content repos to get `2*n` repos below.
    removeContentRepos({})

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName = contentFaker.spec.repo.name
    repoMaster = contentFaker.spec.commit._id
    setMaster(NULL_SHA1)
    remote = snapshot()
    for i in [1...n]
      contentFaker.createFakeContent { euid, store: contentStore }
      remote = snapshot()

    removeContentRepos({ name: repoName })

    contentFaker.createFakeContent { euid, store: contentStore }
    repoName2 = contentFaker.spec.repo.name
    repoMaster2 = contentFaker.spec.commit._id
    setMaster(NULL_SHA1)
    local = snapshot()
    for i in [1...n]
      contentFaker.createFakeContent { euid, store: contentStore }
      local = snapshot()

    setRemote(remote)

    { status, commitSha } = merge()

    expect(status).to.eql 'merge-unrelated'
    leaves = getSnapshotLeaves()
    expect(leaves[repoName].refs).to.deep.equal({
      'branches/master': repoMaster
    })
    expect(leaves[repoName].conflicts).to.deep.eql {}
    expect(leaves[repoName2].refs).to.deep.equal({
      'branches/master': repoMaster2
    })
    expect(leaves[repoName2].conflicts).to.deep.eql {}
    expect(commitSha).to.not.eql(local)
    parents = syncStore.commits.findOne(commitSha).parents
    expect(parents[0]).to.eql(local)
    expect(parents[1]).to.eql(remote)
    expect(_.keys(leaves).length).to.eql(2 * n)
