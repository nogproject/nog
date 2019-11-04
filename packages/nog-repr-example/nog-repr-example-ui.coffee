# Register selectors assuming weak package dependencies.
selectView = (ctx) ->
  content = ctx.last.content
  if _.isObject(content.meta['exampleRepr'])
    'nogReprExampleView'
  else
    null

selectIcon = (ctx) ->
  content = ctx.child.content
  if _.isObject(content.meta['exampleRepr'])
    'nogReprExampleIcon'
  else
    null

Meteor.startup ->
  if (p = Package['nog-files'])?
    p.NogFiles.registerEntryRepr
      icon: selectIcon
      view: selectView
  if (p = Package['nog-tree'])?
    p.NogTree.registerEntryRepr
      selector: selectView


# View that uses a template-level subscription to load additional data.
#
# XXX `currentData()` contains unnecessary context, which probably causes
# spurious rerendering.  Ideally this would be changed to a the smart / dumb
# component approach described in the Meteor guide
# <http://guide.meteor.com/ui-ux.html#components>.  The smart component
# (`NogFiles` or `NogTree`) would pass a minimal reactive data source to the
# dumb component (the plugin) to minimize rerendering.  Unfortunately, this
# architecture would require a major rewrite of `NogFiles` and `NogTree`, which
# we have postponed.
#
# If you intend to write a new plugin, please consider discussing theses
# questions with a core developer before starting.

Template.nogReprExampleView.onCreated ->
  @autorun =>
    data = Template.currentData()
    if data.last.type == 'tree'
      @subscribe 'exampleReprTree',
        ownerName: data.repo.owner
        repoName: data.repo.name
        sha1: data.last.content._id


Template.nogReprExampleView.helpers
  treeDataContext: ->
    EJSON.stringify @, {indent: true, canonical: true}

  numOfTreeChildren: ->
    num = 0
    countTree = (sha1) ->
      num++
      tree = NogContent.trees.findOne(sha1)
      for e in tree.entries
        if e.type == 'tree'
          countTree e.sha1
    countTree @last.content._id
    return num
