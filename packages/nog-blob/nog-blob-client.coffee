# See also
#
#     https://github.com/TTLabs/EvaporateJS/blob/master/evaporate.js
#     https://github.com/ienzam/s3-multipart-upload-browser

{ createMd5Hasher } = require './nog-blob-client-md5.js'

{
  createError
  ERR_BLOB_UPLOAD
  ERR_BLOB_UPLOAD_START
  ERR_BLOB_UPLOAD_WARN
  ERR_BLOB_COMPUTE_SHA1
  ERR_BLOB_DOWNLOAD
} = NogError

reportError = (args...) -> NogBlob.onerror createError(args...)


retryPolicy = {
  nTries: -> Meteor.settings.public.upload.uploadRetries + 1
  retryDelayS: (i) ->
    delays = [0, 2, 5, 10, 30, 60, 120, 180]
    i = Math.max(0, Math.min(i, delays.length - 1))
    return delays[i]
}


# `createSemaphore()` returns a semaphore that can be acquired at most
# `maxAcquireFn()` times.  Further attempts to `acquire(cb)` are queued, and
# the `cb()` is called as soon as a slot has become free by a call to
# `release()`.  `maxAcquireFn()` is a function to support changing the limit at
# runtime.

createSemaphore = (name, maxAcquireFn) -> return {
  name
  maxAcquireFn
  acquired: 0
  queue: []

  acquire: (cb) ->
    if @acquired < @maxAcquireFn()
      @acquired += 1
      Meteor.defer(cb)
    else
      @queue.push(cb)

  release: ->
    if (cb = @queue.shift())?
      Meteor.defer(cb)
    else
      @acquired -= 1
}


# `putSlots` is used to limit the number of concurrent PUT.
#
# `uploadSlots` is used to limit the number of concurrent uploads.
#
# It seems to be a good idea, in general, to first finish uploads before
# starting more, although the primary reason for the limits are more reliable
# uploads with Safari.  Four concurrent PUTs to a local Ceph worked reasonably
# well.  With a greater concurrency, ETag errors appeared increasingly
# frequently.

# Chrome has both in userAgent.  Safari has only one.

isChrome = navigator.userAgent.indexOf('Chrome') > -1
isSafari = (navigator.userAgent.indexOf('Safari') > -1) and not isChrome

if isSafari
  putSlots = createSemaphore(
    'putSlots', -> Meteor.settings.public.upload.concurrentPutsSafari
  )
else
  putSlots = createSemaphore(
    'putSlots', -> Meteor.settings.public.upload.concurrentPuts
  )

uploadSlots = createSemaphore(
  'uploadSlots', -> Meteor.settings.public.upload.concurrentUploads
)


simulate =
  putok: -> false

# <http://www.w3.org/TR/XMLHttpRequest/#states>
XHR_DONE = 4

HTTP_OK = 200

STATUS =
  SHA1: 'sha1'
  KNOWN: 'known'
  STARTING: 'starting'
  UPLOADING: 'uploading'
  DONE: 'done'
  ABORTED: 'aborted'

