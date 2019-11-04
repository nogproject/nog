{ check } = require('meteor/check')
{ createRateLimiter } = require('./rate-limit.js')
{ NogContent } = require('meteor/nog-content')
{ contentIndex, searchContent } = require('./nog-search.coffee')
require('./nog-search-settings.js')

optGlobalReadOnly = Meteor.settings.optGlobalReadOnly
optEnsureIndexes = true

config =
  searchIndexMaxIdPartitions: (
    Meteor.settings.cluster?.maxIdPartitions?.searchIndex ?
    Meteor.settings.cluster?.maxIdPartitions?.default ?
    1
  )

# `searchIndexMaxStringLength` restricts the length of meta values.  Larger
# values will be truncated to avoid MongoDB index errors `17280 Btree::insert:
# key too large to index`.
#
# The MongoDB limit for index keys is 1024, see
# <https://docs.mongodb.com/manual/reference/limits/#indexes>.  A hashed index
# does not have a size limit <http://stackoverflow.com/a/27793044>.  But hashed
# indexes have major limitations and, therefore, are not an alternative: Hashed
# indexes only support equality queries; they cannot be used in compound
# indexes; they do not work with array values
# <https://docs.mongodb.com/manual/core/index-hashed/#considerations>.

# `searchIndexUpdateReadRateLimit` and `searchIndexUpdateWriteRateLimit`
# restrict the rate of operations for background search index background
# updates.  The rate is specified for higher-level application operations,
# which seems simpler than trying to exactly track every individual MongoDB
# call.  If the op budget for a time slice is consumed, the thread pauses until
# the start of the next time slice plus jitter.

{
  searchIndexMaxStringLength
  searchIndexUpdateReadRateLimit
  searchIndexUpdateWriteRateLimit
} = Meteor.settings

check(searchIndexMaxStringLength, Number)
check(searchIndexUpdateReadRateLimit, Number)
check(searchIndexUpdateWriteRateLimit, Number)


readLimiter = createRateLimiter({
  name: 'searchIndexUpdateReads',
  maxOps: searchIndexUpdateReadRateLimit,
  intervalMs: 1000,
})
writeLimiter = createRateLimiter({
  name: 'searchIndexUpdateWrites',
  maxOps: searchIndexUpdateWriteRateLimit,
  intervalMs: 1000,
})


truncatedMetaVal = (val) ->
  isTruncated = false

  if _.isNumber(val) or Match.test(val, [Number])
    return { val, isTruncated }

  truncated = (s) ->
    if s.length <= searchIndexMaxStringLength
      return s
    isTruncated = true
    indicator = '[TRUNCATED]'
    return s[0...(searchIndexMaxStringLength - indicator.length)] + indicator

  if _.isString(val)
    val = truncated(val)
  else
    val = val.map(truncated)

  return { val, isTruncated }


# Selected fields are stored as toplevel fields.  The rest in a `{key, val}`
# array, but only if val is reasonable to index and search (objects, for
# example are not).
encodeContent = (content) ->
  skip = ['content']
  selected = ['description']
  enc = {more: []}
  if content.text?
    enc.text = content.text
  for k, v of content.meta
    if Match.test v, Match.OneOf(String, Number, [String], [Number])
      if k in skip
        continue
      if k in selected
        enc[k] = v
        continue
      { val, isTruncated } = truncatedMetaVal(v)
      if isTruncated
        console.log(
          "[nog-search] Truncated meta value in content index " +
          "to satisfy max string length #{searchIndexMaxStringLength}: " +
          "#{content._id}.meta.#{k}"
        )
      enc.more.push { key: k, val }
  enc


# When running outside of `startup()`, the search did not work for unknown
# reasons.
if Meteor.isServer then Meteor.startup ->
  if optEnsureIndexes
    console.log '[nog-search] Ensuring indexes.'
    contentIndex._ensureIndex {'refs.ownerName': 1}
    contentIndex._ensureIndex {'refs.ownerId': 1}
    contentIndex._ensureIndex {'refs.repoName': 1}
    contentIndex._ensureIndex {'refs.repoId': 1}
    contentIndex._ensureIndex {'refs.sharing.public': 1}
    contentIndex._ensureIndex {'refs.sharing.allCircles': 1}
    contentIndex._ensureIndex {'refs.sharing.circles': 1}
    contentIndex._ensureIndex {'more.key': 1}
    contentIndex._ensureIndex {'more.val': 1}
    contentIndex._ensureIndex {'path': 1}


