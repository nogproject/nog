{ Meteor } = require 'meteor/meteor'
{ Mongo } = require 'meteor/mongo'
{ Match, check } = require 'meteor/check'
{ _ } = require 'meteor/underscore'

{
  NogContent
  NogContentTest
} = require './nog-content.coffee'
{
  CachedRepoSets
  TransientSet
} = require './nog-content-caching-server.coffee'

{ NogError } = require 'meteor/nog-error'
{
  ERR_API_VERSION
  ERR_CONTENT_MISSING
  ERR_CONTENT_REPO_EXISTS
  ERR_DB
  ERR_LOGIC
  ERR_CONFLICT
  ERR_PARAM_INVALID
  ERR_PARAM_MALFORMED
  ERR_REF_MISMATCH
  ERR_REF_NOT_FOUND
  ERR_REPO_MISSING
  ERR_UNKNOWN_USERNAME
  nogthrow
} = NogError


# Set to true to activate repo set update debugging.  XXX: Do not deploy with
# activated debugging.  We should remove the debug code when we are confident
# that the new implementation works as expected.
optDebugRepoSets = false

# See `README.md` for an introduction to the data model.

AA_CREATE_REPO = 'nog-content/create-repo'
AA_DELETE_REPO = 'nog-content/delete-repo'
AA_RENAME_REPO = 'nog-content/rename-repo'
AA_FORK_REPO = 'nog-content/fork-repo'
AA_MODIFY = 'nog-content/modify'
AA_GET = 'nog-content/get'

nullSha1 = '0000000000000000000000000000000000000000'

RAW = {transform: false}

# ISO datetime with UTC `Z`.
rgxISOStringUTC = ///
  ^
  [0-9]{4}-[0-9]{2}-[0-9]{2}
  T
  [0-9]{2}:[0-9]{2}:[0-9]{2}
  Z
  $
  ///


isDuplicateMongoIdError = (err) ->
  return err.code == 11000


matchOptionalISOStringUTC = Match.Where (x) ->
  if x?
    check x, String
    unless (x.match rgxISOStringUTC)?
      throw new Match.Error 'Invalid UTC datetime string.'
  true

# ISO UTC without fractional seconds (for commit idv0)
toSimpleISOString = (d) ->
  d.utc().format('YYYY-MM-DD[T]HH:mm:ss[Z]')

# ISO with tz offset, without fractional seconds.  Ensure trailing `+00:00`,
# because momentjs uses `Z` since 2.13.0; see
# <https://github.com/moment/moment/pull/3098>.
toISOStringTZ = (d) -> d.format().replace(/Z$/, '+00:00')

isObjectContent = (content) -> _.has content, 'blob'
isTreeContent = (content) -> _.has content, 'entries'
isCommitContent = (content) -> _.has content, 'tree'
isCollapsedEntry = (entry) -> _.has entry, 'type'


matchCommitIdVersion = Match.Where (x) ->
  check x, Number
  unless (0 <= x <= 1)
    throw new Match.Error "Expected 0 <= _idversion <= 1; got #{x}."
  true


matchObjectIdVersion = Match.Where (x) ->
  check x, Number
  unless (0 <= x <= 1)
    throw new Match.Error "Expected 0 <= _idversion <= 1; got #{x}."
  true


isSha1 = Match.Where (x) ->
  check x, String
  if not (x.match /^[0-9a-f]{40}$/)?
    throw new Match.Error 'not a sha1'
  true


isEntryType = Match.Where (x) ->
  check x, String
  if (x is 'object') or (x is 'tree')
    true
  else
    throw new Match.Error 'not an entry type'


matchExtEntryType = Match.Where (x) ->
  check x, String
  if (x is 'object') or (x is 'tree') or (x is 'blob') or (x is 'commit')
    true
  else
    throw new Match.Error 'not an extended entry type'


matchObjectContent = Match.Where (c) ->
  check c,
    _idversion: Match.Optional matchObjectIdVersion
    blob: Match.OneOf(null, isSha1)
    meta: Object
    name: String
    text: Match.Optional(Match.OneOf(null, String))
  true


matchCollapsedEntry = Match.Where (e) ->
  check e, {type: isEntryType, sha1: isSha1}
  true


matchCollapsedTreeContent = Match.Where (c) ->
  check c,
    name: String
    meta: Object
    entries: [matchCollapsedEntry]
  true


matchMixedEntry = Match.Where (e) ->
  check e, Object
  if isCollapsedEntry(e)
    check e, matchCollapsedEntry
  else if isObjectContent(e)
    check e, matchObjectContent
  else if isTreeContent(e)
    check e, matchMixedTreeContent
  else
    throw new Match.Error 'malformed entry'
  true


matchMixedTreeContent = Match.Where (c) ->
  check c,
    name: String
    meta: Object
    entries: [matchMixedEntry]
  true


