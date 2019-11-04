isTree = (content) -> content.entries?

selectIcon = (ctx) ->
  content = ctx.child.content
  if isTree content
    return null
  if (content.name.match /[.]md$/)?
    'nogReprMarkdownIcon'
  else
    null


# Register selectors assuming weak package dependencies.
Meteor.startup ->
  if (p = Package['nog-files'])?
    p.NogFiles.registerEntryRepr
      icon: selectIcon
      view: (ctx) ->
        unless ctx.last.type == 'object'
          return null
        name = ctx.last.content.name
        if (name.match /[.]md$/)?
          'nogReprMarkdownFileView'
        else
          null



  if (p = Package['nog-tree'])?
    p.NogTree.registerEntryRepr
      selector: (ctx) ->
        unless ctx.last.type == 'object'
          return null
        name = ctx.last.content.name
        if (name.match /[.]md$/)?
          'objectReprMarkdown'
        else
          null
