{ NogBlob, NogBlobTest } = share

# Access action constants.
AA_UPLOAD = 'nog-blob/upload'
AA_DOWNLOAD = 'nog-blob/download'

{
  ERR_BLOB_ABORT_PENDING
  ERR_BLOB_CONFLICT
  ERR_BLOB_NOT_FOUND
  ERR_BLOB_UPLOAD_EXISTS
  ERR_DB
  ERR_LIMIT_S3_OBJECT_SIZE
  ERR_PARAM_INVALID
  ERR_UPLOADID_UNKNOWN
  ERR_UPLOAD_COMPLETE
  nogthrow
} = NogError

config = NogBlob.config


isSha1 = Match.Where (x) ->
  check x, String
  if not (x.match /^[0-9a-f]{40}$/)?
    throw new Match.Error 'not a sha1'
  true


matchNonNegativeNumber = Match.Where (x) ->
  check x, Number
  unless x >= 0
    throw new Match.Error "Expected non-negative number; got #{x}."
  true


# Compute parts that respect the AWS specs:
# <http://docs.aws.amazon.com/AmazonS3/latest/dev/qfacts.html>
partParamsForSize = (size) ->
  MB = 1024 * 1024
  minPartSize = 5 * MB
  maxNParts = 10000
  usualMaxPartSize = 100 * MB
  usualNParts = 10
  maxSize = maxNParts * 5 * 1000 * MB

  if size > maxSize
    nogthrow ERR_LIMIT_S3_OBJECT_SIZE, {size, maxSize}

  partSize = Math.ceil(size / usualNParts)
  if partSize < minPartSize
    partSize = minPartSize
  else if partSize > usualMaxPartSize
    partSize = usualMaxPartSize

  if partSize > size
    partSize = size

  # S3 requires at least one part, even if it has size 0.
  if partSize > 0
    nParts = Math.ceil(size / partSize)
  else
    nParts = 1

  if nParts > maxNParts
    partSize = Math.ceil(size / maxNParts)
    nParts = Math.ceil(size / partSize)

  return {
    totalSize: size
    nParts: nParts
    partSize: partSize
  }


createParts = (upload, partParams, ids) ->
  upload = _.pick(upload, 'Bucket', 'Key', 'UploadId')
  for i in ids
    if 0 < i <= partParams.nParts
      partNumber: i
      start: (i - 1) * partParams.partSize
      end: Math.min i * partParams.partSize, partParams.totalSize
      url: NogBlob.bucketRouter.getSignedUploadPartUrl(
        _.extend(upload, { PartNumber: i })
      )
    else
      nogthrow ERR_PARAM_INVALID,
        reason: "
            The part number #{i} is out of range [1, #{partParams.nParts}].
          "


collectionContains = (coll, sel) ->
  coll.findOne(sel, {fields: {_id: 1}})?


share.startMultipartUpload_server = (user, opts) ->
  check opts,
    name: String
    size: matchNonNegativeNumber
    sha1: isSha1
    limit: Match.Optional Number
  {sha1, name, size} = opts
  NogBlob.checkAccess user, AA_UPLOAD, {size}

  sel = {_id: sha1, sha1, size, status: 'available'}
  if collectionContains NogBlob.blobs, sel
    return 'known'

  NogBlob.blobs.upsert {
      _id: sha1
    }, {
      $setOnInsert: { sha1, size, status: 'init', mtime: new Date() }
    }
  # If the size does not match, the blob must have been inserted previously
  # with a different size.
  if not collectionContains NogBlob.blobs, {_id: sha1, sha1, size}
    nogthrow ERR_BLOB_CONFLICT, {sha1}

  partParams = partParamsForSize size

  upload = NogBlob.bucketRouter.createMultipartUpload { key: sha1 }

  s3UploadId = upload.UploadId
  bucket = upload.Bucket
  NogBlob.uploads.insert {
      _id: s3UploadId, bucket, partParams, name, heartbeat: new Date(), sha1
    }
  NogBlob.blobs.update {
      _id: sha1, status: 'init'
    }, {
      $set: {status: 'uploading'}
      $currentDate: {mtime: true}
    }

  if opts.limit?
    unless opts.limit > 0
      nogthrow ERR_PARAM_INVALID, {
          reason: "Invalid limit #{opts.limit} (limit must be greater than 0)."
        }
    nStartParts = Math.min opts.limit, partParams.nParts
  else
    nStartParts = Math.min config.maxNStartParts, partParams.nParts
  parts = createParts upload, partParams, [1..nStartParts]

  return {
    s3UploadId: s3UploadId
    nParts: partParams.nParts
    startParts: parts
  }


