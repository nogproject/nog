# To run the tests, provide --settings:
exampleSettings = {
    "AWSAccessKeyId": "<keyid>",
    "AWSSecretAccessKey": "<secretkey>",
    "AWSBucketRegion": "<region>",
    "tests": {
        "aws": {
          "testbucket": "<bucket>"
        }
    }
}
# A production bucket can be used as a test bucket at the same time.  The tests
# will use object keys that should not conflict with real objects.

request = Npm.require 'request'
crypto = Npm.require 'crypto'

describe 'nog-s3', ->
  describe 'tests environment', ->
    it 'Meteor.settings contains the required AWS config.', ->
      expect(Meteor.settings.AWSAccessKeyId).to.exist
      expect(Meteor.settings.AWSSecretAccessKey).to.exist
      expect(Meteor.settings.AWSBucketRegion).to.exist
      expect(Meteor.settings.tests.aws.testbucket).to.exist

  describe 'config', ->
    it 'S3 takes its default config from Meteor.settings', ->
      url = S3.getSignedDownloadUrl {Bucket: 'x', Key: 'x'}
      m = url.match /// X-Amz-Credential=([^&%]*) ///
      expect(m[1]).to.equal Meteor.settings.AWSAccessKeyId

    describe 'S3.configure() updates the config:', ->
      it 'credentials', ->
        fakeKeyId = 'fakeKeyId'
        S3.configure
          accessKeyId: fakeKeyId
          secretAccessKey: 'invalid'
          region: 'fakeregion'
        url = S3.getSignedDownloadUrl {Bucket: 'b', Key: 'x'}
        expect(url).to.contain 'fakeregion'
        m = url.match /// X-Amz-Credential=([^&%]*) ///
        expect(m[1]).to.equal fakeKeyId
        S3.configure
          accessKeyId: Meteor.settings.AWSAccessKeyId
          secretAccessKey: Meteor.settings.AWSSecretAccessKey
          region: Meteor.settings.AWSBucketRegion

      it 'signatureVersion', ->
        fn = -> S3.getSignedDownloadUrl {Bucket: 'b', Key: 'x'}
        expect(fn()).to.contain 'Amz-Algorithm'
        S3.configure {signatureVersion: 's3'}
        expect(fn()).to.not.contain 'Amz-Algorithm'
        S3.configure {signatureVersion: 'v4'}
        expect(fn()).to.contain 'Amz-Algorithm'

      it 'path style', ->
        fn = -> S3.getSignedDownloadUrl {Bucket: 'foobucket', Key: 'x'}
        expect(fn()).to.contain 'https://foobucket'
        S3.configure {s3ForcePathStyle: true}
        expect(fn()).to.contain '.com/foobucket/x'
        S3.configure {s3ForcePathStyle: false}
        expect(fn()).to.contain 'https://foobucket'

      it 'endpoint', ->
        fn = -> S3.getSignedDownloadUrl {Bucket: 'foobucket', Key: 'x'}
        expect(fn()).to.contain 'https://foobucket.s3-'
        for unset in [undefined, null]
          S3.configure {endpoint: 'https://example.com'}
          expect(fn()).to.contain 'https://foobucket.example.com'
          S3.configure {endpoint: unset}
          expect(fn()).to.contain 'https://foobucket.s3-'

      it 'ssl', ->
        fn = -> S3.getSignedDownloadUrl {Bucket: 'foobucket', Key: 'x'}
        expect(fn()).to.contain 'https://foobucket.s3-'
        S3.configure {sslEnabled: false}
        expect(fn()).to.contain 'http://foobucket.s3-'
        S3.configure {sslEnabled: true}
        expect(fn()).to.contain 'https://foobucket.s3-'

      it 'the SSL CA setup is not automatically tested'

  describe 'download', ->
    it 'getSignedDownloadUrl() returns a URL with the expected format', ->
      bucket = 'fakebucket'
      key = '12345'
      localfile = 'fakefilename'
      url = S3.getSignedDownloadUrl
        Bucket: bucket
        Key: key
        ResponseContentDisposition: 'attachment; filename="' + localfile  + '"'
      expect(url).to.match /// ^https://fakebucket.[^.]*.amazonaws.com/12345 ///
      expect(url).to.match /// X-Amz-Credential= ///
      expect(url).to.match /// Expires= ///
      expect(url).to.match /// Signature= ///
      expect(url).to.match(
        /// response-content-disposition=attachment.*fakefilename ///
      )

  describe 'upload', ->
    testbucket = Meteor.settings.tests.aws.testbucket
    testkey = '__test__object'

    # Skip, since it timed out repeatedly without reason.
    it.skip '
      createMultipartUpload() throws an exception with an invalid bucket.
    ', ->
      fn = -> S3.createMultipartUpload {Bucket: 'invalid', Key: 'invalid'}
      expect(fn).to.throw 'S3_CREATE_MULTIPART'

    it 'getSignedUploadPartUrl() returns a URL with the expected format.', ->
      bucket = 'fakebucket'
      key = '12345'
      localfile = 'fakefilename'
      url = S3.getSignedUploadPartUrl
        Bucket: 'fakebucket'
        Key: '12345'
        UploadId: 'fakeid'
        PartNumber: 1
      expect(url).to.match /// ^https://fakebucket.[^.]*.amazonaws.com/12345 ///
      expect(url).to.match /// X-Amz-Credential= ///
      expect(url).to.match /// Expires= ///
      expect(url).to.match /// Signature= ///
      expect(url).to.match /// partNumber= ///
      expect(url).to.match /// uploadId= ///

    # Use only a single part.  Multiple parts would require a larger total size
    # Each part must be at least 5 MB in size, except the last part,
    # <http://docs.aws.amazon.com/AmazonS3/latest/API/mpUploadComplete.html>.
    it.ifRealAws '
      A multipart upload creates the expected s3 object.
    ', (done) ->
      @timeout 20000
      fakeContent = crypto.randomBytes(20).toString('hex')

      opts = { Bucket: testbucket, Key: testkey }
      upload = S3.createMultipartUpload opts

      opts.UploadId = upload.UploadId
      opts.PartNumber = 1
      request {
        method: 'PUT'
        url: S3.getSignedUploadPartUrl opts
        body: fakeContent
      }, Meteor.bindEnvironment (err, res, body) ->
        parts = [
          PartNumber: opts.PartNumber
          ETag: res.headers.etag
        ]
        opts = _.pick opts, 'Bucket', 'Key', 'UploadId'
        opts.MultipartUpload = { Parts: parts }
        S3.completeMultipartUpload opts

        url = S3.getSignedDownloadUrl { Bucket: testbucket, Key: testkey }
        request.get url, (err, res, body) ->
          expect(body).to.equal fakeContent
          done()

    it.ifRealAws '
      abortMultipartUpload() leaves the S3 object alone.
    ', (done) ->
      @timeout 20000
      fakeContent = crypto.randomBytes(20).toString('hex')

      opts = { Bucket: testbucket, Key: testkey }
      upload = S3.createMultipartUpload opts

      opts.UploadId = upload.UploadId
      opts.PartNumber = 1
      request {
        method: 'PUT'
        url: S3.getSignedUploadPartUrl opts
        body: fakeContent
      }, Meteor.bindEnvironment (err, res, body) ->
        S3.abortMultipartUpload _.pick(opts, 'Bucket', 'Key', 'UploadId')
        url = S3.getSignedDownloadUrl _.pick(opts, 'Bucket', 'Key')
        request.get url, (err, res, body) ->
          expect(
            (res.status isnt 200) or (body isnt fakeContent)
          ).to.be.true
          done()

    # Skip, since it timed out repeatedly without reason.
    it.skip '
      completeMultipartUpload() throws an exception with invalid params.
    ', ->
      fn = -> S3.completeMultipartUpload
        Bucket: 'invalid'
        Key: 'invalid'
        UploadId: 'invalid'
        MultipartUpload: {Parts: []}
      expect(fn).to.throw 'S3_COMPLETE_MULTIPART'

    # Skip, since it timed out repeatedly without reason.
    it.skip '
      abortMultipartUpload() throws an exception with invalid params.
    ', ->
      fn = -> S3.abortMultipartUpload
        Bucket: 'invalid'
        Key: 'invalid'
        UploadId: 'invalid'
      expect(fn).to.throw 'S3_ABORT_MULTIPART'