isRepoFullName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ / [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid repo full name.'
  true


isSimpleName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid simple name.'
  true


isSimpleRelPath = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ ( / [a-zA-Z0-9-_]+ )* $ ///)?
    throw new Match.Error 'Invalid ref name.'
  true


matchNonNegativeIntegerString = (ctx) ->
  Match.Where (x) ->
    check x, String
    if not (x.match /// ^ ([1-9][0-9]*)? [0-9] $ ///)?
      throw new Match.Error 'Malformed non-negative integer ' + ctx
    true


matchSimpleFormatQuery = Match.Where (x) ->
  check x, String
  if x == 'minimal' or x == 'hrefs'
    return true
  throw new Match.Error "
      Malformed format query param value (expected `minimal` or `hrefs`, got
      `#{x}`)
    "

matchTreeFormatQuery_v1 = Match.Where (x) ->
  check x, String
  if x.match /// ^ (minimal|hrefs) (.v0)? $ ///
    return true
  throw new Match.Error "
      Malformed format query param value (expected `minimal` or `hrefs` with
      optional `.v0` suffix; got `#{x}`)
    "

matchObjectFormatQuery_v1 = matchCommitFormatQuery_v1 = Match.Where (x) ->
  check x, String
  if x.match /// ^ (minimal|hrefs) (.v[01])? $ ///
    return true
  throw new Match.Error "
      Malformed format query param value (expected `minimal` or `hrefs` with
      optional `.v0` or `.v1` suffix; got `#{x}`)
    "

parseFormatQuery_v1 = (f) ->
  f = f.split '.'
  switch f.length
    when 1
      {format: f[0], fmtversion: null}
    when 2
      {format: f[0], fmtversion: Number(f[1][1])}
    else
      nogthrow ERR_LOGIC


parseRepoFullName = (name) ->
  [ownerName, repoName] = name.split '/'
  return {ownerName, repoName}



# Selected metadata is stored in toplevel fields.  The rest in a `{key, val}`
# array.
#
# `errata` are stored but ignored when computing the content id, so that they
# can be used to amend existing entries.

encodeMeta = (meta) ->
  selected = ['description', 'content']
  enc = {more: []}
  for k, v of meta
    if k in selected
      enc[k] = v
    else
      enc.more.push {key: k, val: v}
  enc


create = (coll, content) ->
  content = _.omit content, '_id', '_idversion'
  id = NogContent.contentId _.omit(content, 'errata')
  if not coll.findOne id
    if content.meta?
      content.meta = encodeMeta content.meta
    coll.insert _.extend {_id: id}, content
  id


# `find().size()` as discussed on SO <http://stackoverflow.com/a/8390458> does
# not work with the Meteor Mongo wrapper.  Use `findOne()` instead and retrieve
# only the id.

collectionContains = (coll, sel) ->
  if typeof sel == 'string'
    cacheKey = "#{coll._name}:#{sel}"
  else if typeof sel == 'object'
    cacheKey = NogContent.contentId(_.extend({_collName: coll._name}, sel))
  else
    nogthrow ERR_LOGIC, { reason: 'Unknown selector type.' }
  if collectionContainsCache.contains(cacheKey)
    return true
  if coll.findOne(sel, {fields: {_id: 1}})?
    collectionContainsCache.insert(cacheKey)
    return true
  return false

# It seems unproblematic to cache information whether a content collection
# contains an entry for a bit, but better not for too long, since it may cause
# minor side-effects.  For example, when a client tries to access a ref right
# after it deleted the repo, it may get ERR_REF_NOT_FOUND instead of
# ERR_REPO_MISSING.  A client may also be able to create new content in a repo
# that it deleted just before.  But it won't be able to update the ref, so the
# new content will be eventually garbage-collected.

collectionContainsCache = new TransientSet {
  name: 'collectionContains cache',
  maxSize: 16 * 1024
  maxAge_s: 5 * 60
}


# `null` must be handled gracefully to allow anonymous commits.
fmtUserAndEmail = (user) ->
  name = user?.profile?.name
  name ?= 'unknown'
  email = user?.emails?[0]?.address
  email ?= 'unknown'
  return "#{name} <#{email}>"


Meteor.startup ->
  NogContent.store.startCron()


# Commands that require a sequence of MongoDB operations are maintained in
# `repo.cmd`.  Each repo can have a single pending command, which blocks other
# commands.  Update selectors `selXcl`, for select exclusive, check that there
# is no pending command.  The generic error for a failed update is
# `ERR_CONFLICT`.  Some error handlers try to determine a more specific reason.
#
# Completion of pending commands is driven by an interval from `startCron()`,
# which calls `tick()` as the function that drives progress.
#
#`deleteRepo()` is currently the only multi-step command.

createContentStore = (opts) -> new Store(opts)

class Store
  constructor: (opts) ->
    {
      @repos, @commits, @trees, @objects, @blobs, @checkAccess, @users,
      @deletedRepos, @repoSets
    } = opts
    @repos._ensureIndex {owner: 1, name: 1}, {unique: true}
    try
      @repos._ensureIndex {ownerId: 1, name: 1}, {unique: true}
    catch err
      console.log '
          [nog-content] Warning: failed to create index (ownerId, name).  This
          message should disappear after applying the migration `addOwnerId()`.
        '
    @repos._ensureIndex {mtime: 1}
    @repos._ensureIndex { 'cmd.ctime': 1 }, { sparse: true }
    if @deletedRepos?
      @deletedRepos._ensureIndex {mtime: 1}
      @deletedRepos._ensureIndex { 'cmd.ctime': 1 }, { sparse: true }

    # Interval between scans for pending commands.
    @cronTickInterval_s = 10

    # Timeout after which a pending command is restarted by a cron `tick()`.
    @defaultCmdTimeout_s = 10

    # Hook that allows tests to simulate interrupted commands.
    @_maybeCrash = ->

  startCron: ->
    tick = => @tick()
    Meteor.setInterval(tick, @cronTickInterval_s * 1000)

  # Tests use `opts = { timeout_s: 0 }` to force immediate completion.
  tick: (opts) ->
    opts ?= {}
    @deleteRepoTick(opts)

  createRepo: (user, opts) ->
    check opts,
      repoFullName: isRepoFullName
    opts = _.extend parseRepoFullName(opts.repoFullName), opts

    @checkAccess user, AA_CREATE_REPO, opts

    ownerDoc = @users.findOne {
        username: opts.ownerName
      }, {
        fields: {_id: 1, username: 1}
      }
    if not ownerDoc?
      nogthrow ERR_UNKNOWN_USERNAME, {username: opts.ownerName}

    try
      id = @repos.insert
        owner: ownerDoc.username
        ownerId: ownerDoc._id
        name: opts.repoName
        refs:
          'branches/master': nullSha1
    catch err
      sel = {owner: opts.ownerName, name: opts.repoName}
      if collectionContains(@repos, sel)
        nogthrow ERR_CONTENT_REPO_EXISTS, opts
      else
        nogthrow ERR_DB, {cause: err}

    return id

  deleteRepo: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
    @checkAccess user, AA_DELETE_REPO, opts

    sel = {owner: opts.ownerName, name: opts.repoName}
    repo = @repos.findOne(sel, RAW)
    unless repo?
      nogthrow ERR_REPO_MISSING

    repo.cmd = { _id: Random.id(), op: 'del', ctime: new Date }
    selXcl = { _id: repo._id, cmd: { $exists: false } }
    setCmd = { $set: { cmd: repo.cmd } }
    unless @repos.update(selXcl, setCmd) == 1
      nogthrow ERR_CONFLICT

    return @_deleteRepo2(repo)

  # Restart func for stale cmd in `repos`.
  _deleteRepo2: (repo) ->
    try
      @deletedRepos.insert repo
    catch err
      unless isDuplicateMongoIdError(err)
        throw err
    @_maybeCrash '_deleteRepo2'
    return @_deleteRepo3(repo)

  # Restart func for stale cmd in `deletedRepos`.
  _deleteRepo3: (repo) ->
    @_maybeCrash '_deleteRepo3'
    @repos.remove repo._id
    @deletedRepos.update {
      _id: repo._id, 'cmd._id': repo.cmd._id
    }, {
      $unset: { cmd: '' },
      $currentDate: { mtime: true },
    }
    return

  deleteRepoTick: (opts) ->
    { timeout_s } = opts
    timeout_s ?= @defaultCmdTimeout_s
    cutoff = new Date()
    cutoff.setSeconds(cutoff.getSeconds() - timeout_s)
    @deletedRepos.find(
      { 'cmd.op': 'del', 'cmd.ctime': { $lte: cutoff } }
    ).forEach (repo) =>
      console.log(
        '[content] Restart cmd', repo.cmd._id, 'deleteRepo', repo._id,
        'from deletedRepos.'
      )
      @_deleteRepo3(repo)
    @repos.find(
      { 'cmd.op': 'del', 'cmd.ctime': { $lte: cutoff } }
    ).forEach (repo) =>
      console.log(
        '[content] Restart cmd', repo.cmd._id, 'deleteRepo', repo._id,
        'from repos.'
      )
      @_deleteRepo2(repo)

  renameRepo: (user, opts) ->
    check opts,
      old:
        ownerName: isSimpleName
        repoName: isSimpleName
      new:
        repoFullName: isRepoFullName
    opts.new = parseRepoFullName(opts.new.repoFullName)
    @checkAccess user, AA_RENAME_REPO, opts

    # Rely on the unique index to detect that the new name already exists.
    selOld = { owner: opts.old.ownerName, name: opts.old.repoName }
    selOldXcl = _.extend({ cmd: { $exists: false } }, selOld)
    try
      n = @repos.update selOldXcl, {
          $set: {owner: opts.new.ownerName, name: opts.new.repoName}
          $push: {oldFullNames: "#{opts.old.ownerName}/#{opts.old.repoName}"}
        }
    catch err
      # If the update failed, check if the name is already used.
      selNew = { owner: opts.new.ownerName, name: opts.new.repoName }
      if collectionContains(@repos, selNew)
        nogthrow ERR_CONTENT_REPO_EXISTS, {
            repoFullName: "#{opts.new.ownerName}/#{opts.new.repoName}"
          }
      else
        nogthrow ERR_DB, {cause: err}
    unless n == 1
      unless @repos.findOne(selOld)?
        nogthrow ERR_REPO_MISSING
      nogthrow ERR_CONFLICT
    return

  forkRepo: (user, opts) ->
    check opts,
      old:
        ownerName: isSimpleName
        repoName: isSimpleName
      new:
        ownerName: isSimpleName
    @checkAccess user, AA_FORK_REPO, opts.old
    @checkAccess user, AA_GET, opts.old

    sel = {owner: opts.old.ownerName, name: opts.old.repoName}
    selXcl = _.extend({ cmd: { $exists: false } }, sel)
    unless (src = @repos.findOne(selXcl))?
      unless @repos.findOne(sel)?
        nogthrow ERR_REPO_MISSING
      nogthrow ERR_CONFLICT

    # Some logic of createRepo() is duplicated here in order to insert the new
    # repo doc atomically.
    ownerName = opts.new.ownerName
    ownerDoc = @users.findOne {username: ownerName}, {fields: {_id: 1}}
    if not ownerDoc?
      nogthrow ERR_UNKNOWN_USERNAME, {username: ownerName}
    ownerId = ownerDoc._id

    dstRepo = {
      owner: ownerName
      ownerId
      name: null
      refs: src.refs
      forkedFrom:
        id: src._id
        owner: src.owner
        name: src.name
    }
    postfix = 1
    loop
      dstRepo.name = src.name
      if postfix > 1
        dstRepo.name += "_#{postfix}"
      postfix++
      @checkAccess user, AA_CREATE_REPO, {
          ownerName: dstRepo.owner, repoName: dstRepo.name
        }
      try
        id = @repos.insert dstRepo
      catch err
        sel = {owner: ownerName, name: dstRepo.name}
        if collectionContains(@repos, sel)
          continue  # Resolve naming conflict by looping with postfix.
        else
          nogthrow ERR_DB, {cause: err}
      break
    return {
      id
      owner: dstRepo.owner
      name: dstRepo.name
    }

  # Circle sharing read access, if active, is checked by `checkAccess()` via a
  # special NogAccess statement (see `nog-sharing.coffee`).
  _checkAccessGetContent: (user, opts) ->
    aopts = _.pick(opts, 'ownerName', 'repoName')
    @checkAccess user, AA_GET, aopts
    @_checkRepoExists(opts)

  # Content creation requires access rights and an existing repo.
  _checkAccessCreateContent: (user, opts) ->
    aopts = _.pick(opts, 'ownerName', 'repoName')
    @checkAccess user, AA_MODIFY, aopts
    @_checkRepoExists(opts)

  _checkRepoExists: (opts) ->
    reposel = {owner: opts.ownerName, name: opts.repoName}
    unless collectionContains(@repos, reposel)
      nogthrow ERR_REPO_MISSING

  getObject: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
      sha1: isSha1
    @_checkAccessGetContent user, opts
    if @repoSets?
      @repoSets.checkMembership opts, 'object'
    @objects.findOne opts.sha1

  createObject: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
      content: matchObjectContent
    @_checkAccessCreateContent user, opts
    @_createObject opts

  _createObject: (opts) ->
    {content} = opts

    if @blobs and (blob = content.blob)? and (blob != nullSha1)
      unless collectionContains(@blobs, blob)
        nogthrow ERR_CONTENT_MISSING, {blob}
      if @repoSets?
        @repoSets.checkMembership {
            ownerName: opts.ownerName
            repoName: opts.repoName
            sha1: blob
          }, 'blob'

    # Create idv1 unless explicitly specified otherwise.
    idv = content._idversion ? 1

    # Munge content to match idv.
    if content.text? and content.meta.content?
      nogthrow ERR_PARAM_INVALID, {
          reason: '`text` and `meta.content` cannot be used together.'
        }

    content = _.clone content
    content.meta = _.clone content.meta
    switch idv
      when 0
        if content.text?
          content.meta.content = content.text
        if _.isNull(content.blob)
          content.blob = nullSha1
        delete content.text
      when 1
        if content.blob? and content.blob == nullSha1
          content.blob = null
        if content.meta.content?
          content.text = content.meta.content
          delete content.meta.content
        else
          content.text ?= null

    sha1 = create @objects, content
    if @repoSets?
      @repoSets.updateMembership opts, {type: 'object', sha1}
    return sha1

  getTree: (user, opts) ->
    @_getTreeCheckOpts(opts)
    @_checkAccessGetContent(user, opts)
    @_getTreeCheckMembership(opts)
    return @_getTree(opts)

  getTreeSudo: (opts) ->
    @_getTreeCheckOpts(opts)
    @_checkRepoExists(opts)
    @_getTreeCheckMembership(opts)
    return @_getTree(opts)

  _getTreeCheckOpts: (opts) ->
    check opts, {
      ownerName: isSimpleName
      repoName: isSimpleName
      sha1: isSha1
      expand: Match.Optional(Number)
    }

  _getTreeCheckMembership: (opts) ->
    if @repoSets?
      @repoSets.checkMembership(opts, 'tree')

  _getTree: (opts) ->
    getExpanded = (sha1, levels) =>
      tree = @trees.findOne sha1
      if levels == 0
        return tree
      tree.entries = for e in tree.entries
        switch e.type
          when 'object'
            @objects.findOne e.sha1
          when 'tree'
            getExpanded(e.sha1, levels - 1)
          else
            nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
      return tree
    getExpanded opts.sha1, opts.expand ? 0

  createTree: (user, opts) ->
    @_createTreeCheckOpts(opts)
    @_checkAccessCreateContent(user, opts)
    @_createTree(opts)

  createTreeSudo: (opts) ->
    @_createTreeCheckOpts(opts)
    @_checkRepoExists(opts)
    @_createTree(opts)

  _createTreeCheckOpts: (opts) ->
    check opts, {
      ownerName: isSimpleName
      repoName: isSimpleName
      content: matchMixedTreeContent
    }

  _createTree: (opts) ->
    {ownerName, repoName} = opts
    collapsed = []
    for e in opts.content.entries
      if isCollapsedEntry(e)
        switch e.type
          when 'object'
            unless collectionContains @objects, e.sha1
              nogthrow ERR_CONTENT_MISSING, {object: e.sha1}
          when 'tree'
            unless collectionContains @trees, e.sha1
              nogthrow ERR_CONTENT_MISSING, {tree: e.sha1}
          else
            nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
        if @repoSets?
          @repoSets.checkMembership {ownerName, repoName, sha1: e.sha1}, e.type
        collapsed.push e
      else if isObjectContent(e)
        collapsed.push {
            type: 'object'
            sha1: @_createObject {ownerName, repoName, content: e}
          }
      else if isTreeContent(e)
        collapsed.push {
            type: 'tree'
            sha1: @_createTree {ownerName, repoName, content: e}
          }
      else
        nogthrow ERR_LOGIC, {reason: 'Malformed content.'}
    c = _.omit opts.content, 'entries'
    c.entries = collapsed
    sha1 = create @trees, c
    if @repoSets?
      @repoSets.updateMembership opts, {type: 'tree', sha1}
    return sha1

  getCommit: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
      sha1: isSha1
    @_checkAccessGetContent user, opts
    if @repoSets?
      @repoSets.checkMembership opts, 'commit'
    @commits.findOne opts.sha1

  createCommit: (user, opts) ->
    @_createCommitCheckOpts(opts)
    @_checkAccessCreateContent(user, opts)
    @_commit(user, opts)

  createCommitSudo: (opts) ->
    @_createCommitCheckOpts(opts)
    @_checkRepoExists(opts)
    @_commit(null, opts)

  _createCommitCheckOpts: (opts) ->
    check opts, {
      ownerName: isSimpleName
      repoName: isSimpleName
      content:
        _idversion: Match.Optional matchCommitIdVersion
        subject: String
        message: String
        parents: [isSha1]
        tree: isSha1
        meta: Match.Optional Object
        authors: Match.Optional [String]
        authorDate: Match.Optional Match.OneOf(String, Date)
        committer: Match.Optional String
        commitDate: Match.Optional Match.OneOf(String, Date)
    }

  _commit: (user, opts) ->
    c = _.clone opts.content
    unless collectionContains @trees, c.tree
      nogthrow ERR_CONTENT_MISSING, {tree: c.tree}
    for p in c.parents
      unless collectionContains @commits, p
        nogthrow ERR_CONTENT_MISSING, {commit: p}
    if @repoSets?
      {ownerName, repoName} = opts
      # Check parents first to keep test implementation simpler.
      for p in c.parents
        @repoSets.checkMembership {ownerName, repoName, sha1: p}, 'commit'
      @repoSets.checkMembership {ownerName, repoName, sha1: c.tree}, 'tree'

    c.meta ?= {}

    # Format dates (`String` or `Date`) as ISO strings as UTC `Z` strings
    # (idv0) or with timezone (idv1).  Use now as the default if a date is
    # invalid.  Use v1 by default.

    idversion = c._idversion ? 1
    now = moment().utc()

    fmtDate = (d) ->
      d = moment.parseZone(d)
      unless d.isValid()
        d = now
      switch idversion
        when 0
          unless d.isUTC()
            nogthrow ERR_PARAM_INVALID, {
              reason: 'Cannot store non-UTC date with idversion 0.'
            }
          toSimpleISOString(d)
        when 1
          toISOStringTZ(d)
        else
          throw ERR_LOGIC


    c.authors ?= [fmtUserAndEmail user]
    c.authorDate = fmtDate(c.authorDate ? now)
    c.committer ?= fmtUserAndEmail user
    c.commitDate = fmtDate(c.commitDate ? now)

    sha1 = create @commits, c
    if @repoSets?
      @repoSets.updateMembership opts, {type: 'commit', sha1}
    return sha1

  copyEntry: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
      content:
        copy:
          type: matchExtEntryType
          sha1: isSha1
          repoFullName: isRepoFullName
    @_checkAccessCreateContent user, opts
    copy = _.extend(
        parseRepoFullName(opts.content.copy.repoFullName), opts.content.copy
      )
    @_checkAccessGetContent user, copy

    switch copy.type
      when 'object'
        unless collectionContains(@objects, copy.sha1)
          nogthrow ERR_CONTENT_MISSING, {object: copy.sha1}
      when 'tree'
        unless collectionContains(@trees, copy.sha1)
          nogthrow ERR_CONTENT_MISSING, {tree: copy.sha1}
      when 'commit'
        unless collectionContains(@commits, copy.sha1)
          nogthrow ERR_CONTENT_MISSING, {commit: copy.sha1}
      when 'blob'
        unless @blobs? and collectionContains(@blobs, copy.sha1)
          nogthrow ERR_CONTENT_MISSING, {blob: copy.sha1}
      else
        nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
    if @repoSets?
      @repoSets.checkMembership {
          ownerName: copy.ownerName
          repoName: copy.repoName
          sha1: copy.sha1
        }, copy.type
      @repoSets.updateMembership opts, {type: copy.type, sha1: copy.sha1}

    return _.pick copy, 'type', 'sha1'


  getRef: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
      refName: isSimpleRelPath

    @_checkAccessGetContent user, opts

    refFieldName = "refs.#{opts.refName}"
    sel =
      owner: opts.ownerName
      name: opts.repoName
    sel[refFieldName] = {$exists: true}
    proj = {fields: {}}
    proj.fields[refFieldName] = 1
    if not (res = @repos.findOne(sel, proj))?
      nogthrow ERR_REF_NOT_FOUND, {refName: opts.refName}
    res.refs[opts.refName]

  getRefs: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
    @_checkAccessGetContent user, opts
    sel =
      owner: opts.ownerName
      name: opts.repoName
    proj = {fields: {refs: 1}}
    @repos.findOne(sel, proj).refs

  updateRef: (user, opts) ->
    @_updateRefCheckOpts(opts)
    @_checkAccessCreateContent(user, opts)
    @_updateRef(opts)

  updateRefSudo: (opts) ->
    @_updateRefCheckOpts(opts)
    @_checkRepoExists(opts)
    @_updateRef(opts)

  _updateRefCheckOpts: (opts) ->
    check opts, {
      ownerName: isSimpleName
      repoName: isSimpleName
      refName: isSimpleRelPath
      old: Match.OneOf isSha1, null
      new: Match.OneOf isSha1, null
    }

  _updateRef: (opts) ->
    opts.old ?= nullSha1
    opts.new ?= nullSha1

    unless opts.new == nullSha1
      unless collectionContains @commits, opts.new
        nogthrow ERR_CONTENT_MISSING, {commit: opts.new}
      if @repoSets?
        @repoSets.checkMembership {
            ownerName: opts.ownerName, repoName: opts.repoName, sha1: opts.new
          }, 'commit'

    refFieldName = "refs.#{opts.refName}"

    # Update null to nullSha1 to avoid condition logic below.
    sel =
      owner: opts.ownerName
      name: opts.repoName
    sel[refFieldName] = null
    modif = {$set: {}}
    modif.$set[refFieldName] = nullSha1
    @repos.update(sel, modif)

    # Then update to new ref.
    sel[refFieldName] = opts.old
    selXcl = _.extend({ cmd: { $exists: false } }, sel)
    if opts.refName == 'branches/master'
      if (c = @commits.findOne({_id: opts.new}, {fields: {commitDate: 1}}))?
        modif.$set['lastCommitDate'] = c.commitDate.toDate()
    modif.$set[refFieldName] = opts.new
    modif.$currentDate = { mtime: true }
    unless @repos.update(selXcl, modif) == 1
      unless @repos.findOne(sel)?
        nogthrow ERR_REF_MISMATCH
      nogthrow ERR_CONFLICT

  stat: (user, opts) ->
    check opts,
      ownerName: isSimpleName
      repoName: isSimpleName
      entries: [{type: String, sha1: isSha1}]
    @_checkAccessGetContent user, opts
    for e in opts.entries
      isAvailable = switch e.type
        when 'object'
          collectionContains(@objects, {_id: e.sha1})
        when 'tree'
          collectionContains(@trees, {_id: e.sha1})
        when 'blob'
          if @blobs?
            collectionContains(@blobs, {_id: e.sha1, status: 'available'})
          else
            false
        when 'commit'
          collectionContains(@commits, {_id: e.sha1})
        else
          false
      if isAvailable and @repoSets?
        isAvailable = @repoSets.isMember {
            ownerName: opts.ownerName,
            repoName: opts.repoName,
            sha1: e.sha1
          }
        # Update the repo set to ensure that the reported stat remains valid
        # for at least `repoSetsExpireAfter_s`.
        if isAvailable
          @repoSets.updateMembership opts, e
      _.extend {status: (if isAvailable then 'exists' else 'unknown')}, e

  hasCommitSudo: (opts) ->
    check opts, { sha: isSha1 }
    return collectionContains(@commits, opts.sha)

  hasTreeSudo: (opts) ->
    check opts, { sha: isSha1 }
    return collectionContains(@trees, opts.sha)

  hasObjectSudo: (opts) ->
    check opts, { sha: isSha1 }
    return collectionContains(@objects, opts.sha)

# `RepoSets` implements repo membership tests in order to support fine-grained
# access control via circles.  Entries that exist in mongo are reported as
# unknown unless they are reachable from the repo that is used during access.
# This ensures that strangers cannot access content by guessing a sha1.
# Entries are reachable if there is a path from a ref or if they have been
# added to the repo recently.
#
# The implementation is tuned towards detecting membership quickly; detecting
# non-membership may be slower.  If an entry is removed from a repo (by
# deleting a ref), it may take a while until the membership test reports false.
#
# Repo membership is cached in the collection `nogcontent.repo_sets` with docs
# `{sha1, repoId, ts}`.  `ts` stands for timestamp.  `sha1` can be of any type
# `blob`, `object`, `tree`, `commit`.  The type is not tracked, assuming that
# different types always have different content representation, so that there
# are no trivial sha1 collisions.  Docs are expired with a TTL index on the
# timestamp `ts`.
#
# The cache is updated by walking recursively along all possible paths from the
# repo refs (commit to tree, tree to entries, object to blob) and updating all
# children timestamps before the parent timestamp, so that the parent timestamp
# is always less or equal than all children timestamps.  The invariant allows
# terminating a walk based on the parent timestamp: if it is new enough, all
# children are guaranteed to be new enough, too.
#
# The cache is only updated if the timestamp of the entry where the walk is
# supposed to start is close to expiry.

repoSetsExpireAfter_s = 24 * 60 * 60


# `MongoLock` uses a db doc to implement a mutex that expires after a certain
# time.  It can be refreshed to postpone expiry.  `lock()` explicitly checks
# expiry, so that it works with any collection.  An alternative would be to use
# TTL index with the right expiry time as described in
# <https://blog.codecentric.de/en/2012/10/mongodb-pessimistic-locking/>.
#
# The implementation uses a `nonce` to safely identify ownership during
# removal, in case the lock timed out and has been replaced by another
# instance.
#
# The opts to the ctor are:
#
#  - `collection`: The collection in which to put the lock doc.
#  - `id`: The id of the lock doc.  It must be unique within `collection`.
#  - `expires_s`: Time until expiry in seconds.
#
class MongoLock
  constructor: (opts) ->
    {@collection, @id, @expires_s} = opts
    @nonce = Random.id()

  lock: ->
    now = new Date()
    locked = false
    until locked
      try
        @collection.insert {_id: @id, nonce: @nonce, ts: now}
        locked = true
      catch err
        unless isDuplicateMongoIdError(err)
          throw err
        Meteor._sleepForMs 150
        cutoff = new Date()
        cutoff.setSeconds(cutoff.getSeconds() - @expires_s)
        if @collection.remove {_id: @id, ts: {$lt: cutoff}}
          console.log '[nog-content] Warning: forced lock reset after expiry:',
            "collection `#{@collection._name}`, lockId `#{@id}`."

  unlock: ->
    @collection.remove {_id: @id, nonce: @nonce}

  refresh: ->
    now = new Date()
    sel = {_id: @id, nonce: @nonce}
    @collection.update sel, {$set: {ts: now}}


class RepoSets
  constructor: (opts) ->
    {
      @repoSets, @repos, @commits, @trees, @objects, @blobs
    } = opts

    # Maintain an TTL index to expire cached membership results.  Maintain an
    # index on `repoId` and `sha1` to quickly test membership: put `repoId`
    # first to be B-tree friendly: the same `repoId` is inserted repeatedly
    # during a walk.
    @repoSets._ensureIndex {repoId: 1, sha1: 1}
    @repoSets._ensureIndex {
        ts: 1
      }, {
        expireAfterSeconds: 2 * repoSetsExpireAfter_s
      }

    # `updateMembership()` stores the per-repo seen entries in `@seen`, so that
    # `isMember()` can check them and return `true` early.
    @seen = {}

  isMember: (opts) ->
    if not @repoSets?
      return true
    reposel = {owner: opts.ownerName, name: opts.repoName}
    repo = @repos.findOne reposel, {fields: {_id: 1, refs: 1}}
    repoId = repo._id
    sel = {sha1: opts.sha1, repoId}
    if collectionContains(@repoSets, sel)
      return true
    for k, v of repo.refs
      if v == nullSha1
        continue
      # Run update in the backgroud and repeatedly check whether it already
      # found `opts.sha1` in order to return `true` ASAP.  Catch errors to
      # avoid spinning forever if `updateMembership()` unexpectedly throws.
      done = false
      go = (fn) -> Meteor.setTimeout fn, 0
      go =>
        try
          @updateMembership opts, {type: 'commit', sha1: v}
          done = true
        catch err
          done = true
          throw err
      until done
        # Yield, then check.
        Meteor._sleepForMs 100
        if @seen[repoId]?[opts.sha1]
          return true
      # Now check based on collection, since in-memory `@seen[repoId]` may have
      # been deleted.
      if collectionContains(@repoSets, sel)
        return true
    return false

  checkMembership: (opts, ty) ->
    unless @isMember opts
      ctx = {}
      ctx[ty] = opts.sha1
      nogthrow ERR_CONTENT_MISSING, ctx

  updateMembership: (opts, entry) ->
    if not @repoSets?
      return

    reposel = {owner: opts.ownerName, name: opts.repoName}
    repoId = @repos.findOne(reposel, {fields: {_id: 1}})._id

    # XXX: Putting the locks into a separate collection would probably be less
    # confusing.  I'm not sure whether it would also be faster.  Let's maintain
    # the locks in `repoSet` to avoid creating another collection.
    lock = new MongoLock {
      collection: repoSets, id: repoId + '.lock', expires_s: 100
    }
    lock.lock()
    @updateMembershipBulkFront repoId, entry, lock
    lock.unlock()


  # `updateMembershipBulkFront()` traverses the connectivity graph in
  # iterations that each handle a complete front of pending entries.  See
  # details in comment at the loop.  Nodes are maintained in a waiting list
  # until all children have been handled.  A node can be safely inserted into
  # the repo set at this point, too (it is as part of a bulk document; see
  # `flush` and `waitingGraph`).  This order guarantees that the `repoSets`
  # collection only contains sha1s of entries for which all dependencies are
  # also in the repo set, recursively.  A traversal can thus safely stop at
  # nodes that are already in the `repoSets` collection.
  #
  # The entries in the `repoSets` collections are tagged by `repoId` and
  # timestamp `ts`.  Each pair `(repoId, ts)` establishes an isolated set to
  # which elements are only added.  Clean up is handled by the the TTL index.
  # An important invariant is that for each element, all dependencies are also
  # in the set, recursively.

  updateMembershipBulkFront: (repoId, entry, lock) ->

    # Maintain a stable target timestamp for each repo to avoid repeated
    # updates.  The target timestamp is only updated if it is older than the
    # cutoff.  The TTL index expires another `repoSetsExpireAfter_s` after the
    # `cutoff`, so that fresh objects remain in the repo set for at least
    # `repoSetsExpireAfter_s`.  After that, they must be reachable by a ref.

    cutoff = new Date()
    cutoff.setSeconds(cutoff.getSeconds() - repoSetsExpireAfter_s)
    tsId = repoId + '.ts'
    until (d = @repoSets.findOne({_id: tsId, ts: {$gte: cutoff}}))?
      now = new Date()
      sel = {_id: tsId}
      @repoSets.upsert sel, {$setOnInsert: {ts: now}}
      sel = {_id: tsId, ts: {$lt: cutoff}}
      @repoSets.update sel, {$set: {ts: now}}
    target = d.ts

    # Do not start the traversal if `entry` is already in the repo set or if it
    # is a blob, which can be inserted right away.

    sel = {sha1: entry.sha1, repoId, ts: {$gte: cutoff}}
    if collectionContains(@repoSets, sel)
      return
    if entry.type == 'blob'
      @repoSets.insert {
        repoId, ts: target, sha1: [entry.sha1]
      }
      return

    # The data structures for managing traversal:
    #
    # `front` contains lists of sha1s for the entries to be processed next.
    #
    # `seen`, type `{sha1: Boolean}`, contains entries that have been visited
    # either by the traversal or that are known to be already in the repo set.
    #
    # `clean`, type `{sha1: Boolean}` contains entries that are known to be
    # handled, including all recursive dependencies.  Either the dependencies
    # have been resolved during traversal via the waiting list (see
    # `waitingFor`) below, or the dependencies have been loaded from the
    # `repoSets` collection (also via the waiting list).

    front =
      commits: []
      trees: []
      objects: []


    seen = {}
    @seen[repoId] = seen  # Give spinning `isMember()` access to `seen`.
    clean = {}

    frontSize = ->
      (front.commits.length + front.trees.length + front.objects.length)

    frontIsEmpty = -> (frontSize() == 0)

    pushCommit = (sha1) ->
      seen[sha1] = true
      front.commits.push sha1

    pushTree = (sha1) ->
      seen[sha1] = true
      front.trees.push sha1

    pushObject = (sha1) ->
      seen[sha1] = true
      front.objects.push sha1

    pushEntry = (e) ->
      switch e.type
        when 'commit'
          pushCommit e.sha1
        when 'tree'
          pushTree e.sha1
        when 'object'
          pushObject e.sha1
        else
          nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}

    takeFront = ->
      commits = front.commits
      front.commits = []
      trees = front.trees
      front.trees = []
      objects = front.objects
      front.objects = []
      {commits, trees, objects}

    # The `waitingGraph` maintains nodes that are waiting for their
    # dependencies to get cleared.  `nDeps`, type `{sha1: Number}` contains the
    # number of dependencies that need to be cleared for a node.  `byDep`, type
    # `{sha1: [sha1]}` contains for each dependency a list of nodes that are
    # waiting.
    #
    # New nodes are added with `waitFor(node, deps)`.  If all dependencies are
    # already resolved, the node gets immediately cleared by calling
    # `clearWaiting()`.  Otherwise, it is put into the `waitingGraph`: the
    # uncleared deps are pushed into `byDep`; the number of uncleared deps into
    # `nDeps`.
    #
    # Deps are cleared by calling `clearWaiting`:
    #
    #  - For objects that have no blob.
    #  - For blobs that are not already in the repo set.
    #  - For nodes whose deps have been cleared.
    #
    # Deps are also cleared by fetching entries from the `repoSets` collection
    # and calling `clearWaitingNoWrite`.
    #
    # `clearWaiting()` queues cleared node for insertion into the `repoSets`
    # collection.  The queue is flushed and inserted as bulk documents that
    # contain a whole list of entries in `sha1`.  Using bulk documents keeps
    # the number of DB operations low.  MongoDB will maintain the individual
    # elements of sha1 in the index as it would if they were stored in
    # individual documents.

    waitingGraph =
      nDeps: {}
      byDep: {}

    writeQueue = []

    if optDebugRepoSets then nWaiting = 0
    waitFor = (node, deps) ->
      deps = _.uniq deps
      n = 0
      for d in deps
        unless clean[d]
          n++
          waitingGraph.byDep[d] ?= []
          waitingGraph.byDep[d].push(node)
      if n > 0
        if optDebugRepoSets then nWaiting++
        waitingGraph.nDeps[node] = n
      else
        clearWaiting(node)

    if optDebugRepoSets then nCleared = 0
    clearWaiting = (dep) ->
      if optDebugRepoSets then nCleared++
      writeQueue.push(dep)
      maybeFlush()
      clearWaitingNoWrite(dep)

    # It must be safe to clear the same `dep` multiple times, since sha1s may
    # be places in multiple documents in the `repoSets` collection.
    clearWaitingNoWrite = (dep) ->
      clean[dep] = true
      if (nodes = waitingGraph.byDep[dep])?
        for n in nodes
          waitingGraph.nDeps[n]--
          if waitingGraph.nDeps[n] == 0
            if optDebugRepoSets then nWaiting--
            delete waitingGraph.nDeps[n]
            clearWaiting(n)
        delete waitingGraph.byDep[dep]

    maybeFlush = ->
      if writeQueue.length >= 1000
        flush()

    flush = =>
      lock.refresh()
      @repoSets.insert {
        repoId, ts: target, sha1: writeQueue
      }
      writeQueue = []

    # Limiting query docs to 10k sha1s seem reasonable.  I tried larger query
    # docs (like 100k sha1s) until I hit Mongo's 16 MB limit (somewhere around
    # 330k sha1s).
    #
    # A smaller limit has some performance impact.  I only did a quick test for
    # a larger history that generates around 170k repo set entries.  The
    # timings for a complete update were:
    #
    #  - chunkSize = 100: 150s
    #  - chunkSize = 10000: 130s
    #
    chunked = (arr) ->
      chunkSize = 10000
      for i in [0...arr.length] by chunkSize
        arr[i...(i + chunkSize)]

    # Push the start `entry` and traverse until the front is empty.
    #
    # In each iteration:
    #
    #  - Take the whole front (it only contains unseen entries).
    #  - Bulk fetch the entries (only what's relevant for connectivity).
    #  - Insert the entries in the waiting list.
    #  - Push the unseen dependencies, i.e. parent commits and tree entries to
    #    build the front for the next iteration.
    #  - Fetch existing repo set bits for the new front and clear them in the
    #    waiting list (this will trigger filling the `writeQueue`).
    #  - Filter the new front, rejecting entries for which existing repo bits
    #    have been added.
    #
    # The `blobs` handling is a bit special.  See note below.

    pushEntry entry
    step = 0
    until frontIsEmpty()
      step++
      if optDebugRepoSets
        console.log(
          'DEBUG:',
          'update step', step,
          'frontSize', frontSize(),
          'seen', _.keys(seen).length,
          'clean', _.keys(clean).length,
          'nWaiting', nWaiting,
          'nCleared', nCleared)

      {commits, trees, objects} = takeFront()
      blobs = []

      for chunk in chunked(commits)
        @commits.find({
          _id: {$in: chunk}
        }, {
          fields: {tree: 1, parents: 1}
          transform: null
        }).map (c) ->
          waitFor(c._id, [c.tree].concat(c.parents))
          unless seen[c.tree]
            pushTree c.tree
          for p in c.parents
            unless seen[p]
              pushCommit p

      for chunk in chunked(trees)
        @trees.find({
          _id: {$in: chunk}
        }, {
          fields: {entries: 1}
          transform: null
        }).map (t) ->
          waitFor(t._id, (e.sha1 for e in t.entries))
          for e in t.entries
            unless seen[e.sha1]
              pushEntry e

      for chunk in chunked(objects)
        @objects.find({
          _id: {$in: chunk}
        }, {
          fields: {blob: 1}
          transform: null
        }).map (o) ->
          if o.blob == nullSha1
            # Clear objects without blob immediately.  They must have been
            # dirty when they were push onto the front.
            clearWaiting(o._id)
          else
            # Do not clear blobs immediately, but only queue them.  Then check
            # below which blobs are already in the `repoSets` collection, and
            # clear them.  Finally, clear only the blobs that had not already
            # been in `repoSets`, which queues them for writing.
            waitFor(o._id, [o.blob])
            unless seen[o.blob]
              seen[o.blob] = true
              blobs.push o.blob

      candidates = front.commits
      candidates = candidates.concat(front.trees)
      candidates = candidates.concat(front.objects)
      candidates = candidates.concat(blobs)
      for chunk in chunked(candidates)
        @repoSets.find({
          repoId, ts: target, sha1: {$in: chunk}
        }, {
          fields: {sha1: 1}
        }).map (d) ->
          for s in d.sha1
            seen[s] = true
            clearWaitingNoWrite(s)
      front.commits = (sha1 for sha1 in front.commits when not clean[sha1])
      front.trees = (sha1 for sha1 in front.trees when not clean[sha1])
      front.objects = (sha1 for sha1 in front.objects when not clean[sha1])

      for b in blobs
        unless clean[b]
          clearWaiting(b)

    # Write the remaining cleared entries.
    flush()

    # Delete in-memory info for `isMember()`.  The `repoSets` collection is
    # up-to-date; `isMember()` can get the info from there.
    delete @seen[repoId]

    if optDebugRepoSets
      console.log(
        'DEBUG:',
        'update step', step,
        'frontSize', frontSize(),
        'seen', _.keys(seen).length,
        'clean', _.keys(clean).length,
        'nWaiting', nWaiting,
        'nCleared', nCleared)
      console.log('DEBUG: update final waitingGraph', waitingGraph)


if optDebugRepoSets then Meteor.methods
  # Call from client with
  #
  #   console.log('start', new Date());
  #   Meteor.call('testUpdateMembership'(
  #       {ownerName: ..., repoName: ..., type: 'commit', sha1: ...},
  #       function() {console.log('done', new Date())}));
  #
  testUpdateMembership: (opts) ->
    console.log 'DEBUG: forced update of repo set'
    NogContent.repoSets.repoSets.remove {}
    NogContent.repoSets.updateMembership opts, opts
    console.log 'DEBUG: forced update done'


absHref = (req, path) ->
  if _.isArray path
    path = path.join '/'
  Meteor.absoluteUrl [req.baseUrl[1..], path].join('/')


dbHref = (req, opts, coll, sha1) ->
  absHref req, [opts.ownerName, opts.repoName, 'db', coll, sha1]


createRefRes = (req, opts) ->
  _id:
    refName: opts.refName
    href: dbHref(req, opts, 'refs', opts.refName)
  entry:
    type: 'commit'
    sha1: opts.sha1
    href: dbHref(req, opts, 'commits', opts.sha1)


completeCommitDates = (res, idversion) ->
  switch idversion
    when 0
      res.authorDate = toSimpleISOString(res.authorDate)
      res.commitDate = toSimpleISOString(res.commitDate)
    when 1
      res.authorDate = toISOStringTZ(res.authorDate)
      res.commitDate = toISOStringTZ(res.commitDate)
    else
      nogthrow ERR_LOGIC

completeCommitRes = (res, req, opts) ->
  res.tree =
    sha1: res.tree
    href: dbHref(req, opts, 'trees', res.tree)
  res.parents = for p in res.parents
    sha1: p
    href: dbHref(req, opts, 'commits', p)
  res._id =
    sha1: res._id
    href: dbHref(req, opts, 'commits', res._id)
  return res

class ReposApi
  constructor: (opts) ->
    @store = opts.store

    # Use blob hrefs by default if store has a blobs collection.
    @useBlobHrefs = opts.store.blobs?

  actions_v1: ->
    [

      {
        method: 'POST'
        path: '/'
        action: @post_createRepo
      }

      {
        method: 'POST'
        path: '/:ownerName/:repoName/db/bulk'
        action: @post_bulk
      }

      {
        method: 'GET'
        path: '/:ownerName/:repoName/db/refs/:refName+'
        action: @get_ref
      }
      {
        method: 'GET'
        path: '/:ownerName/:repoName/db/refs'
        action: @get_refs
      }
      {
        method: 'POST'
        path: '/:ownerName/:repoName/db/refs'
        action: @post_createRef
      }
      {
        method: 'PATCH'
        path: '/:ownerName/:repoName/db/refs/:refName+'
        action: @patch_updateRef
      }
      {
        method: 'DELETE'
        path: '/:ownerName/:repoName/db/refs/:refName+'
        action: @delete_ref
      }

      {
        method: 'POST'
        path: '/:ownerName/:repoName/db/stat'
        action: @post_stat
      }

      {
        method: 'GET'
        path: '/:ownerName/:repoName/db/objects/:sha1'
        action: @get_object_v1
      }
      {
        method: 'POST'
        path: '/:ownerName/:repoName/db/objects'
        action: @post_createObject_v1
      }

      {
        method: 'GET'
        path: '/:ownerName/:repoName/db/trees/:sha1'
        action: @get_tree_v1
      }
      {
        method: 'POST'
        path: '/:ownerName/:repoName/db/trees'
        action: @post_createTree_v1
      }

      {
        method: 'GET'
        path: '/:ownerName/:repoName/db/commits/:sha1'
        action: @get_commit_v1
      }
      {
        method: 'POST'
        path: '/:ownerName/:repoName/db/commits'
        action: @post_createCommit_v1
      }


    ]


  # Use `=>` to bind the actions to this instance.
  post_createRepo: (req) =>
    opts = _.pick req.body, 'repoFullName'
    id = @store.createRepo req.auth?.user, opts
    res = @store.repos.findOne id
    res.statusCode = 201
    res._id =
      id: res._id
      href: absHref req, [res.owner, res.name]
    return res


  # The client can control the format version.
  get_object_v1: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName', 'sha1'
    if (format = req.params.query?.format)?
      check format, matchObjectFormatQuery_v1
      {format, fmtversion} = parseFormatQuery_v1 format
    res = @store.getObject req.auth?.user, opts
    fmtversion ?= res._idversion
    @fmtObjectResult(res, {fmtversion})
    if format == 'minimal'
      return res
    else
      return @completeObjectResult_v1(res, req, opts)

  # The client cannot control the format version.  The format always matches
  # the idversion.
  post_createObject_v1: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName'
    opts.content = req.body
    if (format = req.params.query?.format)?
      check format, matchSimpleFormatQuery
    else
      format = 'hrefs'
    sha1 = @store.createObject req.auth?.user, opts
    res = @store.objects.findOne sha1
    @fmtObjectResult res, {fmtversion: res._idversion}
    if format == 'hrefs'
      res = @completeObjectResult_v1(res, req, opts)
    return _.extend {statusCode: 201}, res

  fmtObjectResult: (res, opts) ->
    switch opts.fmtversion
      when 0
        delete res.text
        if _.isNull(res.blob)
          res.blob = nullSha1
      when 1
        delete res.meta.content

  completeObjectResult_v1: (res, req, opts) ->
    if res.blob?
      res.blob = {sha1: res.blob}
      if @useBlobHrefs
        res.blob.href = dbHref(req, opts, 'blobs', res.blob.sha1)
    res._id =
      sha1: res._id
      href: dbHref(req, opts, 'objects', res._id)
    return res


  get_tree_v1: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName', 'sha1'
    if (expand = req.params.query?.expand)?
      check expand, matchNonNegativeIntegerString('in query param `expand`')
      opts.expand = Number(expand)
    if (format = req.params.query?.format)?
      check format, matchTreeFormatQuery_v1
      {format, fmtversion} = parseFormatQuery_v1 format
    if expand? and fmtversion?
      if expand > 0
        nogthrow ERR_PARAM_INVALID, {
          reason: 'Format version suffix can only be used with expand=0.'
        }
    res = @store.getTree req.auth?.user, opts
    @fmtTreeResult_v1 res
    if format == 'minimal'
      return res
    else
      return @completeTreeResult_v1(res, req, opts)

  # The client cannot control the format version.  The format always matches
  # the idversion.
  post_createTree_v1: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName'
    opts.content = req.body?.tree
    if (format = req.params.query?.format)?
      check format, matchSimpleFormatQuery
    else
      format = 'hrefs'
    sha1 = @store.createTree req.auth?.user, opts
    res = @store.trees.findOne sha1
    if format == 'hrefs'
      res = @completeTreeResult_v1(res, req, opts)
    return _.extend {statusCode: 201}, res

  fmtTreeResult_v1: (res) ->
    fmtEntries = (tree) =>
      for e in tree.entries
        if isObjectContent(e)
          @fmtObjectResult e, {fmtversion: e._idversion}
        else if isTreeContent(e)  # Recurse into expanded tree.
          fmtEntries(e)
    fmtEntries(res)

  # Handle trees that are recursively expanded to any level.
  completeTreeResult_v1: (res, req, opts) ->
    completeEntries = (tree) =>
      tree._id =
        sha1: tree._id
        href: dbHref(req, opts, 'trees', tree._id)
      for e in tree.entries
        if isObjectContent(e)
          @completeObjectResult_v1(e, req, opts)
        else if isTreeContent(e)  # Recurse into expanded tree.
          completeEntries(e)
        else switch e.type  # Add href to collapsed entries.
          when 'object'
            e.href = dbHref(req, opts, 'objects', e.sha1)
          when 'tree'
            e.href = dbHref(req, opts, 'trees', e.sha1)
          else
            nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
    completeEntries(res)
    return res


  # The client can control the format version to ensure that it receives a
  # format that it supports.
  get_commit_v1: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName', 'sha1'
    if (format = req.params.query?.format)?
      check format, matchCommitFormatQuery_v1
      {format, fmtversion} = parseFormatQuery_v1 format
    res = @store.getCommit req.auth?.user, opts
    fmtversion ?= res._idversion
    if res._idversion > fmtversion
      nogthrow ERR_API_VERSION, {
        reason: 'Cannot format the commit to the requested format version.'
      }
    completeCommitDates(res, fmtversion)
    if format == 'minimal'
      return res
    else
      return completeCommitRes(res, req, opts)

  # The format query does not support a `.vX` suffix.  It always returns the
  # format that matches the idversion.
  post_createCommit_v1: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName'
    opts.content = req.body
    if (format = req.params.query?.format)?
      check format, matchSimpleFormatQuery
    else
      format = 'hrefs'
    sha1 = @store.createCommit req.auth?.user, opts
    res = @store.commits.findOne sha1
    completeCommitDates(res, res._idversion)
    if format == 'hrefs'
      res = completeCommitRes(res, req, opts)
    return _.extend {statusCode: 201}, res

  post_bulk: (req) =>
    check req.body, {entries: [Object]}
    opts = _.pick req.params, 'ownerName', 'repoName'
    user = req.auth?.user
    entries = for e, idx in req.body.entries
      opts.content = e
      if isObjectContent(e)
        {type: 'object', sha1: @store.createObject(user, opts)}
      else if isTreeContent(e)
        {type: 'tree', sha1: @store.createTree(user, opts)}
      else if isCommitContent(e)
        {type: 'commit', sha1: @store.createCommit(user, opts)}
      else if e.copy?
        @store.copyEntry user, opts
      else
        nogthrow ERR_PARAM_MALFORMED, {
            reason: "Failed to determine type for entry number #{idx}."
          }
    return {statusCode: 201, entries}


  get_ref: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName', 'refName'
    res = @store.getRef req.auth?.user, opts
    opts.sha1 = res
    return createRefRes req, opts

  get_refs: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName'
    res = @store.getRefs req.auth?.user, opts
    items = for name, sha1 of res
      opts.refName = name
      opts.sha1 = sha1
      createRefRes req, opts
    # Return a format that could be extended to support pagination.
    return {
      count: items.length
      items: items
    }

  post_createRef: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName'
    opts.refName = req.body.refName
    opts.old = null
    opts.new = req.body.sha1
    @store.updateRef req.auth?.user, opts
    opts.sha1 = opts.new
    return _.extend {statusCode: 201}, createRefRes(req, opts)

  patch_updateRef: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName', 'refName'
    _.extend opts, _.pick(req.body, 'old', 'new')
    @store.updateRef req.auth?.user, opts
    opts.sha1 = opts.new
    return createRefRes req, opts

  delete_ref: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName', 'refName'
    opts.old = req.body?.old
    opts.new = null
    @store.updateRef req.auth?.user, opts
    return {statusCode: 204}

  post_stat: (req) =>
    opts = _.pick req.params, 'ownerName', 'repoName'
    opts.entries = req.body.entries
    return {
      entries: @store.stat req.auth?.user, opts
    }


# See comment at `RepoSets` for intro do membership test.
repoSets = new Mongo.Collection 'nogcontent.repo_sets'


init_server = ->
  deps = {users: Meteor.users}
  _.extend deps, _.pick(
      NogContent,
      'repos', 'commits', 'trees', 'objects', 'blobs', 'checkAccess',
      'deletedRepos'
    )
  if Meteor.settings.optStrictRepoMembership
    deps.repoSets = repoSets
    NogContent.repoSets = deps.repoSets = new CachedRepoSets(
      new RepoSets(deps), {maxCacheAge_s: repoSetsExpireAfter_s}
    )
    if (p = Package['nog-blob'])?
      p.NogBlob.configure {repoSets: deps.repoSets}
      console.log '[nog-content] configuring nog-blob to check repo membership.'
  else
    if (p = Package['nog-blob'])?
      p.NogBlob.configure {repoSets: false}
  NogContent.store = new Store deps
  NogContent.api.repos = new ReposApi {store: NogContent.store}


init_server()


# XXX Export `Store` and `ReposApi` for package `nog-sync-spike`.  We will
# reconsider these exports when implementing the production `nog-sync` code.

NogContent.init_server = init_server
NogContent.Store = Store
NogContent.createContentStore = createContentStore
NogContent.ReposApi = ReposApi

NogContentTest.Store = Store
NogContentTest.ReposApi = ReposApi
NogContentTest.RepoSets = RepoSets
NogContentTest.collectionContainsCache = collectionContainsCache
NogContentTest.create = create
