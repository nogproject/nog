isTree = (content) -> content.entries?

selectIcon = (ctx) ->
  content = ctx.child.content
  if isTree content
    return null
  if (content.name.match /[.]html$/)?
    'nogReprHtmlIcon'
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
        if (name.match /[.]html$/)?
          'nogReprHtmlFileView'
        else
          null
