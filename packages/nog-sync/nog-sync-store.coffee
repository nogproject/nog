{ check, Match } = require 'meteor/check'

{ createStagingPrefixTree } = require './nog-sync-merge-lib.coffee'

{
  createEntryCache,
  createEntryNullCache,
} = require './nog-sync-store-cache.js'

{
  createContentCollections
  createContentStore
} = NogContent

{
  ERR_UNKNOWN_USERNAME
  ERR_UPDATE_SYNCHRO
  ERR_SYNCHRO_MISSING
  ERR_SYNCHRO_CONTENT_MISSING
  ERR_CONTENT_MISSING
  ERR_CONTENT_CHECKSUM
  nogthrow
} = NogError

NULL_SHA1 = '0000000000000000000000000000000000000000'

RAW = {transform: false}

AA_GET_SYNCHRO_CONTENT = 'nog-sync/get'
AA_MODIFY_SYNCHRO_CONTENT = 'nog-sync/modify'
AA_CREATE_SYNCHRO = 'nog-sync/create'


isDuplicateMongoIdError = (err) ->
  return err.code == 11000


crypto = require('crypto')
sha1_hex = (d) -> crypto.createHash('sha1').update(d, 'utf8').digest('hex')


# `decodeMeta()` is identical to nogcontent.
#
# `verifyEncodedContent()` assumes that the sha verification works for any the
# `_idversion`.

decodeMeta = (encMeta) ->
  meta = _.omit encMeta, 'more'
  for r in encMeta.more ? []
    meta[r.key] = r.val
  meta

verifyEncodedContent = (type, content) ->
  d = _.clone content
  d.meta = decodeMeta d.meta
  if d._id != NogContent.contentId(NogContent.stripped(d))
    nogthrow ERR_CONTENT_CHECKSUM, {type, sha1: d._id}


matchSimpleName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid simple name.'
  true


matchSimpleRelPath = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ ( / [a-zA-Z0-9-_]+ )* $ ///)?
    throw new Match.Error 'Invalid ref name.'
  true


matchSha1 = Match.Where (x) ->
  check x, String
  if not (x.match /^[0-9a-f]{40}$/)?
    throw new Match.Error 'not a sha1'
  true


# `find().size()` as discussed on SO <http://stackoverflow.com/a/8390458> does
# not work with the Meteor Mongo wrapper.  Use `findOne()` instead and retrieve
# only the id.

collectionContains = (coll, sel) ->
  coll.findOne sel, {fields: {_id: 1}}


leafFromRepo = (repo) ->
  master = repo.refs['branches/master']
  cmaster = repo.conflicts?['branches/master']
  if cmaster?
    refs = {}
    if master?
      cmaster = cmaster.concat([master])
    else
      cmaster = cmaster.concat([NULL_SHA1])
    cmaster.sort()
    cmaster = _.uniq(cmaster, 1)  # 1 == isSorted.
    conflicts = { 'branches/master': cmaster }
  else
    conflicts = {}
    if master?
      refs = { 'branches/master': master }
    else
      refs = {}
  name = ['repo', repo.owner, repo.name].join(':')
  leaf = {
    name
    meta: {
      nog: {
        name: repo.name, owner: repo.owner,
        refs, conflicts,
      }
    }
    # XXX Maybe store entries `{type: 'commit', sha1: p[1]}` to prevent gc.
    entries: []
  }


_getRawEntriesFromStore = (store, opts) ->
  { treeShas, objectShas } = opts
  treeShas = _.unique(treeShas)
  objectShas = _.unique(objectShas)

  trees = []
  if treeShas.length > 0
    store.trees.find({ _id: { $in: treeShas } }, RAW).map (t) =>
      trees.push(t)
  unless trees.length == treeShas.length
    nogthrow ERR_CONTENT_MISSING

  objects = []
  if objectShas.length > 0
    store.objects.find({ _id: { $in: objectShas } }, RAW).map (o) =>
      objects.push(o)
  unless objects.length == objectShas.length
    nogthrow ERR_CONTENT_MISSING

  return { trees, objects }


