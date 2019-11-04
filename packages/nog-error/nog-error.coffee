NogError = {}

config =
  logExpire_s: 60 * 60 * 24 * 5

# See README for introduction to client-side and server-side error handling in
# Meteor.

createError = (specOrCode, contextOrReason, details) ->
  if typeof specOrCode is 'object'
    spec = specOrCode
    context = contextOrReason
  else
    console.log "[nog-error] Using legacy generic error '#{specOrCode}'."
    spec =
      errorCode: specOrCode
      statusCode: 500
      sanitized: 'full'
    if _.isString contextOrReason
      context = {reason: contextOrReason}
    else
      context = {cause: contextOrReason}
    if _.isObject details
      context.cause = details
    else
      context.details = details
  createErrorWithSpec spec, context

NogError.createError = createError

NogError.nogthrow = (args...) ->
  throw NogError.createError args...

NogError.defaultErrorHandler = (e) ->
  console.error e
  if Meteor.isClient
    errs = Session.get('errors') ? []
    errs.push e
    Session.set 'errors', errs


cloneOwn = (obj) -> _.object([k, v] for own k, v of obj)

errForHistory = (err) ->
  errorCode: err.errorCode ? err.code ? err.error
  statusCode: err.statusCode
  reason: err.reason ? err.message
  details: err.details ? null
  sanitized: _.clone(err.sanitized ? null)
  time: err.time ? null
  token: err.token ? null
  context: err.context ? _.omit cloneOwn(err), [
      'errorCode', 'code', 'error', 'statusCode', 'reason', 'message',
      'details', 'sanitized', 'time', 'token',
      'errorType', 'history', 'sanitizedError'
    ]

causeTail = (cause) ->
  if cause?.reason?
    reason = " Cause: #{cause.reason}"
  else if cause?.message?
    reason = " Cause: #{cause.message}"
  else
    reason = ''

  if cause?.details?
    details = " Cause: #{cause.details}"
  else
    details = ''

  if Match.test cause.history, [Object]
    history = cause.history
  else
    history = [errForHistory cause]

  if (serr = cause.sanitizedError)?
    sanitizedReason = " Cause: #{serr.reason}"
    sanitizedDetails = " Cause: #{serr.details}"
  else if cause.errorType is 'Meteor.Error'
    sanitizedReason = " Cause: #{cause.reason}"
    sanitizedDetails = " Cause: #{cause.details}"
  else
    sanitizedReason = ''
    sanitizedDetails = ''

  return {reason, details, history, sanitizedReason, sanitizedDetails}


sanitizedDefaults =
  errorCode: 'NOGERR'
  reason: 'Unspecified error.'
  details: ''

if Meteor.isServer
  errorLog = new Mongo.Collection 'nogerror.errorLog'
  errorLog._ensureIndex {
      time: 1
    }, {
      expireAfterSeconds: config.logExpire_s
    }
else
  errorLog = new Mongo.Collection null

NogError.errorLog = errorLog

logError = Meteor.bindEnvironment (errdoc) ->
  # Pass callback to run async; ignore errors.
  errorLog.insert errdoc, (err, res) ->

createErrorWithSpec = (spec, ctx) ->
  ctx ?= {}
  try
    check ctx, Match.ObjectIncluding(spec.contextPattern ? {})
  catch matcherr
    console.error "
        [nog-error] The context for #{spec.errorCode} did not match the
        expected pattern: #{matcherr.message}.
      "

  errdoc = { errorCode: spec.errorCode, statusCode: spec.statusCode }
  errdoc.context = _.omit ctx, 'cause', 'reason', 'details'

  if ctx.reason?
    errdoc.reason = String(ctx.reason)
  else if _.isFunction spec.reason
    errdoc.reason = spec.reason ctx
  else
    errdoc.reason = String(spec.reason)

  if ctx.details?
    errdoc.details = String(ctx.details)
  else if _.isFunction spec.details
    d = spec.details ctx
    if typeof d is 'object'
      errdoc.details = d.details
      _.extend errdoc.context, _.omit(d, 'details')
    else
      errdoc.details = d
  else
    errdoc.details = spec.details

  errdoc.sanitized = do ->
    if not spec.sanitized?
      sanitized = _.clone sanitizedDefaults
    else if spec.sanitized is 'full'
      sanitized = _.pick errdoc, 'errorCode', 'reason', 'details'
      sanitized.details ?= ''
    else if _.isObject spec.sanitized
      sanitized = _.clone spec.sanitized
      sanitized.errorCode ?= sanitizedDefaults.errorCode
      if _.isFunction sanitized.reason
        sanitized.reason = sanitized.reason ctx
      sanitized.reason ?= sanitizedDefaults.reason
      if _.isFunction sanitized.details
        sanitized.details = sanitized.details ctx
      sanitized.details ?= sanitizedDefaults.details
    else
      console.error "
          [nog-error] Invalid value `spec.sanitized: #{spec.sanitized}`; using
          defaults.
        "
      sanitized = _.clone sanitizedDefaults
    return sanitized

  errdoc.time = new Date()
  errdoc.token = Random.id(6).toLowerCase()
  errdoc.history = [errForHistory errdoc]

  fmtToken = (msg, tok, time) ->
    m = '['
    if Meteor.isServer
      m += 'server'
    else
      m += 'client'
    if time?
      m += ' '
      m += time.toISOString()
    m += ' '
    m += tok
    m += ']'
    if msg?.length
      m += ' '
      m += msg
    else
      m += '.'
    m

  errdoc.details = fmtToken(
      errdoc.details, errdoc.token, errdoc.time
    )
  errdoc.sanitized.details = fmtToken(
      errdoc.sanitized.details, errdoc.token, errdoc.time
    )

  if ctx.cause?
    t = causeTail ctx.cause
    errdoc.reason += t.reason
    errdoc.details += t.details
    if spec.sanitized is 'full'
      errdoc.sanitized.reason += t.reason
      errdoc.sanitized.details += t.details
    else
      errdoc.sanitized.reason += t.sanitizedReason
      errdoc.sanitized.details += t.sanitizedDetails
    errdoc.history = errdoc.history.concat t.history

  logError errdoc

  err = new NogError.Error errdoc
  err

NogError.Error = Meteor.makeErrorType 'NogError.Error', (opts) ->
  _.extend @, _.pick(opts,
      'errorCode', 'reason', 'details', 'statusCode', 'time', 'token',
      'history', 'context'
    )
  @message = @reason + ' [' + @errorCode + ']'
  s = opts.sanitized
  @sanitizedError = new Meteor.Error s.errorCode, s.reason, s.details
