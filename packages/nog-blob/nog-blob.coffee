# `NogBlob` is exported.
NogBlob =
  call: {}
  api: {}
  onerror: NogError.defaultErrorHandler

  # If `checkAccess()` does not throw, access is granted.
  checkAccess: ->

  # `blobs` contains the uploaded blobs.
  blobs: new Mongo.Collection 'blobs'


# Blob indexes:
#
#  - `mtime` to efficiently track updates.
#  - `locks.ts` to efficiently expire locks.  The index is sparse, assuming
#    that there are only a few active locks.

if Meteor.isServer
  NogBlob.blobs._ensureIndex { 'mtime': 1 }
  NogBlob.blobs._ensureIndex { 'locks.ts': 1 }, { sparse: true }


if Meteor.isServer then _.extend NogBlob,
  # `uploads` is used internally to track active uploads.
  uploads: new Mongo.Collection 'uploads'

  # `repoSets` can be set to an instance of `RepoSets` (see package
  # `nog-content`) to check whether a blob can be reached from a repo.
  repoSets: false


# Use nog-access if available (weak dependency).
if Meteor.isServer
  if (p = Package['nog-access'])?
    console.log '[nog-blob] using nog-access default policy.'
    NogBlob.checkAccess = p.NogAccess.checkAccess
  else
    console.log '
        [nog-blob] default access control disabled, since nog-access is not
        available.
      '

if Meteor.isClient then _.extend NogBlob,
  # `files` is a client-only collection that provides reactivity for keeping
  # track of the progress during file upload.
  files:  new Mongo.Collection null


# Exported for testing.
NogBlobTest = {}


NogBlob.config = config =

  # `s3UploadTimeout_ms` controls when a previous upload is canceled in
  # `createMultipartUpload`.
  s3UploadTimeout_ms: 30 * 60 * 1000

  # `maxNStartParts` is the maximum number of concurrent uploads that are
  # initiated when creating a multipart upload.
  maxNStartParts: 5


# `configure()` can be called at any time to change the active config.
NogBlob.configure = (cfg) ->
  cfg ?= {}
  old = {}
  for k in ['onerror', 'checkAccess', 'repoSets']
    if cfg[k]?
      old[k] = NogBlob[k]
      NogBlob[k] = cfg[k]
  for k of config
    if cfg[k]?
      old[k] = config[k]
      config[k] = cfg[k]

  for k, v of cfg
    unless old[k]?
      console.log "Warning: unused config in NogBlob.configure(): #{k} = #{v}."

  if Meteor.isServer
    share.init_server()
  return old


# Call `configure()` during startup to check the initial config and potentially
# initialize missing pieces.
Meteor.startup -> NogBlob.configure()


defMethod = (name, func) ->
  qualname = 'NogBlob.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogBlob.call[name] = (args...) -> Meteor.call qualname, args...


isSha1 = Match.Where (x) ->
  check x, String
  x.match /^[0-9a-f]{40}$/


# When there is a good opportunity, the methods should probably be modified to
# include the repo name in order to support repo sets.

defMethod 'startMultipartUpload', (opts) ->
  check opts,
    name: String
    size: Number
    sha1: isSha1
  if not Meteor.isServer
    return
  share.startMultipartUpload_server Meteor.user(), opts


defMethod 'getUploadParts', (opts) ->
  check opts,
    s3UploadId: String
    partNumbers: Match.Optional [Number]
  if not Meteor.isServer
    return {}
  share.getUploadParts_server Meteor.user(), opts


defMethod 'pushUploadedPart', (opts) ->
  check opts,
    s3UploadId: String
    partNumber: Number
    etag: String
  if not Meteor.isServer
    return {}
  share.pushUploadedPart_server Meteor.user(), opts


defMethod 'completeMultipartUpload', (opts) ->
  check opts,
    s3UploadId: String
  if not Meteor.isServer
    return
  share.completeMultipartUpload_server Meteor.user(), opts


defMethod 'abortMultipartUpload', (opts) ->
  check opts,
    s3UploadId: String
  if not Meteor.isServer
    return
  share.abortMultipartUpload_server Meteor.user(), opts


defMethod 'getBlobDownloadURL', (opts) ->
  check opts,
    sha1: isSha1
    filename: String
  if not Meteor.isServer
    return
  share.getBlobDownloadURL_server Meteor.user(), opts


share.NogBlob = NogBlob
share.NogBlobTest = NogBlobTest