share.getUploadParts_server = (user, opts) ->
  check opts,
    s3UploadId: String
    partNumbers: Match.Optional [Number]
    offset: Match.Optional Number
    limit: Match.Optional Number
  {s3UploadId} = opts
  NogBlob.checkAccess user, AA_UPLOAD, {s3UploadId}

  if not (upload = NogBlob.uploads.findOne s3UploadId)?
    nogthrow ERR_UPLOADID_UNKNOWN, {s3UploadId}
  s3upload =
    Bucket: upload.bucket,
    Key: upload.sha1
    UploadId: s3UploadId
  {partParams} = upload

  partNumbers = opts.partNumbers
  if not partNumbers?
    offset = opts.offset ? 0
    limit = opts.limit ? config.maxNStartParts
    if not (0 <= offset < partParams.nParts)
      nogthrow ERR_PARAM_INVALID,
        reason: "
            The offset #{offset} is out of range [0, #{partParams.nParts}[.
          "
    partNumbers = [(offset + 1)..Math.min(offset + limit, partParams.nParts)]

  NogBlob.uploads.update s3UploadId, {$set: {heartbeat: new Date()}}

  res =
    s3UploadId: s3UploadId
    parts: createParts s3upload, partParams, partNumbers
    nParts: partParams.nParts
  return res


share.pushUploadedPart_server = (user, opts) ->
  check opts,
    s3UploadId: String
    partNumber: Number
    etag: String
  {s3UploadId} = opts
  NogBlob.checkAccess user, AA_UPLOAD, {s3UploadId}

  nModified = NogBlob.uploads.update {
      _id: s3UploadId
    }, {
      $set: {heartbeat: new Date()}
      $push: {parts: {PartNumber: opts.partNumber, ETag: opts.etag}}
    }
  if nModified is 0
    nogthrow ERR_UPLOADID_UNKNOWN, {s3UploadId}

  return


