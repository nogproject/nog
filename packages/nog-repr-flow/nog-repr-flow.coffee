# XXX: refactor iskind helpers to a central place.
iskind = (entry, kind) -> _.isObject(entry.meta[kind])
iskindProgram = (tree) -> iskind tree, 'program'
iskindJob = (tree) -> iskind tree, 'job'
iskindWorkspace = (tree) -> iskind tree, 'workspace'
iskindProgramRegistry = (tree) -> iskind tree, 'programRegistry'


isPackageVersion = (content) -> content.meta.package?.version?

isBundle = (c) ->
  iskindProgram(c) or iskindJob(c) or isPackageVersion(c)


Meteor.startup ->
  if (p = Package['nog-files'])?
    p.NogFiles.registerEntryRepr
      view: (treeCtx) ->
        unless treeCtx.last.type == 'tree'
          return null
        if isBundle(treeCtx.last.content)
          'nogFilesBundleView'
        else
          null

      icon: (entryCtx) ->
        if isBundle(entryCtx.child.content)
          'nogFilesBundleIcon'
        else
          null

      treePermissions: (treeCtx) ->
        if (iskindWorkspace(treeCtx.last.content) or
            iskindProgramRegistry(treeCtx.last.content))
          {write: false}
        else
          null
