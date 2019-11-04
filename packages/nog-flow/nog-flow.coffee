{
  ERR_CONTENT_MISSING
  ERR_LOGIC
  ERR_PARAM_INVALID
  ERR_REF_MISMATCH
  ERR_REPO_MISSING
  ERR_UNKNOWN
  nogthrow
} = NogError

{
  checkAccess
} = NogAccess

{
  strip
} = NogContent


config =
  defaultRuntimeSettings:
    maxHeartbeatInterval_s: 30 * 60
    maxTotalDuration_s: 60 * 60
    maxMem_MB: 8 * 1024
  updateKindsMaxIdPartitions: (
    Meteor.settings.cluster?.maxIdPartitions?.updateKinds ?
    Meteor.settings.cluster?.maxIdPartitions?.default ?
    1
  )


NogFlow =
  call: {}


defMethod = (name, func) ->
  qualname = 'NogFlow.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogFlow.call[name] = (args...) -> Meteor.call qualname, args...


NULL_SHA1 = '0000000000000000000000000000000000000000'


# ISO UTC without fractional seconds.
toSimpleISOString = (d) ->
  d.utc().format('YYYY-MM-DD[T]HH:mm:ss[Z]')

# ISO UTC without ':'.
toURLSafeISOString = (d) ->
  d.utc().format('YYYY-MM-DD[T]HHmmss[Z]')


entryContent = (e) ->
  unless e?
    return e
  if e.type == 'object'
    NogContent.objects.findOne e.sha1
  else if e.type == 'tree'
    NogContent.trees.findOne e.sha1
  else
    e


iskind = (entry, kind) -> _.isObject(entry.meta[kind])


# Maintain a set of kinds on the repo documents.  Update the kinds when master
# changes.
if Meteor.isServer then Meteor.startup ->
  kindsFromCommitId = (commitId) ->
    kinds = []
    commit = NogContent.commits.findOne(commitId, {fields: {tree: 1}})
    root = NogContent.trees.findOne(commit.tree)
    if iskind root, 'workspace'
      kinds.push 'workspace'
      for e in root.entries
        if e.type != 'tree'
          continue
        e = NogContent.trees.findOne(e.sha1)
        for k in ['datalist', 'programs']
          if iskind e, k
            kinds.push k
    if iskind root, 'programRegistry'
      kinds.push 'programRegistry'
    if iskind root, 'catalog'
      kinds.push 'catalog'
    kinds

  updateKinds = Meteor.bindEnvironment (doc) ->
    if not (master = doc.refs['branches/master'])?
      return
    if master == NULL_SHA1
      return
    # Increase `v` to force reindexing.
    kindsCacheKey = {master, v: 5}
    if _.isEqual(doc.kindsCacheKey, kindsCacheKey)
      return
    kinds = kindsFromCommitId(master)
    console.log 'Updating kinds', doc._id, kinds
    NogContent.repos.update doc._id, {$set: {kindsCacheKey, kinds}}

  # Defer the update for each repo (see underscore doc for `debounce`) to
  # efficiently support a sequence of commits.

  wait_ms = 10 * 1000
  updateKindsTable = {}
  updateKindsDeferred = (doc) ->
    updateKindsTable[doc._id] ?= _.debounce(updateKinds, wait_ms)
    updateKindsTable[doc._id](doc)

  observers = {}

  partition = new NogCluster.IdPartition {
    name: 'updateKinds'
    max: config.updateKindsMaxIdPartitions
  }
  partition.onacquire = (part) ->
    console.log "
      [nog-flow] Start updating kinds for repos #{part.selHuman}.
    "
    sel = {_id: part.sel}
    opts =
      fields:
        'refs.branches/master': 1
        kindsCacheKey: 1
        kinds: 1
    observers[part.begin] = NogContent.repos.find(sel, opts).observe {
      added: updateKindsDeferred
      changed: updateKindsDeferred
    }
  partition.onrelease = (part) ->
    console.log "
      [nog-flow] Stop updating kinds for repos #{part.selHuman}.
    "
    observers[part.begin].stop()
    delete observers[part.begin]
  NogCluster.registerHeartbeat(partition)



