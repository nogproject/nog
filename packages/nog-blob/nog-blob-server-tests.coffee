import chai from 'chai'
import sinon from 'sinon'
import sinonChai from 'sinon-chai'
expect = chai.expect
chai.use(sinonChai)

{parse: urlparse} = Npm.require 'url'

fakeFileSha = '3f786850e387550fdab836ed7e6dc881de23001b'
emptyFileSha1 = 'da39a3ee5e6b4b0d3255bfef95601890afd80709'

describe 'nog-blob', -> describe 'BlobsApi', ->
  fakeSha1 = 'd6a3b8479e32f863eeec4ea48afa275939d82736'
  fakeBlobs = new Mongo.Collection null
  fakeBlobs.insert
    _id : fakeSha1
    sha1 : fakeSha1
    size : 9437184
    status : "available"
  checkAccessArgs = null
  blobsApi = new NogBlobTest.BlobsApi
    blobs: fakeBlobs
    checkAccess: (user, action, opts) ->
      checkAccessArgs = {user, action, opts}
  actions = {}
  for a in blobsApi.actions()
    actions[a.method + ' ' + a.path] = a.action

  it "GET /:blob throws 404 if blob is not found.", ->
    fn = -> actions['GET /:blob'] {
      params: {blob: '0000000000000000000000000000000000000000'}
      baseUrl: ''
    }
    expect(fn).to.throw /BLOB_NOT_FOUND/
    try
      fn()
    catch err
      expect(err.statusCode).to.equal 404

  it "GET /:blob returns the expected blob representation.", ->
    blob = actions['GET /:blob'] {
      params: {blob: fakeSha1}
      baseUrl: '/baseurl'
    }
    check blob, Match.ObjectIncluding
      _id:
        href: Match.Where (x) ->
          check x, String
          (x.match /// baseurl ///)? and (x.match new RegExp fakeSha1)
        id: String
      sha1: String
      size: Number
      status: String
      content:
        href: String

  it "GET /:blob/content returns a redirect to the content.", ->
    result = actions['GET /:blob/content'] {
      params: {blob: fakeSha1}
    }
    expect(result.statusCode).to.equal 307
    expect(result.location).to.match /^http/


  for route in ['GET /:blob', 'GET /:blob/content']
    do (route) ->
      it "#{route} calls access check.", ->
        checkAccessArgs = null
        req =
          params: {blob: fakeSha1, owner: 'fakeOwner'}
          baseUrl: '/baseurl'
          auth: {user: 'fakeUser'}
        blob = actions[route](req)
        expect(checkAccessArgs).to.exist
        expect(checkAccessArgs.user).to.equal req.auth.user
        expect(checkAccessArgs.action).to.equal 'nog-blob/download'
        expect(checkAccessArgs.opts).to.equal req.params


describe 'nog-blob', -> describe 'BlobsApi (with repoSets)', ->
  fakeSha1 = 'd6a3b8479e32f863eeec4ea48afa275939d82736'
  fakeBlobs = new Mongo.Collection null
  fakeBlobs.insert
    _id : fakeSha1
    sha1 : fakeSha1
    size : 9437184
    status : "available"
  repoSets = {checkMembership: sinon.spy()}
  blobsApi = new NogBlobTest.BlobsApi
    blobs: fakeBlobs
    checkAccess: ->
    repoSets: repoSets
  actions = {}
  for a in blobsApi.actions()
    actions[a.method + ' ' + a.path] = a.action

  for route in ['GET /:blob', 'GET /:blob/content']
    do (route) ->
      it "#{route} requires ownerName and repoName.", ->
        ownerName = 'fakeOwner'
        repoName = Random.id()
        req =
          baseUrl: '/baseurl'
          auth: {user: 'fakeUser'}
        req.params = {blob: fakeSha1, owner: 'fakeOwner', ownerName}
        fn = -> actions[route](req)
        expect(fn).to.throw /// match.*repoName ///i
        req.params = {blob: fakeSha1, owner: 'fakeOwner', repoName}
        fn = -> actions[route](req)
        expect(fn).to.throw /// match.*ownerName ///i

  for route in ['GET /:blob', 'GET /:blob/content']
    do (route) ->
      it "#{route} calls checkMembership.", ->
        ownerName = 'fakeOwner'
        repoName = Random.id()
        req =
          params: {
            blob: fakeSha1, owner: 'fakeOwner', ownerName, repoName
          }
          baseUrl: '/baseurl'
          auth: {user: 'fakeUser'}
        repoSets.checkMembership.reset()
        blob = actions[route](req)
        expect(repoSets.checkMembership).to.have.been.calledWith(
            sinon.match({ownerName, repoName, sha1: fakeSha1}), 'blob'
          )

  it 'repoSets can be injected by calling configure()', ->
    oldCfg = NogBlob.configure {repoSets: false}
    expect(NogBlob.api.blobs.repoSets).to.not.exist
    NogBlob.configure {repoSets}
    expect(NogBlob.api.blobs.repoSets).to.exist
    NogBlob.configure oldCfg


describe 'nog-blob', -> describe 'UploadsApi', ->
  api = new NogBlobTest.UploadsApi {}
  actions = {}
  for a in api.actions()
    actions[a.method + ' ' + a.path] = a.action

  # Reset blobs collection to avoid that the blob is already known.
  before -> NogBlob.blobs.remove {}

  stubCheckAccess = null
  beforeEach -> stubCheckAccess = sinon.stub NogBlob, 'checkAccess'
  afterEach -> stubCheckAccess.restore()

  baseUrl = '/baseurl'
  basereq =
    baseUrl: baseUrl
    auth: {user: 'fakeUser'}
  expectCheckAccess = ->
    expect(stubCheckAccess).to.have.been.calledWith(
        'fakeUser', 'nog-blob/upload'
      )

  fakeUploadId = null

  it "
    'POST /:sha1/uploads' checks access and calls S3 to start a multipart
    upload.
  ", ->
    br = NogBlob.bucketRouter
    stubS3Create = sinon.stub(
      br, 'createMultipartUpload',
    ).callsFake (opts) ->
      fakeUploadId = Random.id()
      bucket = Random.id()
      _.extend opts, {Bucket: bucket, Key: opts.key, UploadId: fakeUploadId}
    stubS3SignUrl = sinon.stub(
      br, 'getSignedUploadPartUrl',
    ).callsFake (opts) ->
      return "https://#{opts.Bucket}/#{opts.Key}/#{opts.UploadId}?fakesig"

    req =
      params: {sha1: fakeFileSha}
      body: {name: 'a.txt', size: 2}
    res = actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(res.statusCode).to.equal 201
    expect(res.upload.id).to.equal fakeUploadId
    expect(res.upload.href).to.contain baseUrl
    expect(res.upload.href).to.contain fakeFileSha
    expect(res.upload.href).to.contain fakeUploadId
    expect(res.parts.count).to.equal 1
    expect(res.parts.items).to.have.length 1
    expect(res.parts.items[0].href).to.exist
    expect(res.parts.next).to.not.exist
    expectCheckAccess()

    stubS3SignUrl.restore()
    stubS3Create.restore()

  createFakeUpload = ->
    br = NogBlob.bucketRouter
    stubS3Create = sinon.stub(
      br, 'createMultipartUpload',
    ).callsFake (opts) ->
      fakeUploadId = Random.id()
      bucket = Random.id()
      _.extend opts, {Bucket: bucket, Key: opts.key, UploadId: fakeUploadId}
    stubS3SignUrl = sinon.stub(
      br, 'getSignedUploadPartUrl',
    ).callsFake (opts) ->
      return "https://#{opts.Bucket}/#{opts.Key}/#{opts.UploadId}?fakesig"

    req =
      params: {sha1: fakeFileSha}
      body: {name: 'a.txt', size: 2}
    res = actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(res.statusCode).to.equal 201
    expect(res.upload.id).to.equal fakeUploadId

    stubS3SignUrl.restore()
    stubS3Create.restore()

  it "
    'POST /:sha1/uploads' allows starting concurrent uploads of the same
    object.
  ", ->
    for i in [0...3]
      createFakeUpload()

  # This test should be moved to a test of the lower-level implementation.
  it "
    'GET /:sha1/uploads/:s3UploadId/parts' throws with invalid offset.
  ", ->
    req =
      params: {sha1: fakeFileSha, s3UploadId: fakeUploadId}
      query: {offset: 99}
    fn = -> actions['GET /:sha1/uploads/:s3UploadId/parts'](
        _.extend(req, basereq)
      )
    expect(fn).to.throw '[ERR_PARAM_INVALID]'

  # This test should be moved to a test of the lower-level implementation.
  it "
    'GET /:sha1/uploads/:s3UploadId/parts' throws when the upload id is
    unknown.
  ", ->
    req =
      params: {sha1: fakeFileSha, s3UploadId: 'unknown'}
    fn = -> actions['GET /:sha1/uploads/:s3UploadId/parts'](
        _.extend(req, basereq)
      )
    expect(fn).to.throw '[ERR_UPLOADID_UNKNOWN]'

  it "
    'GET /:sha1/uploads/:s3UploadId/parts' checks access and returns parts
    urls.
  ", ->
    br = NogBlob.bucketRouter
    stubS3SignUrl = sinon.stub(
      br, 'getSignedUploadPartUrl',
    ).callsFake (opts) ->
      return "https://#{opts.Bucket}/#{opts.Key}/#{opts.UploadId}?fakesig"

    req =
      params: {sha1: fakeFileSha, s3UploadId: fakeUploadId}
      query: {offset: 0, limit: 5}
    res = actions['GET /:sha1/uploads/:s3UploadId/parts'](
        _.extend(req, basereq)
      )
    expect(res.items).to.have.length 1
    expect(res.items[0].start).to.equal 0
    expect(res.items[0].end).to.equal 2
    expect(res.items[0].href).to.exist
    expectCheckAccess()

    stubS3SignUrl.restore()

  # This test should be moved to a test of the lower-level implementation.
  it "'POST /:sha1/uploads/:s3UploadId' throws when parts are missing.", ->
    req =
      params: {sha1: fakeFileSha, s3UploadId: fakeUploadId}
      body: {s3Parts: []}
    fn = -> actions['POST /:sha1/uploads/:s3UploadId'](_.extend(req, basereq))
    expect(fn).to.throw '[ERR_PARAM_INVALID]'

  # This test should be moved to a test of the lower-level implementation.
  it "'POST /:sha1/uploads/:s3UploadId' throws with too many parts.", ->
    req =
      params: {sha1: fakeFileSha, s3UploadId: fakeUploadId}
      body: {s3Parts: [{PartNumber: 1, ETag: '1'}, {PartNumber: 2, ETag: '2'}]}
    fn = -> actions['POST /:sha1/uploads/:s3UploadId'](_.extend(req, basereq))
    expect(fn).to.throw '[ERR_PARAM_INVALID]'

  # This test should be moved to a test of the lower-level implementation.
  it "'POST /:sha1/uploads/:s3UploadId' throws with unexpected parts.", ->
    req =
      params: {sha1: fakeFileSha, s3UploadId: fakeUploadId}
      body: {s3Parts: [{PartNumber: 2, ETag: '1'}]}
    fn = -> actions['POST /:sha1/uploads/:s3UploadId'](_.extend(req, basereq))
    expect(fn).to.throw '[ERR_PARAM_INVALID]'

  it "
    'POST /:sha1/uploads/:s3UploadId' checks access and returns an expanded
    blob; a concurrent upload is aborted.
  ", ->
    # Complete the second upload first and check that `firstUpload` gets
    # aborted upon POST.
    firstUpload = fakeUploadId
    createFakeUpload()
    stubS3Complete = sinon.stub NogBlob.bucketRouter, 'completeMultipartUpload'
    req =
      params: {sha1: fakeFileSha, s3UploadId: fakeUploadId}
      body: {s3Parts: [{PartNumber: 1, ETag: 'fakeETag'}]}
    res = actions['POST /:sha1/uploads/:s3UploadId'](_.extend(req, basereq))
    expect(res._id.id).to.equal fakeFileSha
    expect(res._id.href).to.contain(baseUrl + '/' + fakeFileSha)
    expect(res.sha1).to.equal fakeFileSha
    expect(res.statusCode).to.equal 201
    expectCheckAccess()

    stubS3Complete.reset()
    stubS3Abort = sinon.stub NogBlob.bucketRouter, 'abortMultipartUpload'
    req.params.s3UploadId = firstUpload
    res = actions['POST /:sha1/uploads/:s3UploadId'](_.extend(req, basereq))
    expect(stubS3Abort).to.have.been.called
    expect(stubS3Complete).to.have.not.been.called
    stubS3Abort.restore()
    stubS3Complete.restore()

  it "
    'POST /:sha1/uploads' throws 409 'conflict' if the blob exists.
  ", ->
    br = NogBlob.bucketRouter
    stubS3Create = sinon.stub(
      br, 'createMultipartUpload',
    ).callsFake (opts) ->
      fakeUploadId = Random.id()
      bucket = Random.id()
      _.extend opts, {Bucket: bucket, Key: opts.key, UploadId: fakeUploadId}

    req =
      params: {sha1: fakeFileSha}
      body: {name: 'a.txt', size: 2}
    fn = -> actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(fn).to.throw '[ERR_BLOB_UPLOAD_EXISTS]'
    expect(stubS3Create).to.have.not.been.called

    stubS3Create.restore()

  it "parts pagination", ->
    @timeout 4000
    br = NogBlob.bucketRouter
    stubS3Create = sinon.stub(
      br, 'createMultipartUpload',
    ).callsFake (opts) ->
      fakeUploadId = Random.id()
      bucket = Random.id()
      _.extend opts, {Bucket: bucket, Key: opts.key, UploadId: fakeUploadId}
    stubS3SignUrl = sinon.stub(
      br, 'getSignedUploadPartUrl',
    ).callsFake (opts) ->
      return "https://#{opts.Bucket}/#{opts.Key}/#{opts.UploadId}?fakesig"

    allParts = []
    addParts = (parts) ->
      for p in parts
        allParts[p.partNumber - 1] = p

    randomSha1 = Random.hexString 40
    GiB = 1024 * 1024 * 1024
    totalSize = 173 * GiB
    req =
      params: {sha1: randomSha1}
      query: {limit: 10}
      body: {name: 'fake.txt', size: totalSize}
    res = actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(res.parts.items).to.exist
    expect(res.parts.offset).to.equal 0
    expect(res.parts.limit).to.exist
    expect(res.parts.limit).to.equal 10
    expect(res.parts.items).to.have.length(res.parts.limit)

    addParts res.parts.items

    next = res.parts.next
    while next?
      parsed = urlparse next, 1
      {offset, limit} = parsed.query
      req =
        params: {sha1: randomSha1, s3UploadId: fakeUploadId}
        query: {offset, limit}
      res = actions['GET /:sha1/uploads/:s3UploadId/parts'](
          _.extend(req, basereq)
        )
      expect(res.items).to.exist
      expect(res.offset).to.equal Number(offset)
      expect(res.limit).to.exist
      if res.next?
        expect(res.limit).to.equal Number(limit)
      addParts res.items
      next = res.next

    end = allParts[0].end
    for p in allParts[1..]
      expect(p.start).to.equal end
      end = p.end
    expect(end).to.equal totalSize

    stubS3SignUrl.restore()
    stubS3Create.restore()

  it "
    'POST /:sha1/uploads' support blob size 0.
  ", ->
    br = NogBlob.bucketRouter
    stubS3Create = sinon.stub(
      br, 'createMultipartUpload',
    ).callsFake (opts) ->
      fakeUploadId = Random.id()
      bucket = Random.id()
      _.extend opts, {Bucket: bucket, Key: opts.key, UploadId: fakeUploadId}
    stubS3SignUrl = sinon.stub(
      br, 'getSignedUploadPartUrl',
    ).callsFake (opts) ->
      return "https://#{opts.Bucket}/#{opts.Key}/#{opts.UploadId}?fakesig"

    req =
      params: {sha1: emptyFileSha1}
      body: {name: 'empty.txt', size: 0}
    res = actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(res.statusCode).to.equal 201
    expect(res.parts.count).to.equal 1
    expect(res.parts.items).to.have.length 1
    expect(res.parts.next).to.not.exist

    stubS3SignUrl.restore()
    stubS3Create.restore()

  it "
    'POST /:sha1/uploads' rejects invalid size.
  ", ->
    req =
      params: {sha1: fakeFileSha}
      body: {name: 'a.txt', size: -1}
    fn = -> actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(fn).to.throw 'Match error'
    expect(fn).to.throw 'Expected non-negative number'


describe 'nog-blob', -> describe 'UploadsApi (with repoSets)', ->
  fakeUploadId = null
  baseUrl = '/baseurl'
  basereq =
    baseUrl: baseUrl
    auth: {user: 'fakeUser'}

  ownerName = 'fakeOwner'
  repoName = Random.id()

  repoSets = {updateMembership: sinon.spy()}
  api = new NogBlobTest.UploadsApi {repoSets: repoSets}
  actions = {}
  for a in api.actions()
    actions[a.method + ' ' + a.path] = a.action

  # Isolate the tests from the global access statements when running together
  # with nog-access.
  stubCheckAccess = null
  beforeEach -> stubCheckAccess = sinon.stub NogBlob, 'checkAccess'
  afterEach -> stubCheckAccess.restore()

  it 'updates RepoSets on complete upload', ->
    br = NogBlob.bucketRouter
    stubS3Create = sinon.stub(
      br, 'createMultipartUpload',
    ).callsFake (opts) ->
      fakeUploadId = Random.id()
      bucket = Random.id()
      _.extend opts, {Bucket: bucket, Key: opts.key, UploadId: fakeUploadId}
    stubS3SignUrl = sinon.stub(
      br, 'getSignedUploadPartUrl',
    ).callsFake (opts) ->
      return "https://#{opts.Bucket}/#{opts.Key}/#{opts.UploadId}?fakesig"

    req =
      params: {ownerName, repoName, sha1: fakeFileSha}
      body: {name: 'a.txt', size: 2}

    fn = -> actions['POST /:sha1/uploads'](_.extend(req, basereq))
    expect(fn).to.throw '[ERR_BLOB_UPLOAD_EXISTS]'
    expect(repoSets.updateMembership).to.have.been.calledWith(
        sinon.match({ownerName, repoName}),
        sinon.match({type: 'blob', sha1: fakeFileSha})
      )
    repoSets.updateMembership.reset()

    NogBlob.blobs.remove {}
    res = actions['POST /:sha1/uploads'](_.extend(req, basereq))

    stubS3SignUrl.restore()
    stubS3Create.restore()

    stubS3Complete = sinon.stub NogBlob.bucketRouter, 'completeMultipartUpload'
    req =
      params: {
        ownerName, repoName, sha1: fakeFileSha, s3UploadId: fakeUploadId
      }
      body: {s3Parts: [{PartNumber: 1, ETag: 'fakeETag'}]}
    res = actions['POST /:sha1/uploads/:s3UploadId'](_.extend(req, basereq))
    stubS3Complete.restore()

    expect(repoSets.updateMembership).to.have.been.calledWith(
        sinon.match({ownerName, repoName}),
        sinon.match({type: 'blob', sha1: fakeFileSha})
      )

  it 'repoSets can be injected by calling configure()', ->
    oldCfg = NogBlob.configure {repoSets: false}
    expect(NogBlob.api.uploads.repoSets).to.not.exist
    NogBlob.configure { repoSets: {} }
    expect(NogBlob.api.uploads.repoSets).to.exist
    NogBlob.configure oldCfg
