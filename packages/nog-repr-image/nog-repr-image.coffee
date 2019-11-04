selectView = (ctx) ->
  limit = Meteor.settings.public.imgPreviewSizeLimit ? 0
  unless ctx.last.type == 'object'
    return null
  name = ctx.last.content.name
  if (name.match /[.](gif|png|jpg|jpeg|tif|tiff)$/i)?
    if (blobId = ctx.last.content.blob)
      if (blob = NogContent.blobs.findOne({_id: blobId}))
        if( blob.size < limit )
          'objectReprImage'
  else
    null


# Register selectors assuming weak package dependencies.
Meteor.startup ->

  if (p = Package['nog-files'])?
    p.NogFiles.registerEntryRepr
      icon: -> null
      view: selectView

  if (p = Package['nog-tree'])?
    p.NogTree.registerEntryRepr
      selector: selectView