if Meteor.isServer
  publishDropdownTargets = (userId, kinds) ->
    unless (user = Meteor.users.findOne userId)?
      return null
    owner = user.username
    unless NogAccess.testAccess(user, 'nog-content/get', {ownerName: owner})
      return null
    NogContent.repos.find {owner, kinds: {$all: kinds}}

  Meteor.publish 'targetDatalists', ->
    publishDropdownTargets @userId, ['workspace', 'datalist']

  Meteor.publish 'targetProgramWorkspaces', ->
    publishDropdownTargets @userId, ['workspace', 'programs']

  Meteor.publish 'targetProgramRegistries', ->
    publishDropdownTargets @userId, ['programRegistry']


matchSimpleName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid simple name.'
  true


matchRepoFullName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ / [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid repo full name.'
  true


matchSha1 = Match.Where (x) ->
  check x, String
  if not (x.match /^[0-9a-f]{40}$/)?
    throw new Match.Error 'not a sha1'
  true


matchEntryType = Match.Where (x) ->
  check x, String
  if (x is 'object') or (x is 'tree')
    true
  else
    throw new Match.Error 'not an entry type'


matchRootKind = Match.Where (x) ->
  check x, String
  switch x
    when 'workspace', 'programRegistry', 'fileRepo'
      true
    else
      throw new Match.Error 'invalid root kind'


matchSubtreeKind = Match.Where (x) ->
  check x, String
  switch x
    when 'datalist', 'programs', 'log', 'jobs', 'results'
      true
    else
      throw new Match.Error 'invalid subtree kind'


commitTree = (user, opts) ->
  {
    ownerName, repoName, tree, subject, message, parents, oldCommit, meta
  } = opts
  message ?= ''
  oldCommit ?= parents[0]
  meta ?= {}
  store = NogContent.store
  treeId = store.createTree user, {ownerName, repoName, content: tree}
  commitId = store.createCommit user, {
      ownerName, repoName, content: {
        subject, message, parents, tree: treeId, meta
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: oldCommit
    new: commitId
  }


defMethod 'addProgramToRegistry', (opts) ->
  check opts,
    src:
      ownerName: matchSimpleName
      repoName: matchSimpleName
      sha1: matchSha1
      path: String
      commitId: matchSha1
    dst:
      ownerName: matchSimpleName
      repoName: matchSimpleName
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts.dst

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  master = store.getCommit user, {ownerName, repoName, sha1: master}

  # Low-level read ok, since methods above checked read access.
  root = entryContent({type: 'tree', sha1: master.tree})

  expandPrograms = (root) ->
    for e, idx in root.entries
      if e.type == 'object'
        continue
      else if e.type == 'tree'
        programs = entryContent(e)
      else
        nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
      if iskind programs, 'programs'
        strip(root)
        strip(programs)
        root.entries[idx] = programs
        return programs
    nogthrow ERR_UNKNOWN, {reason: 'Failed to find programs tree.'}

  programs = expandPrograms root

  entry = NogContent.store.copyEntry user, {
      ownerName, repoName, content: {
        copy:
          repoFullName: "#{opts.src.ownerName}/#{opts.src.repoName}"
          sha1: opts.src.sha1
          type: 'tree'
      }
    }
  programs.entries.unshift entry

  name = entryContent(entry).name
  commitTree user, {
      ownerName, repoName,
      tree: root,
      subject: "
          Add program `#{name}` from repo
          `#{opts.src.ownerName}/#{opts.src.repoName}`
        "
      parents: [master._id]
    }



defMethod 'getPackageVersion', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    packageName: matchSimpleName
  unless Meteor.isServer
    return

  user = Meteor.user()
  AA_GET = 'nog-content/get'
  aopts = _.pick(opts, 'ownerName', 'repoName')
  checkAccess user, AA_GET, aopts
  {ownerName, repoName, packageName} = opts

  unless (repo = NogContent.repos.findOne {owner: ownerName, name: repoName})?
    nogthrow ERR_REPO_MISSING

  resolved = NogContent.resolveRefTreePath(
      repo, "master/programs/#{packageName}"
    )
  unless resolved?
    nogthrow ERR_CONTENT_MISSING {
        reason: 'Failed to resolve program package path.'
      }
  unless iskind(resolved.contentPath[0].content, 'programs')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'First level entry `programs` is not of kind `programs`.'
      }
  pkg = resolved.contentPath[1].content
  unless iskind(pkg, 'package')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Failed to find package container.'
      }
  unless (latest = entryContent pkg.entries[0])?
    nogthrow ERR_PARAM_INVALID, {
        reason: 'There is no latest version.'
      }
  unless iskind(latest, 'package') and iskind(latest, 'program')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Failed to find latest program version entry.'
      }

  return {
    ownerName
    repoName
    packageName
    version: _.extend {}, latest.meta.package.version, {sha1: latest._id}
    commitId: resolved.commitId
  }