class SyncStore

  constructor: (opts) ->
    {
      @synchros, @commits, @trees, @objects,
      @users, @contentStore, @ourPeerName,
      @checkAccess, @testAccess, @cache
    } = opts

    @nRepoPrefixLevels = 2

    # Use a ContentStore without access control as the low-level store.  Access
    # control for the low-level store is disabled, since `SyncStore` checks
    # access for its high-level operation, and access should not be checked
    # again with a different action.

    @_syncStore = createContentStore {
      repos: @synchros
      commits: @commits
      trees: @trees
      objects: @objects
      users: @users
      blobs: null
      deletedRepos: null
      reposSets: null
      checkAccess: ->
    }

  getRefs: (euid, opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
    }
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    { ownerName, synchroName } = opts

    sel = { owner: ownerName, name: synchroName, }
    fields = { refs: 1 }
    unless (s = @synchros.findOne(sel, { fields }))?
      nogthrow ERR_SYNCHRO_MISSING
    return s.refs

  updateRef: (euid, opts) ->
    check opts.synchroName, matchSimpleName
    # Rely on `_syncStore` to check remaining fields.
    @checkAccess euid, AA_MODIFY_SYNCHRO_CONTENT, opts
    opts.repoName = opts.synchroName
    delete opts.synchroName
    @_syncStore.updateRef euid, opts

  createTree: (euid, opts) ->
    check opts.synchroName, matchSimpleName
    # Rely on `_syncStore` to check remaining fields.
    @checkAccess euid, AA_MODIFY_SYNCHRO_CONTENT, opts
    opts.repoName = opts.synchroName
    delete opts.synchroName
    @_syncStore.createTree euid, opts

  createCommit: (euid, opts) ->
    check opts.synchroName, matchSimpleName
    # Rely on `_syncStore` to check remaining fields.
    @checkAccess euid, AA_MODIFY_SYNCHRO_CONTENT, opts
    opts.repoName = opts.synchroName
    delete opts.synchroName
    @_syncStore.createCommit euid, opts

  ensureSynchro: (euid, opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
      selector: Match.Optional {
        repos: Object
      }
    }
    {ownerName, synchroName, selector} = opts
    selector ?= {repos: {}, users: {}}

    @checkAccess euid, AA_CREATE_SYNCHRO, opts

    sel = {
      owner: ownerName
      name: synchroName
    }
    unless collectionContains(@synchros, sel)
      @_syncStore.createRepo euid, {
        repoFullName: [ownerName, synchroName].join('/')
      }

    n = @synchros.update sel, {$set: {selector}}
    unless n == 1
      nogthrow ERR_UPDATE_SYNCHRO, {
        reason: 'Failed to update selector.'
      }

    return


  getPing: (euid, opts) ->
    check opts, {owner: String}
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    {owner} = opts
    unless (s = @synchros.findOne({owner}, {fields: {_ping: 1}}))?
      return null
    return s._ping

  # Functions `*Raw()` use encoded meta as it is stored in Mongo.
  #
  # Functions `*RawSudo()` run without access check.  They are for code paths
  # that have already checked access on a higher level.
  #
  # Functions `insert*Raw*()` bypass dependency checks.  The caller should
  # use them responsibly in order to preserve invariants:
  #
  # - Trees and objects must be deep, that is dependencies must be inserted
  #   first.
  #
  # - Commits may be shallow, that is they may be inserted without parents.  It
  #   is undefined whether shallow provides any guarantees.  The safest
  #   approach, when the synchro code is active, is to assume nothing about
  #   commit ranges.  For example, a commit walk should not make assumptions
  #   about the presence of parent commits solely based on the presence of a
  #   commit.  It may be safe, though, to assume that all parent commits are
  #   present if a content ref points to a commit.

  getCommitRaw: (euid, opts) ->
    check opts, {sha: matchSha1}
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    {sha} = opts
    unless (commit = @commits.findOne(sha, RAW))?
      nogthrow ERR_SYNCHRO_CONTENT_MISSING, {commit: sha}
    return commit

  getTreeRaw: (euid, opts) ->
    check opts, {sha: matchSha1}
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    {sha} = opts
    unless (tree = @trees.findOne(sha, RAW))?
      nogthrow ERR_SYNCHRO_CONTENT_MISSING, {tree: sha}
    return tree

  getObjectRaw: (euid, opts) ->
    check opts, {sha: matchSha1}
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    {sha} = opts
    unless (object = @objects.findOne(sha, RAW))?
      nogthrow ERR_SYNCHRO_CONTENT_MISSING, {object: sha}
    return object

  getEntriesRaw: (euid, opts) ->
    check opts, {
      treeShas: [matchSha1],
      objectShas: [matchSha1],
    }
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    return _getRawEntriesFromStore this, opts

  insertCommitRawSudo: (opts) ->
    { content } = opts
    verifyEncodedContent 'commit', content
    try
      @commits.insert content
    catch err
      # Duplicate id indicates that we already had the commit.
      unless isDuplicateMongoIdError err
        throw err

  insertTreeRawSudo: (opts) ->
    { content } = opts
    verifyEncodedContent 'tree', content
    try
      @trees.insert content
    catch err
      # Duplicate id indicates that we already had the tree.
      unless isDuplicateMongoIdError err
        throw err

  insertObjectRawSudo: (opts) ->
    { content } = opts
    verifyEncodedContent 'object', content
    try
      @objects.insert content
    catch err
      # Duplicate id indicates that we already had the object.
      unless isDuplicateMongoIdError err
        throw err

  hasTreeSudo: (opts) -> @_syncStore.hasTreeSudo opts
  hasObjectSudo: (opts) -> @_syncStore.hasObjectSudo opts

  getCommitSudo: (opts) ->
    check opts, { sha: matchSha1 }
    { sha } = opts
    if (commit = @cache.get(sha))?
      return commit
    unless (commit = @commits.findOne(sha))?
      nogthrow ERR_SYNCHRO_CONTENT_MISSING, { commit: sha }
    @cache.add(commit)
    return commit

  getTreeSudo: (opts) ->
    check opts, { sha: matchSha1 }
    {sha} = opts
    if (tree = @cache.get(sha))?
      return tree
    unless (tree = @trees.findOne(sha))?
      nogthrow ERR_SYNCHRO_CONTENT_MISSING, { tree: sha }
    @cache.add(tree)
    return tree

  getObjectSudo: (euid, opts) ->
    check opts, { sha: matchSha1 }
    {sha} = opts
    if (object = @cache.get(sha))?
      return object
    unless (object = @objects.findOne(sha))?
      nogthrow ERR_SYNCHRO_CONTENT_MISSING, { object: sha }
    @cache.add(object)
    return object


  # The `getContent*()` calls skip the usual content access checks, such as
  # `action: 'nog-content/get'` and require synchro access permission instead.
  # This should be fine for root-like nogsyncbots, which have permission to
  # access all content.
  #
  # We'd need to reconsider if we wanted to extend the synchro design to
  # ordinary users.

  getContentCommitRaw: (euid, opts) ->
    check opts, { sha: matchSha1 }
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    { sha } = opts
    unless (commit = @contentStore.commits.findOne(sha, RAW))?
      nogthrow ERR_CONTENT_MISSING, { commit: sha }
    return commit

  getContentTreeRaw: (euid, opts) ->
    check opts, { sha: matchSha1 }
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    { sha } = opts
    unless (tree = @contentStore.trees.findOne(sha, RAW))?
      nogthrow ERR_CONTENT_MISSING, { tree: sha }
    return tree

  getContentObjectRaw: (euid, opts) ->
    check opts, { sha: matchSha1 }
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    { sha } = opts
    unless (object = @contentStore.objects.findOne(sha, RAW))?
      nogthrow ERR_CONTENT_MISSING, { object: sha }
    return object

  getContentEntriesRaw: (euid, opts) ->
    check opts, {
      treeShas: [matchSha1],
      objectShas: [matchSha1],
    }
    @checkAccess euid, AA_GET_SYNCHRO_CONTENT, opts
    return _getRawEntriesFromStore @contentStore, opts

  insertContentCommitRawSudo: (opts) ->
    { content } = opts
    verifyEncodedContent 'commit', content
    try
      @contentStore.commits.insert content
    catch err
      # Duplicate id indicates that we already had the commit.
      unless isDuplicateMongoIdError err
        throw err

  insertContentTreeRawSudo: (opts) ->
    { content } = opts
    verifyEncodedContent 'tree', content
    try
      @contentStore.trees.insert content
    catch err
      # Duplicate id indicates that we already had the tree.
      unless isDuplicateMongoIdError err
        throw err

  insertContentObjectRawSudo: (opts) ->
    { content } = opts
    verifyEncodedContent 'object', content
    try
      @contentStore.objects.insert content
    catch err
      # Duplicate id indicates that we already had the object.
      unless isDuplicateMongoIdError err
        throw err

  # XXX The meaning of blob placeholders need to be clarified.  We will
  # probably add a notion of a remote blob that is pending transfer.
  # We should also support storing blobs in multiple caches.

  insertContentBlobPlaceholderSudo: (opts) ->
    check opts, { sha: matchSha1 }
    { sha } = opts
    blob = { _id: sha, sha1: sha, status: 'placeholder', size: -1, }
    try
      @contentStore.blobs.insert blob
    catch err
      # Duplicate id indicates that we already had the object.
      unless isDuplicateMongoIdError err
        throw err


  snapshot: (args...) -> @snapshotAnonC args...
  fullSnapshot: (args...) -> @fullSnapshotAnonC args...


  snapshotAnonC: (euid, opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
    }
    @checkAccess euid, AA_MODIFY_SYNCHRO_CONTENT, opts
    opts = _.extend({
      conflictStyle: 'anonymous',
      strategy: 'incremental'
    }, opts)
    @_snapshot(euid, opts)


  fullSnapshotAnonC: (euid, opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
    }
    @checkAccess euid, AA_MODIFY_SYNCHRO_CONTENT, opts
    opts = _.extend({
      conflictStyle: 'anonymous',
      strategy: 'full'
    }, opts)
    @_snapshot(euid, opts)


  _snapshot: (euid, opts) ->
    {ownerName, synchroName, conflictStyle, strategy} = opts
    refName = 'branches/master'

    sel = {
      owner: ownerName
      name: synchroName
    }
    unless (syn = @synchros.findOne(sel))?
      nogthrow ERR_SYNCHRO_MISSING

    old = syn.refs[refName] ? NULL_SHA1

    if (op = @getOp({ ownerName, synchroName }))?
      console.log("[nog-sync] refusing snapshot due to active op #{op.op}.")
      return { status: 'refused', commit: old }

    mtimeMaxProcessed = null
    if strategy == 'incremental' && old != NULL_SHA1
      { mtimeMaxProcessed, rootSha } = @_incrSnapshotTree euid, {
        syn, ownerName, synchroName, baseCommitSha: old
      }
    else
      rootSha = @_fullSnapshotTree euid, {
        syn, ownerName, synchroName, conflictStyle
      }

    # Keep old commit if already up-to-date.
    unless old == NULL_SHA1
      oldc = @_syncStore.getCommit euid, {
        ownerName
        repoName: synchroName
        sha1: old
      }
      if oldc.tree == rootSha
        return {status: 'unchanged', commit: old}

    commit = {
      subject: 'Sync snapshot'
      message: ''
      parents: (if old != NULL_SHA1 then [old] else [])
      tree: rootSha
    }
    commitId = @_syncStore.createCommit euid, {
      ownerName
      repoName: synchroName
      content: commit
    }

    @_syncStore.updateRef euid, {
      ownerName
      repoName: synchroName
      refName
      old
      new: commitId
    }

    if mtimeMaxProcessed?
      @synchros.update {
        owner: ownerName, name: synchroName
      }, {
        $set: { mtimeMaxProcessed }
      }

    return {status: 'updated', commit: commitId}


  _fullSnapshotTree: (euid, opts) ->
    {syn, ownerName, synchroName, conflictStyle} = opts

    root = {
      name: 'synchro'
      entries: []
      meta: {}
    }

    root.entries.push {
      type: 'tree'
      sha1: @_fullReposSnapshotTree euid, {
        ownerName
        synchroName
        conflictStyle
        selector: syn.selector.repos
      }
    }

    return @_syncStore.createTree euid, {
      ownerName
      repoName: synchroName
      content: root
    }

  # XXX: The snapshot code directly uses the content collections without access
  # control.  The security critical part is the definition of the selector.
  # Whoever controls the selector can control which content is visible in the
  # snapshot.  The approach keeps the overhead for access control low, but it
  # is riskier than accessing content through the store methods.  We should
  # consider adding a function to `NogContent.Store` that returns a snapshot of
  # a repo after checking access control.  Or we add an access check here in
  # each map callback.
  #
  # XXX Handling of ids needs to be clarified, in particular related to
  # sharing.  The tentative decision is to consider MongoDB ids as local and
  # use URL-like names in the snapshot.  A repo is identified by
  # "{owner}/{name}".  Sharing would refer to a circle by name (not by a
  # MongoDB id); something like "{owner}/{circle}".  Details TBD.
  #
  # XXX The snapshot is currently restricted to `master`.  We will later
  # reconsider whether we add support for other branches.
  #
  # XXX Sharing is not yet implemented.

  _fullReposSnapshotTree: (euid, opts) ->
    {ownerName, synchroName, selector, conflictStyle} = opts

    snaps = @contentStore.repos.find(selector).map (repo) =>
      leaf = leafFromRepo(repo)
      treeId = @_syncStore.createTree euid, {
        ownerName
        repoName: synchroName
        content: leaf
      }
      return { name: leaf.name, treeId }

    prefixTree = {}
    for s in snaps
      shaid = sha1_hex(s.name)
      tree = prefixTree
      for lv in [0...@nRepoPrefixLevels]
        prefix = shaid[0..(lv * 2 + 1)]
        tree[prefix] ?= {}
        tree = tree[prefix]
      tree[shaid] = s

    asNogTree = (pfxTree, name) ->
      prefixes = _.keys(pfxTree)
      prefixes.sort()
      return {
        name,
        meta: {}
        entries: for pfx in prefixes
          child = pfxTree[pfx]
          if (treeId = child.treeId)?
            {type: 'tree', sha1: treeId}
          else
            asNogTree(child, 'repos:' + pfx)
      }

    return @_syncStore.createTree euid, {
      ownerName
      repoName: synchroName
      content: asNogTree(prefixTree, 'repos')
    }


  _incrSnapshotTree: (euid, opts) ->
    {syn, ownerName, synchroName, baseCommitSha } = opts

    store = {
      getCommit: (sha) => @getCommitSudo({ sha })
      getTree: (sha) => @getTreeSudo({ sha })
    }

    baseCommit = @getCommitSudo({ sha: baseCommitSha })
    root = @getTreeSudo({ sha: baseCommit.tree })
    root = _.clone(root)
    delete root._id
    delete root._idversion
    reposTreeSha = root.entries[0].sha1

    pfxTree = createStagingPrefixTree({ store, rootSha: reposTreeSha })

    selector = _.clone(syn.selector.repos)
    mtimeMaxProcessed = syn.mtimeMaxProcessed ? new Date(0)
    selector['mtime'] = { $gte: mtimeMaxProcessed }

    pfxUpsertRepo = (repo) =>
      if (mtime = repo.mtime)? && mtime > mtimeMaxProcessed
        mtimeMaxProcessed = mtime
      leaf = leafFromRepo(repo)
      pfxTree.set(leaf)

    pfxDelRepo = (repo) =>
      if (mtime = repo.mtime)? && mtime > mtimeMaxProcessed
        mtimeMaxProcessed = mtime
      existing = @contentStore.repos.findOne {
        owner: repo.owner, name: repo.name,
      }
      if existing?
        return
      leaf = leafFromRepo(repo)
      pfxTree.del(leaf)

    @contentStore.repos.find(selector).forEach pfxUpsertRepo
    @contentStore.deletedRepos.find(selector).forEach pfxDelRepo

    root.entries = _.clone(root.entries)
    root.entries[0] = pfxTree.asNogTree()
    rootSha = @_syncStore.createTree euid, {
      ownerName
      repoName: synchroName
      content: root
    }

    return { mtimeMaxProcessed, rootSha }


  pingSynchro: (euid, opts) ->
    {ownerName, synchroName, token} = opts
    @checkAccess euid, AA_MODIFY_SYNCHRO_CONTENT, opts
    sel = {
      owner: ownerName
      name: synchroName
    }
    $set = {}
    $set["_ping.#{ownerName}"] = token
    @synchros.update sel, {$set}


  setOp: (opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
      op: String
      prevOp: Match.Optional(String)
    }
    { ownerName, synchroName, prevOp, op } = opts
    sel = {
      owner: ownerName
      name: synchroName
    }
    if prevOp == '*'
      true  # Do not check state.
    else if prevOp?
      sel['op.op'] = prevOp
    else
      sel['op'] = { $exists: false }
    n = @synchros.update sel, {
      $set: { 'op.op': op },
      $currentDate: { 'op.atime': true },
    }
    return n == 1

  clearOp: (opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
      prevOp: String
    }
    { ownerName, synchroName, prevOp } = opts
    sel = {
      owner: ownerName
      name: synchroName
    }
    if prevOp == '*'
      true  # Do not check state.
    else if prevOp?
      sel['op.op'] = prevOp
    n = @synchros.update sel, {
      $unset: { op: '' }
    }
    return n == 1

  getOp: (opts) ->
    check opts, {
      ownerName: matchSimpleName
      synchroName: matchSimpleName
    }
    { ownerName, synchroName } = opts
    synchro = @synchros.findOne({
      owner: ownerName
      name: synchroName
    })
    return synchro?.op


createSyncStore = (opts) ->
  { namespace, caching } = opts

  nsColl = namespace.coll
  colls = createContentCollections {
    names: {repos: "#{nsColl}.synchros"}
    namespace: {coll: nsColl}
  }
  colls.synchros = colls.repos
  delete colls.repos

  if caching.maxNElements > 0
    cache = createEntryCache({
      name: "entryCache-#{nsColl}",
      maxNElements: caching.maxNElements,
      maxAge_s: caching.maxAge_s,
    })
  else
    cache = createEntryNullCache()

  return new SyncStore _.extend({cache}, colls, opts)


module.exports.AA_GET_SYNCHRO_CONTENT = AA_GET_SYNCHRO_CONTENT
module.exports.createSyncStore = createSyncStore
