{ Meteor } = require 'meteor/meteor'
{ _ } = require 'meteor/underscore'

{ NogContent } = require './nog-content.coffee'
{ findEntryQualified } = require './nog-content-treepath.coffee'


iskind = (entry, kind) -> _.isObject(entry.meta[kind])

NULL_SHA1 = '0000000000000000000000000000000000000000'


entryContent = (e) ->
  if e.type == 'object'
    NogContent.objects.findOne e.sha1
  else if e.type == 'tree'
    NogContent.trees.findOne e.sha1
  else
    e


# `repoWithRefTreePath` publishes a repo and the commit together with related
# entries for `refTreePath` which may start with a commit id or a named ref
# followed by a numeric or name path.  Examples:
#
#  - `master/0.1.0`
#  - `master/foo/bar/LOG.md`
#  - `1111111111111111111111111111111111111111/0.1.0`
#  - `1111111111111111111111111111111111111111/foo/bar/LOG.md`
#
# The subscription may publish more than necessary: Commits and entries are
# added when the refs change, but they are never removed.  This simplifies
# the implementation and should be sufficiently efficient in practice.
Meteor.publish 'repoWithRefTreePath', (params) ->
  aopts = {ownerName: params.repo.owner, repoName: params.repo.name}
  if not NogContent.testAccess @userId, 'nog-content/get', aopts
    @ready()
    return null

  if not (repo = NogContent.repos.findOne(params.repo))?
    @ready()
    return null

  if not (refTreePath = NogContent.parseRefTreePath repo, params.refTreePath)?
    @ready()
    return null

  # Meteor._sleepForMs 500

  isPublished =
    commits: {}
    trees: {}
    objects: {}
    blobs: {}

  # Publish raw, untransformed docs, since the transform will be applied at
  # the client.
  RAW = {transform: false}

  # `addCommits` publishes commits and the related entries along the treepath.
  #
  # Walk the path even if a commit or tree has already been published, since
  # they may have been published for a different path.
  addCommits = (ids) =>
    # `other` is used to collect ids of related entries during the path walk
    # in `addTree`, so that a batch fetch can be used to get them from the
    # db.
    other =
      objects: []
      trees: []
      blobs: []

    for id in ids
      if (c = NogContent.commits.findOne(id, RAW))?
        if not isPublished.commits[id]
          @added 'commits', id, c
          isPublished.commits[id] = true
        addTree c.tree, refTreePath.namePath, findEntryQualified, other

    osel = {_id: {$in: other.objects}}
    for o in NogContent.objects.find(osel, RAW).fetch()
      @added 'objects', o._id, o
      isPublished.objects[o._id] = true
      # `NULL_SHA1` must be skipped to support object idversion 0.  `o.blob`
      # may equal `NULL_SHA1`, because the `find()` above fetches
      # untransformed, raw docs.
      if o.blob? and o.blob != NULL_SHA1
        other.blobs.push o.blob unless isPublished.blobs[o.blobs]

    tsel = {_id: {$in: other.trees}}
    for t in NogContent.trees.find(tsel, RAW).fetch()
      @added 'trees', t._id, t
      isPublished.trees[t._id] = true

    bsel = {_id: {$in: other.blobs}}
    for b in NogContent.blobs.find(bsel, RAW).fetch()
      @added 'blobs', b._id, b
      isPublished.blobs[b._id] = true

  # Walk along the first children as deeply as possible and add to `other`.
  # The current version and the upstream dependency are assumed to be found
  # along the first children path.
  addOtherPackage = (content, other) ->
    loop
      content = entryContent(content.entries[0])
      unless content.entries?
        return
      for e in content.entries ? []
        switch e.type
          when 'object'
            other.objects.push e.sha1 unless isPublished.objects[e.sha1]
          when 'tree'
            other.trees.push e.sha1 unless isPublished.trees[e.sha1]

  addTree = (id, path, findEntry, other) =>
    if not (tree = NogContent.trees.findOne(id, RAW))?
      return
    if not isPublished.trees[id]
      @added 'trees', id, tree
      isPublished.trees[id] = true
    for e in tree.entries
      switch e.type
        when 'object'
          other.objects.push e.sha1 unless isPublished.objects[e.sha1]
        when 'tree'
          other.trees.push e.sha1 unless isPublished.trees[e.sha1]
        else
          console.log "Unknown type #{e.type}."
    # Publish additional entries for package (to display README).
    c = NogContent.trees.findOne(id)  # without RAW to get transformed meta.
    if iskind(c, 'package')
      addOtherPackage(c, other)
    if path.length
      [head, tail...] = path
      if (e = findEntry(tree.entries, head))?
        if e.type is 'tree'
          addTree e.sha1, tail, findEntry, other

  requiredCommits = (repo) ->
    ids = []
    if (c = refTreePath.commitId)?
      ids.push c
    if (r = refTreePath.refFullName)?
      if (id = repo.refs?[r])?
        ids.push id
    ids

  handle = NogContent.repos.find(params.repo).observeChanges
    added: (id, fields) =>
      @added 'repos', id, fields
      addCommits requiredCommits fields
    changed: (id, fields) =>
      @changed 'repos', id, fields
      addCommits requiredCommits fields
    removed: (id) =>
      @removed 'repos', id

  @ready()

  @onStop ->
    handle.stop()

  return
