# `NogFiles` is the global API object for file-browsing-related functions.
NogFiles = {}

{
  ERR_REF_MISMATCH,
  ERR_LOGIC,
  ERR_CONTENT_MISSING,
  ERR_PARAM_INVALID,
  ERR_UNKNOWN,
  nogthrow
} = NogError

NULL_SHA1 = '0000000000000000000000000000000000000000'

# Registry for file entry representations.
#
# Specs are added to the front of the specs list, so that later specs win.
# This can be used to override repr specs from the main app, because it is
# initialized after the packages.

entryReprSpecs = []

NogFiles.registerEntryRepr = (spec) -> entryReprSpecs.unshift spec

NogFiles.entryView = (treeCtx) ->
  for r in entryReprSpecs
    if (t = r.view(treeCtx))?
      return t
  return null

NogFiles.entryIcon = (entryCtx) ->
  for r in entryReprSpecs
    if (t = r.icon(entryCtx))?
      return t
  return null

NogFiles.treePermissions = (treeCtx) ->
  for r in entryReprSpecs
    if (t = r.treePermissions?(treeCtx))?
      return t
  return null


# Use nog-access if available (weak dependency).  Do not print messages on the
# client.
if Meteor.isClient
  if (p = Package['nog-access'])?
    NogFiles.testAccess = p.NogAccess.testAccess
  else
    NogFiles.testAccess = -> true

  NogFiles.configure = (cfg) ->
    cfg ?= {}
    for k in ['testAccess']
      if cfg[k]?
        NogFiles[k] = cfg[k]


iskind = (entry, kind) -> _.isObject(entry.meta[kind])


# ISO UTC without fractional seconds.
toSimpleISOString = (d) ->
  d.utc().format('YYYY-MM-DD[T]HH:mm:ss[Z]')


matchSimpleName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid simple name.'
  true


matchSha1 = Match.Where (x) ->
  check x, String
  if not (x.match /^[0-9a-f]{40}$/)?
    throw new Match.Error 'not a sha1'
  true


entryContent = (e) ->
  unless e?
    return e
  if e.type == 'object'
    NogContent.objects.findOne e.sha1
  else if e.type == 'tree'
    NogContent.trees.findOne e.sha1
  else
    e

{
  strip
} = NogContent


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


NogFiles.call = {}

defNogFilesMethod = (name, func) ->
  qualname = 'NogFiles.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogFiles.call[name] = (args...) -> Meteor.call qualname, args...


defNogFilesMethod 'addBlobToDatalist', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    numericPath: [Number]
    name: String
    blob: matchSha1
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName, name, blob, numericPath} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})
  contentPath = expandNumericPath root, opts.numericPath
  len = contentPath.length
  if len > 0
    leaf = contentPath[len - 1]
  else
    leaf = root
  leaf.entries.unshift {
      name
      blob
      meta:
        description: "Upload from local file '#{name}'."
    }

  # FIXME: move the repo set handling to `NogBlob.uploadFile()`.
  unless (NogBlob.blobs.findOne blob)?
    nogthrow ERR_CONTENT_MISSING, {reason: "The blob is missing."}
  NogBlob.repoSets.updateMembership {
      ownerName, repoName
    }, {
      type: 'blob', sha1: blob
    }

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Upload of local file '#{name}'"
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: master._id
    new: commit
  }


defNogFilesMethod 'renameChildren', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    numericPath: [Number]
    children: [{index: Number, newName: String}]
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName, children} = opts

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

  for c in children
    content = entryContent leaf.entries[c.index]
    strip(content)
    content.name = c.newName
    leaf.entries[c.index] = content

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Rename children of #{leaf.name}"
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


