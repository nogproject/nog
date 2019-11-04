# Workaround for `require` to avoid `Cannot read property of 'Stream'` during
# package testing with Meteor 1.3, see
# <https://forums.meteor.com/t/error-in-test-mode-on-1-3-typeerror-cannot-read-property-stream-of-undefined/21696/7>.
# The workaround that resets `process.browser` as described does not work
# reliably.  Instead, simply delete `process.browser` and keep it deleted,
# since this is server-only code.

delete process.browser
AWS = Npm.require 'aws-sdk'

# The AWS API is used through explicit events 'success' and 'error' to avoid
# the callback API, since callbacks have been observed to be spuriously called,
# causing false results and duplicate resolves of futures.

S3 = {}

config =
  accessKeyId: Meteor.settings.AWSAccessKeyId
  secretAccessKey: Meteor.settings.AWSSecretAccessKey
  region: Meteor.settings.AWSBucketRegion
  signatureVersion: Meteor.settings.AWSSignatureVersion ? 'v4'
  s3ForcePathStyle: Meteor.settings.AWSS3ForcePathStyle ? false
  endpoint: Meteor.settings.AWSEndpoint
  sslEnabled: Meteor.settings.AWSSslEnabled ? true
  ca: Meteor.settings.AWSCa

Future = Npm.require 'fibers/future'
fs = Npm.require 'fs'
https = Npm.require 'https'

# Set all simulations to false for production.
simulate =
  create: -> false
  complete: -> false
  abort: -> false

{
  createError
  ERR_S3_CREATE_MULTIPART
  ERR_S3_ABORT_MULTIPART
  ERR_S3_COMPLETE_MULTIPART
} = NogError

# `_s3` is the internal service object.
_s3 = null

# Node does not use the system CAs.  So we provide an option to configure the
# CA by injecting an https.Agent with a custom CA list.
#
# See general idea: <http://stackoverflow.com/a/31059809>,
# <https://github.com/aws/aws-sdk-js/issues/662#issuecomment-121678212>.
#
# See implementation details:
# <https://github.com/aws/aws-sdk-js/blob/7d02a64bc559a0edba3799dae42a3f964cfb9c28/lib/http/node.js#L35>
# <https://github.com/aws/aws-sdk-js/blob/7d02a64bc559a0edba3799dae42a3f964cfb9c28/lib/http/node.js#L94>
#
# The CA file is split into a list for compatibility with Node 0.10.40, see
# <https://nodejs.org/docs/v0.10.40/api/https.html>, which is used by Meteor
# 1.2, see <https://github.com/meteor/meteor/blob/devel/History.md>.

sslAgent = (ca) ->
  rgx = /(?=-----BEGIN CERTIFICATE-----)/
  agent = new https.Agent {
    ca: fs.readFileSync(ca).toString().split(rgx)
    rejectUnauthorized: true
  }
  agent.setMaxListeners(0)
  agent

# `ca` is handled explicitly.  All other fields are named such that they can be
# directly passed to AWS.S3(); see:
#
# - General options:
#   <http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/Config.html#constructor-property>
# - S3 options:
#   <http://docs.aws.amazon.com/AWSJavaScriptSDK/latest/AWS/S3.html#constructor-property>

init_s3 = ->
  c = _.omit(config, 'ca')
  if config.ca?
    console.log "[nog-s3] Using SSL CA bundle `#{config.ca}`."
    c.httpOptions = {agent: sslAgent(config.ca)}
  _s3 = new AWS.S3(c)


# Human-readable message first to help operator.  Then strict check to ensure a
# sane config.
checkConfig = (cfg) ->
  hint = "
      Either set `Meteor.settings.AWSAccessKeyId` or call
      `S3.configure({accessKeyId: <keyid>, secretAccessKey: <key>, region:
      <region>});`.
    "
  if not cfg.accessKeyId?
    console.error "[nog-s3] Missing AWS access key id. #{hint}"
  if not cfg.secretAccessKey?
    console.error "[nog-s3] Missing AWS secret access key. #{hint}"
  if not cfg.region?
    console.error "[nog-s3] Missing AWS region. #{hint}"

  check cfg.accessKeyId, String
  check cfg.secretAccessKey, String
  check cfg.region, String
  check cfg.signatureVersion, String
  check cfg.s3ForcePathStyle, Match.Optional(Boolean)
  check cfg.endpoint, Match.Optional(String)
  check cfg.sslEnabled, Boolean
  check cfg.ca, Match.Optional(String)

  if cfg.signatureVersion not in ['s3', 'v4']
    console.error "[nog-s3] Invalid AWS signature version. #{hint}"
  if cfg.region == 'eu-central-1' and cfg.signatureVersion == 's3'
    console.error "
      [nog-s3] Invalid AWS signature version for region `eu-central-1`; it
      requires `v4`.
    "

Meteor.startup ->
  checkConfig config
  init_s3()

S3.configure = (cfg) ->
  for k, v of cfg
    if not (k of config)
      throw new Meteor.Error 'config', "Unknown nog-s3 config key '#{k}'."
    if _.isNull(v)
      config[k] = undefined
    else
      config[k] = v
  checkConfig config
  # Reinitialize if _s3 has already been initialized.
  if _s3?
    init_s3()


S3.createMultipartUpload = (opts) ->
  if simulate.create()
    r = _.pick opts, 'Bucket', 'Key'
    r = _.extend r, {UploadId: 'fake-upload-id'}
    return r

  fut = new Future
  req = _s3.createMultipartUpload opts
  req.on 'success', (res) ->
    fut.return res.data
  req.on 'error', (err) ->
    fut.throw createError ERR_S3_CREATE_MULTIPART,
      cause: err
      s3Bucket: opts.Bucket
      s3ObjectKey: opts.Key
  req.send()
  fut.wait()


S3.getSignedUploadPartUrl = (opts) ->
  _s3.getSignedUrl 'uploadPart', opts

S3.getSignedDownloadUrl = (opts) ->
  _s3.getSignedUrl 'getObject', opts


S3.completeMultipartUpload = (opts) ->
  if simulate.complete()
    return console.log 'Would completeMultipartUpload', opts

  fut = new Future
  req = _s3.completeMultipartUpload opts
  req.on 'success', (res) ->
    fut.return res.data
  req.on 'error', (err) ->
    fut.throw createError ERR_S3_COMPLETE_MULTIPART,
      cause: err
      s3Bucket: opts.Bucket
      s3ObjectKey: opts.Key
      s3UploadId: opts.UploadId
  req.send()
  fut.wait()


S3.abortMultipartUpload = (opts) ->
  if simulate.abort()
    return console.log 'Would call S3.abortMultipartUpload', opts

  fut = new Future
  req = _s3.abortMultipartUpload opts
  req.on 'success', (res) ->
    fut.return res.data
  req.on 'error', (err) ->
    fut.throw createError ERR_S3_ABORT_MULTIPART,
      cause: err
      s3Bucket: opts.Bucket
      s3ObjectKey: opts.Key
      s3UploadId: opts.UploadId
  req.send()
  fut.wait()
