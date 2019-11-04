import { Meteor } from 'meteor/meteor'
import { Match } from 'meteor/check'
import { NogAccess } from 'meteor/nog-access'
import { NogContent } from 'meteor/nog-content'


matchSimpleName = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ [a-zA-Z0-9-_]+ $ ///)?
    throw new Match.Error 'Invalid simple name.'
  true


Meteor.publish 'workspaceContent', (opts) ->
  check opts,
    ownerName: matchSimpleName
    repoName: matchSimpleName

  {ownerName, repoName} = opts

  aopts = {ownerName: ownerName, repoName: repoName}
  if not NogAccess.testAccess @userId, 'nog-content/get', aopts
    @ready()
    return null

  repoParams = {owner: ownerName, name: repoName}
  if not (NogContent.repos.findOne(repoParams))?
    @ready()
    return null

  # Publish raw, untransformed docs, since the transform will be applied at
  # the client.
  RAW = {transform: false}

  isPublished =
    trees: {}
    objects: {}

  # Tree traversal is restricted with regular expressions that match the path.
  # Paths are rejected if they match `namespace` but not `select`.  Paths end
  # with slash to indicate a tree and without slash to indicate an object.
  #
  # The primary purpose is to skip deeper levels of result trees so that the
  # publication always scales with the number of sub-results independent of the
  # size of additional details in sub-result sub-trees.  See template
  # `workspaceFlowResult.helpers.resultSet` for the entries that must be sent
  # to the client.
  #
  # In theory, this type of filtering is incompatible with early traversal
  # termination based on `isPublished.trees[id]`.  A tree could have been
  # marked as published with some sub-entries rejected during path filtering.
  # Another paths might lead to acceptance of the sub entries.
  #
  # We assume that such type of conflicting decisions do not happen in practice
  # and use both path-based filtering and early termination together.
  #
  # The next time we touch this, we should consider adding a traversal
  # logic here that is similar to what the client uses.  The publication could
  # parse the kinds and traverse the tree such that it sends exactly what the
  # client actually needs.

  filter = [
    {
      namespace: /// ^/results/ ///
      select: ///
        # Anchored at start in toplevel results ...
        ^/results/
        # a result ...
        ([^/]+/
        # with optional sub-results ...
        ([^/]+/)?
        # with md files, see template `workspaceFlowResult.helpers.resultSet`.
        ([^/]+.md
        # ignoring other files and deeper trees below the sub-results.
        # The match is anchored at the end.
        )?)?$
      ///
    }
  ]
  acceptPath = (p) =>
    for f in filter
      if p.match(f.namespace)
        unless p.match(f.select)
          return false
    return true

  addTree = (id, path) =>
    if isPublished.trees[id]
      return
    if not (tree = NogContent.trees.findOne(id, RAW))?
      return
    # Build the path, using `path=null` to indicate the root.  The root tree
    # name is ignored and slash used instead.
    if path
      path = "#{path}#{tree.name}/"
    else
      path = '/'
    unless acceptPath(path)
      return
    for e in tree.entries
      switch e.type
        when 'object'
          if not isPublished.objects[e.sha1]
            obj = NogContent.objects.findOne(e.sha1, RAW)
            unless acceptPath("#{path}#{obj.name}")
              continue
            @added 'objects', obj._id, obj
            isPublished.objects[obj._id] = true
        when 'tree'
          addTree(e.sha1, path)
        else
          console.log "Unknown type #{e.type}."
    @added 'trees', id, tree
    isPublished.trees[id] = true

  addCommit = (commitId) =>
    # Access check: `getCommit` will throw if access is denied.
    NogContent.store.getCommit @userId, {ownerName, repoName, sha1: commitId}

    unless (commit = NogContent.commits.findOne(commitId, RAW))?
      return
    @added 'commits', commitId, commit

  # The order of adding/changing the items is implemented from bottom to
  # top (objects -> trees -> commits -> repo) to avoid a flickering page
  # content.
  # Syncing changed data triggers a re-rednering on the client for each
  # changed item. On the client, we iterate the data structure from top
  # to bottom, and a new master ref would point to items that not yet
  # exist in case of top-to-bottom order. This would cause undefined
  # items and a flickering page content.

  counterId = @._subscriptionId
  counter = {
      updates: 0
    }

  handle = NogContent.repos.find(repoParams).observeChanges
    added: (id, fields) =>
      repo = NogContent.repos.findOne(repoParams)
      commitId = repo.refs['branches/master']
      commit = NogContent.commits.findOne(commitId)
      addTree(commit.tree, null)
      addCommit(commitId)
      @added 'repos', id, fields
      counter['updates'] = counter['updates'] + 1
      @added 'workspaceContent', counterId, counter
    changed: (id, fields) =>
      repo = NogContent.repos.findOne(repoParams)
      commitId = repo.refs['branches/master']
      commit = NogContent.commits.findOne(commitId)
      addTree(commit.tree, null)
      addCommit(commitId)
      @changed 'repos', id, fields
      counter['updates'] = counter['updates'] + 1
      @changed 'workspaceContent', counterId, counter
    removed: (id) =>
      @removed 'repos', id

  @ready()

  @onStop ->
    handle.stop()