if Meteor.isServer
  contentIndexState = new Mongo.Collection 'content_index_state'

  removeEmptyRefs = ->
    contentIndex.remove {refs: {$size: 0}}

  # Remove index entries for repos that no longer exist.
  cleanupStaleIndexEntries = ()->
    coll = contentIndex.rawCollection()
    distinctSync = Meteor.wrapAsync coll.distinct, coll
    for repoId in distinctSync('refs.repoId')
      if not (NogContent.repos.findOne(repoId))?
        writeLimiter.op(10)
        contentIndex.update {
            refs: {$elemMatch: {repoId}}
          }, {
            $pull: {refs: {repoId}}
          }, {
            multi: true
          }
    removeEmptyRefs()

    coll = contentIndexState.rawCollection()
    distinctSync = Meteor.wrapAsync coll.distinct, coll
    for repoId in distinctSync('repoId')
      if not (NogContent.repos.findOne(repoId))?
        contentIndexState.remove {repoId}

  # Cleanup once at startup and then every hour.
  #
  # XXX: This should probably be improved to avoid unexpected load on Mongo.
  # Ideas:
  #
  # - Use a scheme that avoids running the cleanup from multiple containers.
  # - Add some jitter.
  # - Schedule based on calendar time.

  if optGlobalReadOnly
    console.log(
      '[search] [GRO] Stale index cleanup disabled in read-only mode.'
    )
  else
    Meteor.startup cleanupStaleIndexEntries
    Meteor.setInterval cleanupStaleIndexEntries, 60 * 60 * 1000


  updateContentIndex = (opts) ->
    check opts,
      repoId: String
      refName: String
    {repoId, refName} = opts

    version = 21

    # Loop until the state update succeeds.
    loop
      writeLimiter.op()

      if not (repo = NogContent.repos.findOne(repoId))?
        # If repo does not exist, remove its index entries.
        contentIndex.update {
            refs: {$elemMatch: {repoId}}
          }, {
            $pull: {refs: {repoId}}
          }, {
            multi: true
          }
        # No `removeEmptyRefs()` here, since it requires a collection scan.
        # Rely on `cleanupStaleIndexEntries` to garbage collect.
        contentIndexState.remove {repoId}
        return null

      if not (commitId = repo.refs[refName])?
        # If ref does not exist, remove its index entries.  This ignores that
        # there might be a race condition when the ref is deleted and
        # re-created right away.
        contentIndex.update {
            refs: {$elemMatch: {repoId, refName}}
          }, {
            $pull: {refs: {repoId, refName}}
          }, {
            multi: true
          }
        # No `removeEmptyRefs()` here, since it requires a collection scan.
        # Rely on `cleanupStaleIndexEntries` to garbage collect.
        contentIndexState.remove {repoId, refName}
        return null

      if not (commit = NogContent.commits.findOne(commitId))?
        # Ignore inconsistency in the content store.
        return null

      ownerName = repo.owner
      ownerId = repo.ownerId
      repoName = repo.name
      sharing = repo.sharing
      sharingKey = EJSON.stringify sharing, {canonical: true}

      # If the state matches, take the stored date and propagate it.
      sel = {
          repoId, refName, ownerName, ownerId, repoName,
          commitId, sharingKey, version
        }
      if (state = contentIndexState.findOne(sel))?
        date = state.date
        break

      # Otherwise, try to update the state to a new date.
      date = new Date()
      sel = {repoId, refName}
      contentIndexState.upsert sel, {
          $set: sel
          $setOnInsert: {
            ownerName, ownerId, repoName, commitId, sharingKey, date, version
          }
        }
      contentIndexState.update {
          repoId, refName, date: {$lt: date}
        }, {
          $set: {
            ownerName, ownerId, repoName, commitId, sharingKey, date, version
          }
        }

    refState = {
      repoId, refName, ownerName, ownerId, repoName, sharing, commitId, date
    }

    willRecheck = false

    walk = (entry, path) ->
      readLimiter.op()

      switch entry.type
        when 'object'
          content = NogContent.objects.findOne(entry.sha1)
        when 'tree'
          content = NogContent.trees.findOne(entry.sha1)
      if not content?
        return null

      if not path?
        path = ''
      else if path is ''
        path = content.name
      else
        path = path + '/' + content.name

      # Skip if there is a current or newer entry in refs.
      sel =
        sha1: entry.sha1
        path: path
        type: entry.type
        v: version
        refs:
          $elemMatch: {
            repoId, refName, date: {$gte: date}
          }
      if contentIndex.findOne(sel)?
        return

      willRecheck = true

      if content.entries?
        for e in content.entries
          walk e, path

      # Loop until the expected state has been reached.
      until contentIndex.findOne {
        sha1: entry.sha1
        path: path
        type: entry.type
        v: version
        refs:
          $elemMatch: {
            repoId, refName, date: {$gte: date}
          }
      }
        writeLimiter.op(3)
        # Ensure index doc is present.
        sel =
          sha1: entry.sha1
          path: path
          type: entry.type
          v: version
        contentIndex.upsert sel, {
            $set: sel
            $setOnInsert: _.extend({refs: [refState]}, encodeContent(content))
          }
        # Ensure that refs entry is present.
        contentIndex.update {
            sha1: entry.sha1
            path: path
            type: entry.type
            v: version
            refs: {
              $not: {$elemMatch: {repoId, refName}}
            }
          }, {
            $push: {refs: refState}
          }
        # Update outdated refs entry.
        contentIndex.update {
            sha1: entry.sha1
            path: path
            type: entry.type
            v: version
            refs: {
              $elemMatch: {repoId, refName, date: {$lt: date}}
            }
          }, {
            $set: { 'refs.$': refState }
          }

    walk {type: 'tree', sha1: commit.tree}, null

    writeLimiter.op()
    selRef = {repoId, refName, date: {$lt: date}}
    contentIndex.update {
        refs: {$elemMatch: selRef}
      }, {
        $pull: {refs: selRef}
      }, {
        multi: true
      }

    # No `removeEmptyRefs()` here, since it requires a collection scan.  Rely
    # on `cleanupStaleIndexEntries` to garbage collect.

    # Check again if changes were made.
    if willRecheck
      updateContentIndex(opts)


  # The selector for `updateContentIndex` is probably too naive, since it only
  # updates `master`.
  #
  # The update is deferred for each repo (see underscore doc for `debounce`) to
  # efficiently support a sequence of commits.
  Meteor.startup ->
    nActive = 0
    wait_ms = 15 * 1000
    update = Meteor.bindEnvironment (repoId) ->
      console.log '[nog-search] Begin search index update, repoId:', repoId
      nActive++
      updateContentIndex {
          repoId
          refName: 'branches/master'
        }
      nActive--
      console.log '[nog-search] End search index update, repoId:', repoId,
        "(#{nActive} more index updates)"
    updateRepo = {}
    updateDeferred = (id) ->
      updateRepo[id] ?= _.debounce(update, wait_ms)
      updateRepo[id](id)

    observers = {}

    partition = new NogCluster.IdPartition {
      name: 'searchIndex'
      max: config.searchIndexMaxIdPartitions
    }
    partition.onacquire = (part) ->
      console.log "
        [nog-search] Start updating search index for repos #{part.selHuman}.
      "
      sel = {_id: part.sel}
      observers[part.begin] = NogContent.repos.find(sel).observeChanges {
        added: updateDeferred
        changed: updateDeferred
        removed: updateDeferred
      }
    partition.onrelease = (part) ->
      console.log "
        [nog-search] Stop updating search index for repos #{part.selHuman}.
      "
      observers[part.begin].stop()
      delete observers[part.begin]
    NogCluster.registerHeartbeat(partition)



