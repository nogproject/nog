import { Meteor } from 'meteor/meteor'
import { createMongoCache } from './mongo-cache.js'
import './nog-tree-settings.js'


resolveCache = createMongoCache({
  name: 'cache.resolveImgSrc',
  expireAfterSeconds: Meteor.settings.cache.resolveImgSrc.expireAfterSeconds,
})


Meteor.methods
  # `opts` must contain `ownerName` and `repoName` to select the repo.  The
  # image can either be specified as a `refTreePath`; or as `blob` and `name`
  # (only the suffix is used to choose the mime type).
  resolveImgSrc: (opts) ->
    NogAccess.checkAccess Meteor.user(), 'nog-content/get', opts
    this.unblock()

    repo = NogContent.repos.findOne {
        owner: opts.ownerName, name: opts.repoName
      }
    if not repo?
      return null

    if opts.blob?
      {blob, name} = opts
      unless blob? and name?
        return
    else
      unless (parsed = NogContent.parseRefTreePath(repo, opts.refTreePath))?
        return null

      if (c = parsed.commitId)?
        commitId = c
      else if (c = repo.refs[parsed.refFullName])?
        commitId = c
      else
        return null

      cacheKey = "#{commitId}/#{parsed.treePath}"
      if (hit = resolveCache.get(cacheKey))?
        {blob, name} = hit
      else
        res = NogContent.resolveRefTreePath repo, opts.refTreePath
        if not res?
          return null
        object = res.last.content
        {blob, name} = object
        unless blob? and name?
          return
        resolveCache.add(cacheKey, {blob, name})

    if not (blob = NogContent.blobs.findOne(blob))?
      return null

    return NogBlob.bucketRouter.getImgSrc { blob, filename: name }
