{ _ } = require 'meteor/underscore'


# `TransientSet` maintains a set that is automatically cleared when it exceeds
# a size or age threshold.  It can be useful for implementing a cache of
# limited size and with eventual consistency.  Specifically, it is used for
# caching content collection membership information and repo sets (see
# `CachedRepoSets`).

class TransientSet
  constructor: (opts) ->
    opts ?= {}
    @maxSize = opts.maxSize ? (16 * 1024)
    @maxAge_s = opts.maxAge_s ? (5 * 60)
    @name = opts.name ? 'unknown'
    @clear()

  clear: ->
    @elements = {}
    @nElements = 0
    @nHits = 0
    @nMisses = 0
    @created = new Date()

  clearMaybe: ->
    if @age_s() > @maxAge_s or @size() > @maxSize
      console.log(
        "[nog-content] #{@name} hit ratio before clear:",
        @hitRatio().toFixed(3)
      )
      @clear()

  size: -> @nElements

  age_s: ->
    now = new Date()
    return (now - @created) / 1000

  hitRatio: ->
    if @nHits
      @nHits / (@nHits + @nMisses)
    else
      0

  contains: (key) ->
    @clearMaybe()
    if @elements[key]?
      @nHits++
      return true
    else
      @nMisses++
      return false

  insert: (key) ->
    @clearMaybe()
    if @elements[key]?
      return
    @nElements++
    @elements[key] = true


# `CachedRepoSets` can be used as a caching drop-in wrapper for `RepoSets`.  It
# asks the underlying `RepoSets` when necessary.

class CachedRepoSets
  constructor: (repoSets, opts) ->
    opts ?= {}
    @repoSets = repoSets
    @cache = new TransientSet {
      name: 'repo sets cache'
      maxSize: opts.maxCacheSize ? (512 * 1024)
      maxAge_s: opts.maxCacheAge_s ? (60 * 60)
    }

  isMember: (opts) ->
    cacheKey = NogContent.contentId(
      _.pick(opts, 'ownerName', 'repoName', 'sha1')
    )
    if @cache.contains cacheKey
      return true
    if @repoSets.isMember opts
      @cache.insert cacheKey
      return true
    return false

  checkMembership: (opts, ty) ->
    cacheKey = NogContent.contentId(
      _.pick(opts, 'ownerName', 'repoName', 'sha1')
    )
    if @cache.contains cacheKey
      return
    @repoSets.checkMembership opts, ty  # Throws if not member.
    @cache.insert cacheKey

  updateMembership: (opts, entry) ->
    cacheKey = NogContent.contentId(
      _.extend({sha1: entry.sha1}, _.pick(opts, 'ownerName', 'repoName'))
    )
    @cache.insert cacheKey
    @repoSets.updateMembership opts, entry


module.exports.TransientSet = TransientSet
module.exports.CachedRepoSets = CachedRepoSets