# MongoDB text search is not available at meteor.com.  We use regex search on
# the same fields as a fallback.

optTextSearch = Meteor.settings.public?.optTextSearch ? true

if optTextSearch
  console.log '[app] Using text search.'
else
  console.log '[app] Text search disabled by optTextSearch.'


if Meteor.isServer and optTextSearch
  # Create fulltext index.  Give it a name to be able to refer to it, for
  # example, in `dropIndex()`.
  spec =
    'text': 'text'
    'description': 'text'
    'path': 'text'
  opts =
    name: 'fulltext'
    weights:
      'text': 10
      'description': 5
      'path': 1

  if optGlobalReadOnly
    console.log "
      [app] [GRO] Skipped creating MongoDB fulltext index in read-only mode.
    "
  else
    # XXX: There can be only one fulltext index: try to drop old index names
    # before creating the new one.  This can be removed after the new name has
    # been deployed at least once to everywhere.
    try
      NogContent.contentIndex._dropIndex 'content'
    catch err
      true

    # NogContent.contentIndex._dropIndex opts.name
    NogContent.contentIndex._ensureIndex spec, opts

  # # Fulltext search.
  # console.log 'xxx test search', NogContent.contentIndex.find(
  #     {$text: {$search: 'preview'}},
  #     {fields: {path: 1, 'description': 1}}
  #   ).fetch()

  # # Get index info, which might be useful to decide whether to drop indices.
  # # Meteor does not provide a wrapped function, so reach through to the
  # # low-level MongoDB lib.  See
  # # <http://mongodb.github.io/node-mongodb-native/2.0/api/Collection.html#indexes>.
  # NogContent.objects.rawCollection().indexes (err, res) ->
  #   for r in res
  #     console.log 'xxx index', r