defMethod 'updatePackageDependency', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    package:
      numericPath: [Number]
    dependency:
      name: matchSimpleName
      oldSha1: matchSha1
      newSha1: matchSha1
    origin:
      ownerName: matchSimpleName
      repoName: matchSimpleName
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  unless master == opts.commitId
    nogthrow ERR_REF_MISMATCH, {
        reason: 'The client commit id does not match `branches/master`.'
      }
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent {type: 'tree', sha1: master.tree}

  origin = NogContent.store.copyEntry user, {
      ownerName, repoName, content: {
        copy:
          repoFullName: "#{opts.origin.ownerName}/#{opts.origin.repoName}"
          sha1: opts.dependency.newSha1
          type: 'tree'
      }
    }

  # `numericPath` is expected to point to the package parent tree.  Append '0'
  # to get latest version.
  contentPath = expandNumericPath root, opts.package.numericPath.concat [0]
  len = contentPath.length
  unless len >= 2
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Numeric path is too short (expected at least 2 levels).'
      }
  pkg = contentPath[len - 2]
  unless iskind(pkg, 'package')
    nogthrow ERR_PARAM_INVALID, {
        reason: '`numericPath` does not point to entry of kind `package`.'
      }
  leaf = contentPath[len - 1]
  unless iskind(leaf, 'program') and iskind(leaf, 'package')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'First child entry is not of kind `package` and `program`.'
      }

  deepclone = (e) -> JSON.parse(JSON.stringify(e))

  updateFrozen = (tree, frozen) ->
    for f, idx in tree.meta.package.frozen
      if f.name == frozen.name
        tree.meta.package.frozen[idx] = frozen
        return 1
    return 0

  updateEntry = (tree, sha1, entry) ->
    for e, idx in tree.entries
      if e.sha1 == sha1
        tree.entries.splice(idx, 1, entry)
        return 1
    return 0

  originContent = entryContent origin

  latest = deepclone(leaf)
  strip(latest)
  datetag = toSimpleISOString moment()
  latest.name = pkg.name + '@' + datetag
  latest.meta.description = 'Workspace program'

  # Update frozen version in package meta.
  frozen = _.extend {}, originContent.meta.package.version, {
      sha1: origin.sha1
      name: opts.dependency.name
    }
  if updateFrozen(latest, frozen) != 1
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Failed to update frozen version.'
      }

  # Update child entry to point to latest upstream.
  if updateEntry(latest, opts.dependency.oldSha1, origin) != 1
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Failed to update dependency entry.'
      }

  # Insert new version in package parent tree.
  pkg.entries.unshift latest

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: 'Update package depenceny'
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: opts.commitId
    new: commit
  }


defMethod 'addWorkspaceKindTree', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    kind: matchSubtreeKind
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})

  haveKind = (tree, kind) ->
    for e in tree.entries
      e = entryContent(e)
      if iskind(e, kind)
        return true
    false

  if haveKind(root, opts.kind)
    return

  strip(root)
  meta = {}
  meta[opts.kind] = {}
  root.entries.push {
      name: opts.kind
      meta
      entries: []
    }

  commitTree user, {
      ownerName, repoName,
      tree: root,
      subject: "Add #{opts.kind} tree"
      parents: [master._id]
    }


defMethod 'createWorkspaceRepo', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    rootKinds: [matchRootKind]
    subtrees: [matchSubtreeKind]
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts

  store.createRepo user, {repoFullName: "#{ownerName}/#{repoName}"}

  root = {
    name: "#{repoName} root"
    meta: {}
    entries:[]
  }
  for k in opts.rootKinds
    root.meta[k] = {}
  for k in opts.subtrees
    subtree =
        name: k
        meta: {}
        entries: []
    subtree.meta[k] = {}
    root.entries.push subtree

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Create workspace"
        message: ''
        parents: []
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: NULL_SHA1
    new: commit
  }


