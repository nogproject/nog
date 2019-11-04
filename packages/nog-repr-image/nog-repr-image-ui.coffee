Template.objectReprImage.helpers
  imgSrc: ->
    blobPath =
      ownerName: @repo.owner
      repoName: @repo.name
      name: @last.content.name
      blob: @last.content.blob
    NogContent.resolveImgSrc(blobPath)
