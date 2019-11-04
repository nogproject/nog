{ Meteor } = require 'meteor/meteor'
{ _ } = require 'meteor/underscore'
{ Mongo } = require 'meteor/mongo'
{ NogError } = require 'meteor/nog-error'
{
  ERR_CONTENT_CHECKSUM
  nogthrow
} = NogError


# Exported API.
NogContent =
  call: {}
  api: {}
  onerror: NogError.defaultErrorHandler


decodeMeta = (encMeta) ->
  meta = _.omit encMeta, 'more'
  for r in encMeta.more ? []
    meta[r.key] = r.val
  meta

# ISO datetime with UTC `Z`.
rgxISOStringUTC = ///
  ^
  [0-9]{4}-[0-9]{2}-[0-9]{2}
  T
  [0-9]{2}:[0-9]{2}:[0-9]{2}
  Z
  $
  ///

NULL_SHA1 = '0000000000000000000000000000000000000000'

if Meteor.isServer
  crypto = Npm.require('crypto')
  sha1_hex = (d) -> crypto.createHash('sha1').update(d, 'utf8').digest('hex')
else
  { CryptoJS } = require 'meteor/jparker:crypto-core'
  sha1_hex = (d) -> CryptoJS.SHA1(d).toString()

NogContent.contentId = (d) ->
  sha1_hex(EJSON.stringify(d, {canonical: true}))


# XXX: We could expose this through settings to disable checksumming if we
# suspect that it creates performance problems.
optVerifyContentIds = true


verifyId = (d, type) ->
  if d._id != NogContent.contentId(NogContent.stripped(d))
    nogthrow ERR_CONTENT_CHECKSUM, {type, sha1: d._id}


hasAll = (obj, keys) ->
  _.all(keys, (k) -> _.has(obj, k))


verifyCommitId = (d) ->
  keys = ['_id', 'subject', 'message', 'parents', 'tree', 'meta', 'authors',
          'authorDate', 'committer', 'commitDate']
  unless hasAll d, keys
    return
  verifyId d, 'commit'


verifyObjectId = (d) ->
  unless _.has(d, '_idversion')
    return
  switch d._idversion
    when 0
      keys = ['_id', 'name', 'blob', 'meta']
    when 1
      keys = ['_id', 'name', 'blob', 'text', 'meta']
  unless hasAll d, keys
    return
  verifyId d, 'object'


verifyTreeId = (d) ->
  keys = ['_id', 'name', 'entries', 'meta']
  unless hasAll d, keys
    return
  verifyId d, 'tree'


# `createContentCollections({namespace, names})` creates `Mongo.Collections`
# for repos, deletedRepos, commits, trees, and objects; referred to as coll in
# the following.  The full MongoDB collection name can either be specified
# explicitly as `names[coll]`.  If undefined, `#{namespace.coll}.#{coll}` is
# used.  If `namespace.coll` is also undefined, the name defaults to `#{coll}`.

NogContent.createContentCollections = (opts) ->
  opts ?= {}
  {namespace, names} = opts
  names ?= {}
  namespace ?= {}

  makeName = (basename) ->
    if (n = names[basename])?
      return n
    else if (ns = namespace.coll)?
      return "#{ns}.#{basename}"
    else
      return basename

  colls = {}

  colls.repos = new Mongo.Collection makeName('repos'), {
    transform: (d) ->
      if d.owner? and d.name?
        d.fullName = d.owner + '/' + d.name
      d
  }

  colls.commits = new Mongo.Collection makeName('commits'), {
    transform: (d) ->
      # To avoid ambiguous cases, determine _idversion only if both dates are
      # available.
      if (ad = d.authorDate)? and (cd = d.commitDate)?
        if ad.match(rgxISOStringUTC) and cd.match(rgxISOStringUTC)
          d._idversion = 0
        else
          d._idversion = 1
      if d.meta?
        d.meta = decodeMeta d.meta
      verifyCommitId d
      if d.authorDate?
        d.authorDate = moment.parseZone(d.authorDate)
      if d.commitDate?
        d.commitDate = moment.parseZone(d.commitDate)
      d
  }

  colls.trees = new Mongo.Collection makeName('trees'), {
    transform: (d) ->
      d._idversion = 0
      if d.meta?
        d.meta = decodeMeta d.meta
      verifyTreeId d
      d
  }

  # Heuristic to detect idversion:
  #
  #  - Only v1 has text, so it must be v1.
  #  - If the object has a `name`, `blob`, and `meta`, it may still be a partial
  #    fetch, without `text`.  But we assume that it is complete, and, therefore,
  #    assign idv0, and verify the sha1.
  #
  # XXX: For compatibility during a transition period, transform doc to contain
  # both text and meta.content.  When all code paths have been ported to use
  # text, the compatibility transform should be remove.
  #
  colls.objects = new Mongo.Collection makeName('objects'), {
    transform: (d) ->
      if d.meta?
        d.meta = decodeMeta d.meta
      if _.has(d, 'text')
        d._idversion = 1
      else if hasAll(d, ['name', 'blob', 'meta'])
        d._idversion = 0
      verifyObjectId d
      if d.text?
        unless d.meta?.content?
          d.meta ?= {}
          d.meta.content = d.text
      else if d.meta?.content?
        d.text = d.meta.content
      if d.blob == NULL_SHA1
        d.blob = null
      d
  }

  colls.deletedRepos = new Mongo.Collection makeName('trash.repos')

  return colls