class MultipartUpload
  INIT: 'init'
  UPLOADING: STATUS.UPLOADING
  ABORTED: STATUS.ABORTED
  DONE: STATUS.DONE
  KNOWN: STATUS.KNOWN

  constructor: (@file, @sha1) ->
    @nTries = retryPolicy.nTries()
    @onerror = ->
    @onwarning = ->

    # `onstatus(status)` is called on status changes.  Possible sequences:
    #
    #   1) -> uploading -> aborted | done.
    #   2) -> known.
    #
    @onstatus = ->

    # `onprogress()` is called to report progress with `{uploaded:
    # <nBytes:Number>, progress: <0..100>}`.
    @onprogress = ->

    # `_active` contains the active parts (maps partNumber to part object).
    @_active = {}

    # `_todo` is a list of remaining parts that are not yet activated.
    # Part objects are minimally initialized.
    @_todo = []

    # `_state` is used internally: init, uploading, aborted.
    @_state = @INIT

    # `@_uploaded` tracks the total size of all completed parts.  The progress
    # of the active parts is tracked in `part.uploaded`.
    @_uploaded = 0

  _reportProgress: ->
    u = @_uploaded
    for k, part of @_active
      u += part.uploaded
    p = Math.round 100 * u / @file.size
    @onprogress
      uploaded: u
      progress: p

  # `_whenactive` calls `next` only if the upload has not been aborted.  Use it
  # to wrap callbacks to avoid running them unnecessarily.
  _whenactive: (next) ->
    if @_state isnt @ABORTED
      next()

  start: () ->
    NogBlob.call.startMultipartUpload
      name: @file.name
      size: @file.size
      sha1: @sha1
    , (err, res) => @_whenactive =>
      if err
        @_abort()
        @onerror createError ERR_BLOB_UPLOAD_START,
          cause: err
          fileSize: @file.size
          fileName: @file.name
          sha1: @sha1
        return
      if res is 'known'
        @onstatus @KNOWN
        return
      @_state = @UPLOADING
      @onstatus @UPLOADING
      @_s3UploadId = res.s3UploadId
      @_todo = _.clone res.startParts
      if res.nParts > res.startParts.length
        for i in [(res.startParts.length + 1)..res.nParts]
          @_todo.push
            partNumber: i
      for p in res.startParts
        @_activateOne()

  _activateOne: () ->
    part = @_todo.shift()
    if not part?
      return @_checkDone()
    part.try = 0
    part.uploaded = 0
    @_active[part.partNumber] = part
    @_sendPart part

  _sendPart: (part) ->
    part.try++
    if part.url?
      @_put part
    else
      NogBlob.call.getUploadParts
        s3UploadId: @_s3UploadId
        partNumbers: [part.partNumber]
      , (err, res) => @_whenactive =>
        if err
          handleGetUploadPartsError err
        else
          _.extend part, res.parts[0]
          @_put part

    handleGetUploadPartsError = (err) =>
      if part.try < @nTries
        time_sec = @_scheduleRetrySend part
        @onwarning createError ERR_BLOB_UPLOAD_WARN,
          cause: err
          reason: "
              Failed to get upload URL for part #{part.partNumber}; try
              #{part.try}/#{@nTries}; retrying in #{time_sec} seconds.
            "
        return
      else
        @_abort()
        @onerror createError ERR_BLOB_UPLOAD,
          cause: err
          reason: "
              Failed to get upload URL for part #{part.partNumber}; aborting
              after #{@nTries} tries.
            "
        return

  # `_put` verifies the ETag from the S3 response before continuing.  To do so,
  # it uploads to S3 and in parallel computes the MD5 locally.
  #
  # We've measured MD5 hash rates of several 10 MB/s up to more than 100 MB/s,
  # so the local MD5 computation usually does not slow down the upload.
  #
  # The MD5 is computed only during the first try and skipped during retries.

  _put: (part) ->
    req = null

    onReqReadyStateChange = => @_whenactive =>
      if req.readyState isnt XHR_DONE
        return
      else if simulate.putok()
        console.log "Simulating successful PUT of part #{part.partNumber}."
        part.etag = 'dummy'
        part.try = 0
        @_pushPart part
      else if req.status isnt HTTP_OK
        handleHTTPError()
      else
        part.etag = req.getResponseHeader 'ETag'
        continueAfterMd5Check()

    onReqProgress = (e) => @_whenactive =>
      if e.lengthComputable
        part.uploaded = e.loaded
        @_reportProgress()
        return

    onHashSuccess = (md5) =>
      part.md5 = md5
      continueAfterMd5Check()

    onHashError = (err) =>
      handleHashError()

    continueAfterMd5Check = =>
      unless part.etag? and part.md5?
        return
      if part.etag != "\"#{part.md5}\""
        handleEtagError()
        return
      part.try = 0
      putSlots.release()
      @_pushPart part

    handleHTTPError = =>
      putSlots.release()
      if part.try < @nTries
        time_sec = @_scheduleRetrySend part
        @onwarning createError ERR_BLOB_UPLOAD_WARN,
          reason: "
              HTTP PUT to S3 failed for part #{part.partNumber}; try
              #{part.try}/#{@nTries}; retrying in #{time_sec} seconds.
              Status code: #{req.status}.
            "
      else
        @_abort()
        @onerror createError ERR_BLOB_UPLOAD,
          reason: "
              HTTP PUT to S3 failed for part #{part.partNumber}; aborting after
              #{@nTries} tries., Status code: #{req.status}.
            "

    handleEtagError = =>
      putSlots.release()
      if part.try < @nTries
        time_sec = @_scheduleRetrySend part
        @onwarning createError ERR_BLOB_UPLOAD_WARN,
          reason: "
            S3 ETag verification failed for part #{part.partNumber};
            try #{part.try}/#{@nTries}; retrying in #{time_sec} seconds.
          "
          details: "
            Computed \"#{part.md5}\" locally;
            S3 reported #{part.etag}.
          "
      else
        @_abort()
        @onerror createError ERR_BLOB_UPLOAD,
          reason: "
            S3 ETag verification failed for part #{part.partNumber};
            aborting after #{@nTries} tries.
          "
          details: "
            Computed \"#{part.md5}\" locally;
            S3 reported #{part.etag}.
          "

    handleHashError = =>
      putSlots.release()
      @_abort()
      @onerror createError ERR_BLOB_UPLOAD,
        reason: "MD5 computation for part #{part.partNumber} failed."

    putSlots.acquire =>
      req = new XMLHttpRequest()
      part.req = req
      req.onreadystatechange = onReqReadyStateChange
      req.upload.onprogress = onReqProgress
      isAsync = true
      req.open 'PUT', part.url, isAsync
      req.send @file.slice part.start, part.end

    unless part.md5?  # Skip during retries.
      hasher = createMd5Hasher(@file.slice(part.start, part.end))
      hasher.onsuccess = onHashSuccess
      hasher.onerror = onHashError
      hasher.start()

  _scheduleRetrySend: (part) ->
    part.url = undefined
    cb = => @_whenactive =>
      @_sendPart part
    time_sec = retryPolicy.retryDelayS(part.try)
    setTimeout cb, time_sec * 1000
    return time_sec

  _pushPart: (part) ->
    part.try++
    NogBlob.call.pushUploadedPart
      s3UploadId: @_s3UploadId
      partNumber: part.partNumber
      etag: part.etag
    , (err, res) => @_whenactive =>
      if err
        handlePushError err
      else
        @_uploaded += part.end - part.start
        delete @_active[part.partNumber]
        @_reportProgress()
        @_activateOne()

    handlePushError = (err) =>
      if part.try < @nTries
        time_sec = @_scheduleRetryPushPart part
        @onwarning createError ERR_BLOB_UPLOAD_WARN,
          cause: err
          reason: "
              Failed to report S3 upload of part #{part.partNumber}; try
              #{part.try}/#{@nTries}; retrying in #{time_sec} seconds.
            "
      else
        @_abort()
        @onerror createError ERR_BLOB_UPLOAD,
          cause: err
          reason: "
              Failed to report S3 upload of part #{part.partNumber}; aborting
              after #{@nTries} tries.
            "

  _scheduleRetryPushPart: (part) ->
    cb = => @_whenactive =>
      @_pushPart part
    time_sec = part.try * 2
    setTimeout cb, part.try * time_sec * 1000
    return time_sec

  _checkDone: ->
    if @_todo.length > 0
      return
    if Object.keys(@_active).length > 0
      return
    @_completeUpload()

  _completeUpload: ->
    NogBlob.call.completeMultipartUpload
      s3UploadId: @_s3UploadId
    , (err, res) => @_whenactive =>
      if err
        @_abort()
        @onerror createError ERR_BLOB_UPLOAD,
          reason: 'Failed to complete multipart upload.'
          cause: err
      else
        @onstatus @DONE

  _abort: () ->
    if @_state is @ABORTED
      return

    if @_state is @INIT
      # Nothing to abort.
      @_state = @ABORTED
      @onstatus @ABORTED
      return

    # Cleanup if @UPLOADING.
    @_state = @ABORTED
    for own k, part of @_active
      part.req?.abort()
    @onstatus @ABORTED

    NogBlob.call.abortMultipartUpload
      s3UploadId: @_s3UploadId
    , (err, res) =>
      # Only report abort errors, but do not retry, since we cannot do anything
      # reasonable here.  The server will clean up pending multi part uploads
      # after a timeout.
      if err
        @onerror createError ERR_BLOB_UPLOAD,
          reason: 'Failed to abortMultipartUpload'
          cause: err