expandNumericPath = (root, numericPath) ->
  contentPath = []
  strip(root)
  parent = root
  level = 0
  while (idx = numericPath.shift())?
    unless (e = parent.entries[idx])?
      nogthrow ERR_CONTENT_MISSING, {
          reason: "Numeric path index out-of-bounds at level #{level}."
        }
    c = entryContent(e)
    strip(c)
    parent.entries[idx] = c
    parent = c
    contentPath.push(c)
    level++
  contentPath


expandNumericPathLeaf = (root, numericPath) ->
  contentPath = expandNumericPath root, numericPath
  len = contentPath.length
  if len > 0
    contentPath[len - 1]
  else
    root


defMethod 'updateProgramParams', (opts) ->
  unless Meteor.isServer
    return
  check opts.params, Object
  opts.set = {params: opts.params}
  delete opts.params
  opts.subject = "Change program params"
  updateProgramMeta opts


defMethod 'updateProgramRuntime', (opts) ->
  unless Meteor.isServer
    return
  check opts.runtime, Object
  opts.set = {runtime: opts.runtime}
  delete opts.runtime
  opts.subject = "Change program runtime settings"
  updateProgramMeta opts


if Meteor.isServer then updateProgramMeta = (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    numericPath: [Number]
    set:
      params: Match.Optional Object
      runtime: Match.Optional Object
    subject: String

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  unless master == opts.commitId
    nogthrow ERR_REF_MISMATCH, {
        reason: 'The client commit id does not match `branches/master`.'
      }
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})

  contentPath = expandNumericPath root, opts.numericPath
  len = contentPath.length
  unless len >= 4
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Numeric path is too short (expected at least 4 levels).'
      }
  unless iskind(contentPath[0], 'programs')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'First level subtree is not of kind `programs`.'
      }
  leaf = contentPath[len - 1]
  unless iskind(leaf, 'program')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Leaf entry is not of kind `program`.'
      }
  instance = contentPath[len - 2]
  unless iskind(instance, 'program') and iskind(instance, 'package')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Parent of leaf is not of kind `program` and `package`.'
      }
  pkg = contentPath[len - 3]
  unless iskind(pkg, 'package')
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Second level above leaf is not of kind `package`.'
      }

  deepclone = (e) -> JSON.parse(JSON.stringify(e))

  # A better implementation would probably use semver semantics.
  nextVersion = (v) -> {date: toSimpleISOString moment()}

  # Insert a copy at index 1 to keep the current version.
  pkg.entries.splice(1, 0, deepclone(instance))
  # Then modify the head version.
  version = nextVersion(instance.meta.package.version)
  name = pkg.name + '@' + version.date
  instance.name = name
  instance.meta.description = pkg.meta.description +
    " (version #{version.date})"
  instance.meta.package.version = version

  for k, v of opts.set
    leaf.meta.program[k] = v

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: name + ': ' + opts.subject
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: opts.commitId
    new: commit
  }

  namePath = (c.name for c in contentPath).join('/')
  return {
    ownerName, repoName, refTreePath: 'master/' + namePath
  }


