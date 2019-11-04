Meteor.publish 'exampleReprTree', (params) ->
  check params,
    sha1: String
    ownerName: String
    repoName: String

  # Call store.getTree() to check access.  It will throw an exception if the
  # user has no permission to access the tree through the repo.
  store = NogContent.store
  tree = store.getTree @userId, params

  # Publish raw, untransformed docs, since the transform will be applied at
  # the client.
  RAW = {transform: false}

  # Recursively publish the tree and children that are trees.
  isPublished = {}
  addTree = (sha1) =>
    if not (tree = NogContent.trees.findOne(sha1, RAW))?
      return
    if not isPublished[sha1]
      @added 'trees', sha1, tree
      isPublished[sha1] = true
    for e in tree.entries
      if e.type == 'tree'
        addTree e.sha1

  addTree params.sha1

  # Simulate some latency to show loading....
  Meteor._sleepForMs 1000

  @ready()