# `checkPartsComplete()` assumes that `parts` is sorted.
checkPartsComplete = (parts, partParams) ->
  if parts.length isnt partParams.nParts
    nogthrow ERR_PARAM_INVALID,
      reason: "
          Wrong number of parts (expected #{partParams.nParts}, got
          #{parts.length}).
        "
  for i in [0...partParams.nParts]
    pn = parts[i].PartNumber
    if pn isnt (i + 1)
      nogthrow ERR_PARAM_INVALID, {reason: "Unexpected PartNumber #{pn}."}


share.completeMultipartUpload_server = (user, opts) ->
  check opts,
    s3UploadId: String
    s3Parts: Match.Optional [{PartNumber: Number, ETag: String}]
  {s3UploadId} = opts
  NogBlob.checkAccess user, AA_UPLOAD, {s3UploadId}

  if not (upload = NogBlob.uploads.findOne s3UploadId)?
    nogthrow ERR_UPLOADID_UNKNOWN, {s3UploadId}
  {sha1, partParams, bucket} = upload
  parts = opts.s3Parts ? upload.parts
  # S3 requires the parts to be sorted.
  parts = _.sortBy parts, 'PartNumber'
  checkPartsComplete parts, upload.partParams

  # If the status isn't uploading, a concurrent upload must have succeeded; so
  # abort this one.
  if not collectionContains(NogBlob.blobs, {_id: sha1, status: 'uploading'})
    try
      NogBlob.bucketRouter.abortMultipartUpload
        Bucket: bucket
        Key: sha1
        UploadId: s3UploadId
    catch cause
      # It might be a temporary network error.  Keep the upload in the
      # collection, so that a clean up job can abort it later.
      nogthrow ERR_UPLOAD_COMPLETE, {cause}
    NogBlob.uploads.remove {_id: s3UploadId}
    return sha1

  try
    NogBlob.bucketRouter.completeMultipartUpload
      Bucket: bucket
      Key: sha1
      UploadId: s3UploadId
      MultipartUpload: {Parts: parts}
  catch cause
    # This might be a temporary network error.  Keep the upload in the uploads
    # collection, so that a clean up job can cancel it later.
    nogthrow ERR_UPLOAD_COMPLETE, {cause}
  NogBlob.uploads.remove {_id: s3UploadId}

  # Push log separately to add entry even if status != 'uploading'.

  username = user?.username ? 'unknown'
  NogBlob.blobs.update {
      _id: sha1
    }, {
      $push: {
        log: {
          ts: new Date()
          msg: "Uploaded by '#{username}' from local file '#{upload.name}'."
        }
      }
    }

  NogBlob.blobs.update {
      _id: sha1, status: 'uploading'
    }, {
      $set: {
        status: 'available',
        locs: [
          {
            bucket: bucket,
            status: 'online',
            mpp: { n: partParams.nParts, psize: partParams.partSize }
          }
        ]
      }
      $currentDate: {mtime: true}
    }
  return sha1


share.abortMultipartUpload_server = (user, opts) ->
  check opts,
    s3UploadId: String
  {s3UploadId} = opts
  NogBlob.checkAccess user, AA_UPLOAD, opts

  if not (upload = NogBlob.uploads.findOne s3UploadId)?
    nogthrow ERR_UPLOADID_UNKNOWN, {s3UploadId}

  NogBlob.bucketRouter.abortMultipartUpload
    Bucket: upload.bucket
    Key: upload.sha1
    UploadId: s3UploadId

  NogBlob.uploads.remove {_id: s3UploadId}

  return


share.getBlobDownloadURL_server = (user, opts) ->
  check opts,
    sha1: isSha1
    filename: String

  NogBlob.checkAccess user, AA_DOWNLOAD, {}

  if not (blob = NogBlob.blobs.findOne {_id: opts.sha1})?
    nogthrow ERR_BLOB_NOT_FOUND, {blob: opts.sha1}

  return NogBlob.bucketRouter.getDownloadUrl {
    blob,
    filename: opts.filename
  }


Meteor.publish 'nog-blob/blobs', (sha1s) ->
  NogBlob.checkAccess @userId, AA_UPLOAD, {sha1s}
  return NogBlob.blobs.find {_id: {$in: sha1s}},
    fields:
      sha1: true
      size: true
      status: true


class BlobsApi
  constructor: (deps) ->
    {
      @blobs, @checkAccess, @repoSets
    } = deps

  actions: () ->
    [
      { method: 'GET', path: '/:blob', action: @get_blob }
      { method: 'GET', path: '/:blob/content', action: @get_blob_content }
    ]

  # Use `=>` to bind the actions to access this instance's state.
  get_blob: (req) =>
    @checkAccess req.auth?.user, AA_DOWNLOAD, req.params
    {params, baseUrl} = req
    if @repoSets?
      params = _.pick params, 'blob', 'ownerName', 'repoName'
      check params, {blob: isSha1, ownerName: String, repoName: String}
    else
      params = _.pick params, 'blob'
      check params, { blob: isSha1 }
    blob = @blobs.findOne params.blob
    if not blob?
      nogthrow ERR_BLOB_NOT_FOUND, {blob: params.blob}
    if @repoSets?
      @repoSets.checkMembership {
          ownerName: params.ownerName
          repoName: params.repoName
          sha1: params.blob
        }, 'blob'
    res = _.pick blob, 'size', 'status', 'sha1'
    res._id =
      id: blob._id,
      href: Meteor.absoluteUrl(baseUrl[1..] + '/' + blob._id)
    res.content =
      href: NogBlob.bucketRouter.getDownloadUrl {
        blob, filename: "#{blob._id}.dat"
      }
    res

  get_blob_content: (req) =>
    @checkAccess req.auth?.user, AA_DOWNLOAD, req.params
    {params} = req
    if @repoSets?
      params = _.pick params, 'blob', 'ownerName', 'repoName'
      check params, {blob: isSha1, ownerName: String, repoName: String}
    else
      params = _.pick params, 'blob'
      check params, { blob: isSha1 }
    blob = @blobs.findOne params.blob
    if not blob?
      nogthrow ERR_BLOB_NOT_FOUND, {blob: params.blob}
    if @repoSets?
      @repoSets.checkMembership {
          ownerName: params.ownerName
          repoName: params.repoName
          sha1: params.blob
        }, 'blob'
    return {
      statusCode: 307
      location: NogBlob.bucketRouter.getDownloadUrl {
        blob, filename: "#{blob._id}.dat"
      }
    }


{format: urlformat} = Npm.require 'url'


asPartsPage = (opts) ->
  count = opts.nParts
  offset = opts.parts[0].partNumber - 1
  limit = opts.parts.length
  nextOffset = offset + limit
  if nextOffset < count
    next = urlformat
      pathname: [
          opts.baseUrl, opts.sha1, 'uploads', opts.s3UploadId, 'parts'
        ].join('/')
      query:
        offset: nextOffset
        limit: limit
    next = Meteor.absoluteUrl(next[1..])
  else
    next = null
  return {
    count: count
    offset: offset
    limit: limit
    next: next
    items: for p in opts.parts
      p.href = p.url
      _.omit p, 'url'
  }


# `UploadsApi` must be mounted at the same path as `BlobApi` to share the base
# url.  This is similar to the AWS multipart API
# <http://docs.aws.amazon.com/AmazonS3/latest/dev/sdksupportformpu.html>.  A
# notable difference is that `UploadsApi` uses a path suffix `/uploads` instead
# of a query `?upload`.
class UploadsApi
  constructor: (deps) ->
    {@repoSets} = deps

  actions: () ->
    [
      {
        method: 'POST'
        path: '/:sha1/uploads'
        action: @post_start
      }
      {
        method: 'GET'
        path: '/:sha1/uploads/:s3UploadId/parts'
        action: @get_parts
      }
      {
        method: 'POST'
        path: '/:sha1/uploads/:s3UploadId'
        action: @post_complete
      }
    ]

  # Use `=>` to bind the actions to this instance.
  post_start: (req) =>
    opts = _.pick req.params, 'sha1'
    if req.query?.limit?
      opts.limit = Number(req.query.limit)
    _.extend opts, req.body
    res = share.startMultipartUpload_server req.auth?.user, opts

    # Return 409 'conflict' if the blob already exists.  An alternative would
    # be 303 'see other'.  See discussion on SO
    # <http://stackoverflow.com/questions/3825990/http-response-code-for-post-when-resource-already-exists>.
    #
    # Since the upload is not started, the blob will be added to the repo sets
    # now, so that it can be used for an object.  This is not perfect, since it
    # allows a malicious user to guess sha1s and get access to the
    # corresponding content by contructing an object knowing only the sha1 of
    # the blob.  It seems acceptable, since we trust clients to send valid
    # sha1s anyway.  An alternative would be to allow another upload and put it
    # into some kind of quarantine until a server side job has confirmed the
    # sha1 and then delete the upload and use the first upload instead.  The
    # additional upload would be prove that the user actually had access to the
    # data that correspond to the sha1.
    if res is 'known'
      if @repoSets?
        params = _.pick req.params, 'ownerName', 'repoName'
        check params, {ownerName: String, repoName: String}
        @repoSets.updateMembership params, {type: 'blob', sha1: opts.sha1}
      nogthrow ERR_BLOB_UPLOAD_EXISTS, {sha1: opts.sha1}

    # Return 201 'created', since the upload has an id and behaves like a new
    # resource.
    return {
      statusCode: 201
      upload:
        id: res.s3UploadId
        href: do ->
          path = [
              req.baseUrl, opts.sha1, 'uploads', res.s3UploadId
            ].join('/')
          Meteor.absoluteUrl(path[1..])
      parts: asPartsPage
        baseUrl: req.baseUrl
        sha1: opts.sha1
        s3UploadId: res.s3UploadId
        nParts: res.nParts
        parts: res.startParts
    }


  get_parts: (req) =>
    sha1 = req.params.sha1
    check sha1, isSha1
    opts = {
      s3UploadId: req.params.s3UploadId
    }
    if req.query?.offset?
      opts.offset = Number(req.query.offset)
    if req.query?.limit?
      opts.limit = Number(req.query.limit)

    res = share.getUploadParts_server req.auth?.user, opts

    return asPartsPage
        baseUrl: req.baseUrl
        sha1: sha1
        s3UploadId: res.s3UploadId
        nParts: res.nParts
        parts: res.parts

  post_complete: (req) =>
    opts = {
      s3UploadId: req.params.s3UploadId
      s3Parts: req.body?.s3Parts
    }
    blobid = share.completeMultipartUpload_server req.auth?.user, opts

    if @repoSets?
      params = _.pick req.params, 'ownerName', 'repoName'
      check params, {ownerName: String, repoName: String}
      @repoSets.updateMembership params, {type: 'blob', sha1: blobid}

    # Assume that BlobsApi is mounted at the same path as UploadApi.
    baseUrl = req.baseUrl

    # The implementation is similar to the code in BlobsApi.  Consider
    # factoring out common parts.
    #
    # The download URL returned here could be tied to the upload bucket.  But
    # it is not necessary, since the bucket router will automatically select
    # the upload bucket if it is the only one that contains the blob.

    blob = NogBlob.blobs.findOne blobid
    res = _.pick blob, 'size', 'status', 'sha1'
    res._id =
      id: blob._id,
      href: Meteor.absoluteUrl(baseUrl[1..] + '/' + blob._id)
    res.content =
      href: NogBlob.bucketRouter.getDownloadUrl {
        blob, filename: "#{blob._id}.dat"
      }
    res.statusCode = 201
    res


share.init_server = ->
  deps = _.pick NogBlob, 'blobs', 'checkAccess'
  if NogBlob.repoSets
    deps.repoSets = NogBlob.repoSets
  NogBlob.api.blobs = new BlobsApi deps
  NogBlob.api.uploads = new UploadsApi deps

share.init_server()

NogBlobTest.BlobsApi = BlobsApi
NogBlobTest.UploadsApi = UploadsApi