class Hasher
  constructor: (@file) ->
    # `onprogress()` is called to report progress with `{processed:
    # <nBytes:Number>, progress: <0..100>}`.
    @onprogress = ->

    # `onsuccess()` is called with the hash.
    @onsuccess = ->

    # `onerror()` is called with Meteor.Error.
    @onerror = ->

  start: ->
    rusha = new Worker '/packages/nog-blob/js/rusha-b601dbae5b34a4a08fbf7cc7252a940443a45cde.js'
    rusha.onmessage = (e) =>
      if e.data.event is 'progress'
        @onprogress
          processed: e.data.processed
          progress: Math.round e.data.processed / @file.size * 100
      else if e.data.event is 'success'
        @onsuccess e.data.hash
      else if e.data.event is 'error'
        @onerror createError ERR_BLOB_COMPUTE_SHA1, {cause: e.data.error}
    rusha.postMessage
      id: 0
      data: @file

NogBlob.uploadFile = (file, done) ->
  if _.isObject(done) and not _.isFunction(done)
    { done, onwarning } = done
    done ?= ->
    onwarning ?= ->
  else
    done ?= ->
    onwarning = (err) -> NogBlob.onerror(err)

  _id = null

  computeSha1 = () ->
    hasher = new Hasher file
    hasher.onprogress = (p) ->
      NogBlob.files.update _id, {$set: {sha1Progress: p.progress}}
    hasher.onsuccess = (h) ->
      sha1 = h
      NogBlob.files.update _id,
        $set:
          sha1: sha1
          status: STATUS.STARTING
      upload sha1
    hasher.onerror = (cause) ->
      err = createError ERR_BLOB_UPLOAD,
        reason: 'Computing sha1 failed.'
        cause: cause
      finish(err)
    hasher.start()

  upload = (sha1) ->
    upl = new MultipartUpload file, sha1
    upl.onerror = (cause) ->
      err = createError ERR_BLOB_UPLOAD,
        reason: 'Upload failed.'
        cause: cause
      finish(err)
    upl.onwarning = (cause) ->
      err = createError ERR_BLOB_UPLOAD_WARN,
        reason: 'Upload warning.'
        cause: cause
      onwarning(err)
    upl.onstatus = (s) ->
      NogBlob.files.update _id, {$set: {status: s}}
      if (s is STATUS.DONE) or (s is STATUS.KNOWN)
        finish(null, {_id, filename: file.name, size: file.size, sha1: sha1})
    upl.onprogress = (p) ->
      NogBlob.files.update _id,
        $set:
          uploaded: p.uploaded
          uploadProgress: p.progress
    upl.start()

  didCallDone = false
  finish = (err, res) ->
    if didCallDone
      return
    didCallDone = true
    uploadSlots.release()
    done(err, res)

  _id = NogBlob.files.insert
    name: file.name
    size: file.size
    sha1Progress: 0
    uploadProgress: 0
    status: STATUS.SHA1

  uploadSlots.acquire ->
    computeSha1()

  return _id