defMethod 'runProgram', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    sha1: matchSha1
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  unless master == opts.commitId
    nogthrow ERR_REF_MISMATCH, {
        reason: 'The client commit id does not match `branches/master`.'
      }
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})
  program = store.getTree user, {ownerName, repoName, sha1: opts.sha1}

  expandJobs = (root) ->
    strip(root)
    for e, idx in root.entries
      if e.type == 'object'
        continue
      else if e.type == 'tree'
        jobs = entryContent(e)
      else
        nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
      if iskind jobs, 'jobs'
        strip(jobs)
        root.entries[idx] = jobs
        return jobs
    return null

  unless (jobs = expandJobs(root))?
    jobs =
      name: 'jobs'
      meta:
        jobs: {}
      entries: []
    root.entries.push jobs

  unless iskind(program, 'package')
    nogthrow ERR_PARAM_INVALID, {reason: 'Tree is not a program package.'}
  unless (current = program.entries[0])?
    nogthrow ERR_CONTENT_MISSING, {reason: 'Missing current program instance.'}
  current = entryContent(current)
  unless iskind(current, 'package') and iskind(current, 'program')
    nogthrow ERR_CONTENT_MISSING, {
        reason: '
            First child of program package is not of kind `program` and
            `package`.
          '
      }

  checkProgramTrust {
      ownerName, repoName, current
    }

  jobId = toURLSafeISOString(moment()) + '-' + Random.id()
  job =
    name: jobId
    entries: []
    meta:
      job:
        id: jobId
        status: 'pending'
        program:
          name: current.meta.package.name
          sha1: current._id
  jobs.entries.unshift job

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Create job #{jobId}"
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
      ownerName, repoName
      refName: 'branches/master'
      old: opts.commitId
      new: commit
    }

  findRuntimeSettings = (current) ->
    current = entryContent current
    unless current.entries?
      return null
    for e in current.entries
      content = entryContent e
      if content.name == 'runtime'
        return content.meta.program.runtime
    return null

  runtime = findRuntimeSettings(current) ? config.defaultRuntimeSettings

  NogExec.submit {
      jobId
      ownerId: user._id
      ownerName
      repoName
      commitId: commit
      runtime
    }

  return {
    ownerName, repoName,
    refTreePath: ['master', jobs.name, job.name].join('/')
  }


NogFlow.metaChangeViolation = (oldMeta, newMeta) ->
  # Keep `versions` in the blacklist, although the implementation has been
  # removed.  But we used it for a while, so better prevent users from adding
  # it to avoid potential confusion.
  blacklistMetaFields = [
    'workspace', 'datalist', 'programs', 'jobs', 'job', 'package', 'program',
    'programRegistry', 'log', 'history', 'versions'
  ]
  for b in blacklistMetaFields
    if ((b of oldMeta) or (b of newMeta))
      unless _.isEqual(oldMeta[b], newMeta[b])
        return "The meta field `#{b}` must not be modified."
  return null


defMethod 'setMeta', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    numericPath: [Number]
    meta: Object
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName, meta} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  unless master == opts.commitId
    nogthrow ERR_REF_MISMATCH, {
        reason: 'The client commit id does not match `branches/master`.'
      }
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})

  contentPath = expandNumericPath root, opts.numericPath
  len = contentPath.length
  if len > 0
    leaf = contentPath[len - 1]
  else
    leaf = root
  if (e = NogFlow.metaChangeViolation(leaf.meta, meta))?
    nogthrow ERR_PARAM_INVALID, {reason: e}
  leaf.meta = meta

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Update meta of '#{leaf.name}'"
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: opts.commitId
    new: commit
  }


# `checkProgramTrust()` walks over the program tree and throws if it is not
# trusted.  Trust is checked for all entries of kind `program` that contain
# `platform`, `code`, or `args`.  `params` are ignored to allow untrusted
# owners to configure a program.
#
# Trust is established in `findTrust()`:
#
#  - If any of the parents is trusted.
#  - If a frozen package dependency origin is in a trusted repo.
#  - If a package is in a trusted registry.
#
# XXX Consider replacing static whitelists by information that is stored in the
# database.  Ideas: `role:trusteddevs`, config doc with admin UI.


# Check settings and compile into `RegExp` instances.
compileTrustSettings = (cfg) ->
  check cfg,
    repoWhitelist: [String]
    registryWhitelist: [matchRepoFullName]
  return {
    repoWhitelist: (new RegExp(e) for e in cfg.repoWhitelist)
    registryWhitelist: cfg.registryWhitelist
  }


if Meteor.isServer
  trustConfig = compileTrustSettings Meteor.settings.programTrust


isTrustedRepo = (repoFullName) ->
  for w in trustConfig.repoWhitelist
    if (repoFullName.match w)?
      return true
  return false