colls = NogContent.createContentCollections()

NogContent.repos = colls.repos
NogContent.commits = colls.commits
NogContent.trees = colls.trees
NogContent.objects = colls.objects
NogContent.deletedRepos = colls.deletedRepos


if Meteor.isServer
  # `checkAccess()` throws to deny; otherwise grant.
  # `testAccess()` returns false to deny; true to grant.
  # Use nog-access if available (weak dependency).
  if (p = Package['nog-access'])?
    console.log '[nog-content] using nog-access default policy.'
    NogContent.checkAccess = p.NogAccess.checkAccess
    NogContent.testAccess = p.NogAccess.testAccess
  else
    console.log '
      [nog-content] default access control disabled, since nog-access is not
      available.
    '
    NogContent.checkAccess = ->
    NogContent.testAccess = -> true


# Use nog-blob if available (weak dependency).
if (p = Package['nog-blob'])?
  if Meteor.isServer
    console.log '[nog-content] using nog-blob.'
  NogContent.blobs = p.NogBlob.blobs
else
  if Meteor.isServer
    console.log '[nog-content] no nog-blob.'
  NogContent.blobs = null


# `configure()` reinitializes the internal objects, so that it can be called at
# any time.
NogContent.configure = (cfg) ->
  cfg ?= {}
  old = {}
  for k in ['checkAccess', 'testAccess']
    if cfg[k]?
      old[k] = NogContent[k]
      NogContent[k] = cfg[k]

  for k, v of cfg
    unless old[k]
      console.log "Warning: unused config in NogContent.configure(): #{k} = #{v}."

  NogContent.init_server()

  return old


NogContent.strip = (c) ->
  delete c._id
  delete c._idversion
  delete c.errata


NogContent.stripped = (c) ->
  _.omit(c, '_id', '_idversion', 'errata')


# Exported for testing.
NogContentTest = {}


defMethod = (name, func) ->
  qualname = 'NogContent.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogContent.call[name] = (args...) -> Meteor.call qualname, args...


defMethod 'createRepo', (opts) ->
  check opts,
    repoFullName: String
  unless Meteor.isServer
    return
  NogContent.store.createRepo Meteor.user(), opts


defMethod 'deleteRepo', (opts) ->
  check opts,
    ownerName: String
    repoName: String
  unless Meteor.isServer
    return
  NogContent.store.deleteRepo Meteor.user(), opts


defMethod 'renameRepo', (opts) ->
  check opts,
    old:
      ownerName: String
      repoName: String
    new:
      repoFullName: String
  unless Meteor.isServer
    return
  NogContent.store.renameRepo Meteor.user(), opts


defMethod 'forkRepo', (opts) ->
  check opts,
    old:
      ownerName: String
      repoName: String
    new:
      ownerName: String
  unless Meteor.isServer
    return
  Meteor._sleepForMs 1000
  NogContent.store.forkRepo Meteor.user(), opts


module.exports.NogContent = NogContent
module.exports.NogContentTest = NogContentTest