Meteor.startup ->
  Tracker.autorun ->
    # Create a reactive list of the sha1s from Files and subscribe to the
    # corresponding Blobs.
    sel = {sha1: {$exists: true}}
    proj = {fields: {sha1: true}}
    sha1s = _.pluck NogBlob.files.find(sel, proj).fetch(), 'sha1'
    Meteor.subscribe 'nog-blob/blobs', sha1s

NogBlob.fileHelpers =
  name: -> @name
  size: -> @size
  sha1Progress: -> @sha1Progress
  haveSha1: -> @sha1?
  sha1: -> @sha1
  uploaded: ->
    switch @status
      when STATUS.ABORTED then 0
      when STATUS.KNOWN
        blob = NogBlob.blobs.findOne @sha1
        if blob? then switch blob.status
          when 'available' then @size
          else 0
        else 0
      else @uploaded
  progress: ->
    switch @status
      when STATUS.ABORTED then 0
      when STATUS.KNOWN
        blob = NogBlob.blobs.findOne @sha1
        if blob? then switch blob.status
          when 'available' then 100
          else 0
        else 0
      else @uploadProgress
  progressWidth: ->
    switch @status
      when STATUS.ABORTED, STATUS.KNOWN then 100
      else @uploadProgress
  status: ->
    switch @status
      when STATUS.SHA1 then 'computing sha1...'
      when STATUS.KNOWN
        blob = NogBlob.blobs.findOne @sha1
        if blob? then switch blob.status
          when 'uploading' then 'waiting for other upload...'
          when 'available' then 'available from other upload.'
          when 'missing' then 'missing after other upload failed.'
          else "known, with unexpected status '#{blob.status}'."
        else 'known, waiting for details...'
      when STATUS.STARTING then 'starting upload...'
      when STATUS.UPLOADING then 'uploading...'
      when STATUS.DONE then 'available after upload.'
      when STATUS.ABORTED then 'upload failed.'
  uploadCompleteClass: ->
    switch @status
      when STATUS.DONE then 'progress-bar-success'
      when STATUS.ABORTED then 'progress-bar-danger'
      when STATUS.KNOWN
        blob = NogBlob.blobs.findOne @sha1
        if blob then switch blob.status
          when 'available' then 'progress-bar-success'
          when 'missing' then 'progress-bar-danger'
          else ''
        else ''
      else ''

Template.uploadItem.helpers NogBlob.fileHelpers


# The template `aBlobHref` expects a data context with `blob` and `name`.  The
# name will be used for displaying and as the filename during download.
#
# The idea for automatic downloading by using `createElement()` is described at
# <http://pixelscommander.com/en/javascript/javascript-file-download-ignore-content-type/>
#
# The download attribute is not supported in Safari, see
# <http://caniuse.com/#feat=download>.  But since we use the
# `Content-Disposition` header, it works nonetheless.
#
Template.aBlobHref.events
  'click .js-nog-blob-download': (ev) ->
    ev.preventDefault()
    ev.stopPropagation()
    sha1 = @blob
    filename = @name
    NogBlob.call.getBlobDownloadURL {sha1, filename}, (err, res) ->
      if err
        return reportError ERR_BLOB_DOWNLOAD,
          reason: 'Failed to get download URL.'
          cause: err
      console.log 'Download url:', res
      link = document.createElement 'a'
      link.href = res
      link.download = ''
      e = document.createEvent 'MouseEvents'
      e.initEvent 'click', true, true
      link.dispatchEvent e

share.NogBlobTest.Hasher = Hasher