checkProgramTrust = (opts) ->
  {ownerName, repoName, current} = opts
  current = entryContent current

  # `trustSha1s[sha1]` == `true` if trust has been found for an entry.
  trustSha1s = {}

  findChildTreeByName = (tree, part) ->
    for e in tree.entries
      unless e.type == 'tree'
        continue
      content = entryContent e
      if content.name == part
        return content
    return null

  existsChildSha1 = (tree, sha1) ->
    for e in tree.entries
      if e.sha1 == sha1
        return true
    return false

  existsPackagePath = (opts) ->
    {repoFullName, path, sha1} = opts
    [owner, name] = repoFullName.split '/'
    r = NogContent.repos.findOne {
        owner, name, 'refs.branches/master': {$exists: true}
      }, {
        fields: {'refs.branches/master': 1}
      }
    unless r?
      return false
    unless (master = r.refs?['branches/master'])?
      return false
    unless (commit = NogContent.commits.findOne master)?
      return false
    tree = entryContent {type: 'tree', sha1: commit.tree}
    while (part = path.shift())?
      unless (tree = findChildTreeByName tree, part)?
        return false
    return existsChildSha1 tree, sha1

  isInTrustedRegistry = (entry) ->
    unless (pkg = entry.meta.package)?
      return false
    for w in trustConfig.registryWhitelist
      opts = {
        repoFullName: w
        path: ['programs', pkg.name]
        sha1: entry._id
      }
      if existsPackagePath opts
        return true
    return false

  findDepsTrust = (deps, frozen) ->
    origin = {}
    for d in deps
      origin[d.name] = d.repoFullName
    for f in frozen
      unless (o = origin[f.name])?
        continue
      unless isTrustedRepo o
        continue
      opts = {
        repoFullName: o
        path: ['programs', f.name]
        sha1: f.sha1
      }
      if existsPackagePath opts
        trustSha1s[f.sha1] = true

  findTrust = (entry, path) ->
    entry = entryContent entry
    if _.any(trustSha1s[p] for p in path)
      trustSha1s[entry._id] = true
    unless (pkg = entry.meta.package)?
      return
    if (deps = pkg.dependencies)? and (frozen = pkg.frozen)?
      findDepsTrust deps, frozen
    if trustSha1s[entry._id]
      return
    if isInTrustedRegistry entry
      trustSha1s[entry._id] = true

  checkTrust = (entry) ->
    entry = entryContent entry
    unless (program = entry.meta.program)?
      return
    if trustSha1s[entry._id]?
      return
    needstrust = ['platform', 'code', 'args']
    for n in needstrust
      if program[n]?
        # FIXME: create better error spec.
        nogthrow ERR_PARAM_INVALID, {
            reason: "Untrusted program #{n} in #{entry.name}."
          }

  walk = (tree, path) ->
    tree = entryContent tree
    findTrust tree, path
    checkTrust tree
    path = _.clone path
    path.push tree._id
    for e in tree.entries
      switch e.type
        when 'object'
          findTrust e, path
          checkTrust e
        when 'tree'
          walk e, path
        else
          nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}

  walk current, []


if Meteor.isServer
  Meteor.publish 'workspaceRepo', (opts) ->
    check opts,
      ownerName: matchSimpleName
      repoName: matchSimpleName
    aopts = _.pick(opts, 'ownerName', 'repoName')
    unless NogAccess.testAccess(@userId, 'nog-content/get', aopts)
      return null
    NogContent.repos.find {
        owner: opts.ownerName
        name: opts.repoName
      }


if Meteor.isServer
  Meteor.publish 'workspaceCommit', (opts) ->
    check opts,
      ownerName: matchSimpleName
      repoName: matchSimpleName
      commitId: matchSha1

    {ownerName, repoName, commitId} = opts

    # Access check: `getCommit` will throw if access is denied.
    NogContent.store.getCommit @userId, {ownerName, repoName, sha1: commitId}

    # Publish raw, untransformed docs, since the transform will be applied at
    # the client.
    RAW = {transform: false}

    unless (commit = NogContent.commits.findOne(commitId, RAW))?
      return
    @added 'commits', commitId, commit

    isPublished =
      trees: {}
      objects: {}

    addTree = (id) =>
      if not (tree = NogContent.trees.findOne(id, RAW))?
        return
      if not isPublished.trees[id]
        @added 'trees', id, tree
        isPublished.trees[id] = true
      for e in tree.entries
        switch e.type
          when 'object'
            if not isPublished.objects[e.sha1]
              obj = NogContent.objects.findOne(e.sha1, RAW)
              @added 'objects', obj._id, obj
              isPublished.objects[obj._id] = true
            else
          when 'tree'
            addTree e.sha1
          else
            console.log "Unknown type #{e.type}."

    treeId = commit.tree
    addTree treeId

    @ready()


share.NogFlow = NogFlow