defNogFilesMethod 'addSubtree', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    numericPath: [Number]
    folderName: String
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

  contentPath = expandNumericPath root, opts.numericPath
  len = contentPath.length
  if len > 0
    leaf = contentPath[len - 1]
  else
    leaf = root

  subtree =
    name: opts.folderName
    meta: {}
    entries: []
  leaf.entries.push subtree

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
    ownerName, repoName, content: {
      subject: "Add subtree #{opts.folderName} to #{leaf.name}"
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


defNogFilesMethod 'moveInRepo', (opts) ->
  check opts,
    repo:
      ownerName: matchSimpleName
      repoName: matchSimpleName
      commitId: matchSha1
    src:
      numericPath: [Match.Integer]
      children: [Match.Integer]
    dst:
      numericPath: [Match.Integer]
      index: Match.Integer
  if _.isEqual(opts.src.numericPath, opts.dst.numericPath)
    nogthrow ERR_PARAM_INVALID, {
        reason: 'Source and destination numericPath are equal.'
      }

  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName, commitId} = opts.repo
  {src, dst} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  unless master == commitId
    nogthrow ERR_REF_MISMATCH, {
        reason: 'The client commit id does not match `branches/master`.'
      }
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})

  # Expand src first.  Then collect entries before expanding dst to ensure that
  # the entries are still collapsed.  Then expand dst before changing, so that
  # the numeric paths are valid.
  srcTree = expandNumericPathLeaf root, src.numericPath
  move = []
  for i in src.children
    move.push srcTree.entries[i]
  dstTree = expandNumericPathLeaf root, dst.numericPath

  # Remove back to front to keep indices valid.
  descending = (a, b) -> b - a
  for i in src.children.sort(descending)
    srcTree.entries.splice(i, 1)

  # Insert elements in order as specified in `children`.
  dstTree.entries[dst.index...dst.index] = move

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Moved entries from `#{srcTree.name}` to `#{dstTree.name}`"
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: commitId
    new: commit
  }


defNogFilesMethod 'copyInRepo', (opts) ->
  check opts,
    repo:
      ownerName: matchSimpleName
      repoName: matchSimpleName
      commitId: matchSha1
    src:
      numericPath: [Match.Integer]
      children: [Match.Integer]
    dst:
      numericPath: [Match.Integer]
      index: Match.Integer
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName, commitId} = opts.repo
  {src, dst} = opts

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  unless master == commitId
    nogthrow ERR_REF_MISMATCH, {
        reason: 'The client commit id does not match `branches/master`.'
      }
  master = store.getCommit user, {ownerName, repoName, sha1: master}
  root = entryContent({type: 'tree', sha1: master.tree})

  # Expand src first.  Then collect entries before expanding dst to ensure that
  # the entries are still collapsed.  Then expand dst before changing, so that
  # the numeric paths are valid.
  srcTree = expandNumericPathLeaf root, src.numericPath
  move = []
  for i in src.children
    move.push srcTree.entries[i]
  dstTree = expandNumericPathLeaf root, dst.numericPath
  # Insert elements in order as specified in `children`.
  dstTree.entries[dst.index...dst.index] = move

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Copied entries from `#{srcTree.name}` to `#{dstTree.name}`"
        message: ''
        parents: [master._id]
        tree: tree
      }
    }
  store.updateRef user, {
    ownerName, repoName
    refName: 'branches/master'
    old: commitId
    new: commit
  }


defNogFilesMethod 'addToDatalist', (opts) ->
  check opts,
    src:
      ownerName: matchSimpleName
      repoName: matchSimpleName
      commitId: matchSha1
      entries: [
        type: String
        sha1: matchSha1
      ]
    dst:
      create: Match.Optional Boolean
      ownerName: matchSimpleName
      repoName: matchSimpleName
  unless Meteor.isServer
    return
  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts.dst

  createRepo = ->
    store.createRepo user, {repoFullName: "#{ownerName}/#{repoName}"}
    root = {
      name: "#{repoName} Workspace"
      meta: {workspace: {}}
      entries:[
        {
          name: 'datalist'
          meta: {datalist: {}}
          entries: []
        }
      ]
    }
    tree = store.createTree user, {ownerName, repoName, content: root}
    commit = store.createCommit user, {
        ownerName, repoName, content: {
          subject: "Create datalist workspace"
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

  if opts.dst.create
    createRepo()

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  master = store.getCommit user, {ownerName, repoName, sha1: master}

  # Low-level read ok, since methods above checked read access.
  root = store.trees.findOne master.tree
  strip(root)

  expandDatalist = (root) ->
    for e, idx in root.entries
      if e.type == 'object'
        continue
      else if e.type == 'tree'
        datalist = store.trees.findOne e.sha1
      else
        nogthrow ERR_LOGIC, {reason: 'Unknown entry type.'}
      if _.isObject(datalist.meta.datalist)
        strip(datalist)
        root.entries[idx] = datalist
        return datalist
    nogthrow ERR_UNKNOWN, {reason: 'Failed to find datalist tree.'}

  datalist = expandDatalist root

  children = opts.src.entries
  for e, idx in children
    entry = NogContent.store.copyEntry user, {
        ownerName, repoName, content: {
          copy:
            repoFullName: "#{opts.src.ownerName}/#{opts.src.repoName}"
            sha1: e.sha1
            type: e.type
        }
      }
    datalist.entries.unshift entry

  name = entryContent(entry).name
  commitTree user, {
      ownerName, repoName,
      tree: root,
      subject: "
          Add `#{name}` from repo `#{opts.src.ownerName}/#{opts.src.repoName}`
          to datalist
        "
      parents: [master._id]
    }


defNogFilesMethod 'addProgram', (opts) ->
  check opts,
    src:
      ownerName: matchSimpleName
      repoName: matchSimpleName
      commitId: matchSha1
      entries: [
        sha1: matchSha1
      ]
    dst:
      create: Match.Optional Boolean
      ownerName: matchSimpleName
      repoName: matchSimpleName
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName} = opts.dst

  listOfBasePrograms = []
  # Get latest program version and copy it to dst.  An unchecked
  # `entryContent()` is ok, since access will be checked during `copyEntry()`.
  for e, idx in opts.src.entries
    programPkg = entryContent({type: 'tree', sha1: e.sha1})
    unless (latest = programPkg.entries[0])?
      nogthrow ERR_UNKNOWN, {reason: 'Failed to find latest program version.'}
    baseProgramEntry = store.copyEntry user, {
        ownerName, repoName, content: {
          copy:
            repoFullName: "#{opts.src.ownerName}/#{opts.src.repoName}"
            type: latest.type
            sha1: latest.sha1
        }
      }
    listOfBasePrograms.push entryContent(baseProgramEntry)

  master = store.getRef user, {ownerName, repoName, refName: 'branches/master'}
  master = store.getCommit user, {ownerName, repoName, sha1: master}

  # Low-level read now ok, since methods above checked read access.
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

  for baseProgram in listOfBasePrograms
    baseProgramName = baseProgram.meta.package.name
    description = "Workspace version of #{baseProgramName}"
    datetag = toSimpleISOString moment()
    srcFullName = opts.src.ownerName + '/' + opts.src.repoName
    srcFrozen = _.extend {}, baseProgram.meta.package.version, {
        name: baseProgramName, sha1: baseProgram._id
      }
    runtime = baseProgram.meta.program.runtime ? config.defaultRuntimeSettings
    programs.entries.unshift
      name: baseProgramName
      meta:
        description: description
        package:
          name: "#{ownerName}/#{repoName}/#{baseProgramName}"
          description: description
          authors: [{username: ownerName}]
      entries: [
        {
          name: baseProgramName + '@' + datetag
          meta:
            description: description + " (version #{datetag})"
            package:
              name: "#{ownerName}/#{repoName}/#{baseProgramName}"
              description: description
              authors: [{username: ownerName}]
              dependencies: [
                {name: baseProgramName, repoFullName: srcFullName}
              ]
              frozen: [
                srcFrozen
              ]
              version: {date: datetag}
            program: {}
          entries: [
            baseProgramEntry
            {
              name: 'params'
              blob: NULL_SHA1
              meta:
                description: 'Workspace program parameters'
                program:
                  params: baseProgram.meta.program.params ? {}
            }
            {
              name: 'runtime'
              blob: NULL_SHA1
              meta:
                description: 'Program execution runtime settings'
                program:
                  runtime: runtime
            }
          ]
        }
      ]

  # Commit modified root tree.
  commitTree user, {
      ownerName, repoName,
      tree: root,
      subject: "
          Add workspace version of base program `#{baseProgramName}` from
          repo `#{opts.src.ownerName}/#{opts.src.repoName}`
        "
      parents: [master._id]
    }


defNogFilesMethod 'deleteChildren', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName
    commitId: matchSha1
    numericPath: [Number]
    children: [Number]
  unless Meteor.isServer
    return

  user = Meteor.user()
  store = NogContent.store
  {ownerName, repoName, children} = opts

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
  descending = (a, b) -> b - a
  for i in children.sort(descending)
    leaf.entries.splice(i, 1)

  tree = store.createTree user, {ownerName, repoName, content: root}
  commit = store.createCommit user, {
      ownerName, repoName, content: {
        subject: "Delete children #{children.join(', ')} of #{leaf.name}"
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


share.NogFiles = NogFiles
