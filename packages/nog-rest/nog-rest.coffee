{
  ERR_LIMIT
  ERR_PROC_TIMEOUT
  createError
} = NogError

NogRest =
  checkRequestAuth: (req) ->
  authenticateFromHeader: null

# Use nog-auth if available (weak dependency) for signed URL auth.
#
# There is no default for header-based authentication.  It is disabled by
# default.
if (p = Package['nog-auth'])?
  console.log '[nog-rest] using nog-auth.'
  NogRest.checkRequestAuth = p.NogAuth.checkRequestAuth
else
  console.log '[nog-rest] no default auth, since nog-auth is not available.'


NogRest.config = config =

  # `timeout_s` is the duration after which an action will be canceled.
  #
  # 25s seems to be a reasonable default:  `nog.py` uses a read timeout
  # slightly below 30s.  A timeout of 30s is the default at Heroku, see
  # <https://devcenter.heroku.com/articles/request-timeout>.
  #
  timeout_s: Meteor.settings?.NogRest?.actionTimeout_s ? 25


# Use strict checking to ensure a sane config, and report reasonably
# operator-friendly messages.
checkConfig = (cfg) ->
  check cfg.timeout_s, Match.Where (x) ->
    unless Match.test x, Number
      throw new Match.Error 'timeout_s must be a number'
    unless x > 0
      throw new Match.Error 'timeout_s must be > 0'
    true


NogRest.configure = (cfg) ->
  cfg ?= {}
  old = {}
  for k in ['checkRequestAuth', 'authenticateFromHeader']
    if cfg[k]?
      old[k] = NogRest[k]
      NogRest[k] = cfg[k]
  for k of config
    if cfg[k]?
      old[k] = config[k]
      config[k] = cfg[k]

  for k, v of cfg
    if old[k] is undefined
      console.log "Warning: unused config in NogRest.configure(): #{k} = #{v}."

  checkConfig config
  return old


# Call `configure()` during startup to check the initial config and potentially
# initialize missing pieces.
Meteor.startup -> NogRest.configure()


optDebug = Meteor.settings.optDebugApiErrors ? false

if optDebug
  console.log '[nog-rest] API error debugging is enabled.  It should never be
    enabled in a production setup, since it may leak sensitive information.'


Picker.middleware Npm.require('body-parser').json({limit: '10mb'})

{parse: urlparse} = Npm.require('url')
pathToRegexp = Npm.require 'path-to-regexp'
{nogthrow} = NogError

NogRest.actions = (baseUrl, routes) ->
  for route in routes
    addRoute baseUrl, route

addRoute = (baseUrl, route) ->
  # Mimic Picker's URL matching by using the same `pathToRegexp()`, which is
  # also used by Express.  Then assign `params` and `baseUrl` to req, as
  # Express does it.
  _keys = []
  baseUrlRgx = pathToRegexp baseUrl, _keys, {end: false, strict: false}
  Picker.route baseUrl + route.path, (params, req, res, next) ->
    if req.method isnt route.method
      return next()

    # Use picker hack to handle body-parser size limit errors.
    if (cause = req.error)?
      if cause.type == 'entity.too.large'
        err = createError ERR_LIMIT, {
            reason: "The request is larger than the limit #{cause.limit}."
            cause
          }
      else
        err = cause
      return endError res, err

    # Setup for canceling after timeout: A timeout is registered before calling
    # the action (see below) and stored in `timeoutId`.  If it triggers,
    # `cancel` will set `isCanceled` and end the request with an error.  If the
    # action completes before the timeout triggers, the timeout will be cleared
    # and processing continues.  The timeout must be managed along the main
    # code path and in the catch clause.
    timeoutId = null
    isCanceled = false
    cancel = ->
      isCanceled = true
      endError res, createError ERR_PROC_TIMEOUT, {
          reason: "Request processing was canceled after #{config.timeout_s}
            seconds."
        }

    try
      if NogRest.authenticateFromHeader? and req.headers.authorization?
        NogRest.authenticateFromHeader(req)
      else
        NogRest.checkRequestAuth(req)
      # Parse query if `checkRequestAuth()` has not added it.
      parsed = urlparse(req.url, true)
      req.query ?= parsed.query
      req.params = params
      if not (m = parsed.pathname.match baseUrlRgx)?
        nogthrow 500, "Internal problem while parsing URL.",
          "Regex from baseUrl '#{baseUrl}' did not match url '#{req.url}'."
      req.baseUrl = m[0]

      timeoutId = Meteor.setTimeout cancel, config.timeout_s * 1000
      result = route.action req
      if isCanceled
        return
      Meteor.clearTimeout(timeoutId)

      if (sc = result.statusCode)? and (300 <= sc < 400)
        endRedirect res, result
      else
        endOk res, result
    catch err
      if isCanceled
        return
      if timeoutId?
        Meteor.clearTimeout(timeoutId)
      endError res, err

endOk = (res, result) ->
  result ?= null
  statusCode = result?.statusCode ? 200
  result = _.omit(result, 'statusCode')
  res.writeHead statusCode, {'Content-Type': 'application/json'}
  res.end JSON.stringify
    statusCode: statusCode
    data: result

endRedirect = (res, result) ->
  res.writeHead result.statusCode, {
    'Content-Type': 'application/json'
    'Location': result.location
  }
  res.end JSON.stringify result

endError = (res, err) ->
  doc = {}
  if err.errorType is 'Match.Error'
    doc.statusCode = 422
    doc.errorCode = 'ERR_MATCH'
    doc.message = err.message
    if optDebug
      doc.errorObject = err
  else if err.errorType is 'Meteor.Error'
    doc.errorCode = err.error
    doc.statusCode = err.statusCode ? 500
    doc.message = err.message
    doc.details = err.details
    if optDebug
      doc.errorObject = err
  else if err.errorType is 'NogError.Error'
    doc.errorCode = err.sanitizedError.error
    doc.statusCode = err.statusCode ? 500
    doc.message = err.sanitizedError.message
    doc.details = err.sanitizedError.details
    if optDebug
      doc.errorObject = err
  else
    doc.statusCode = 500
    doc.errorCode = 'ERR_UNEXPECTED_EXCEPTION'
    doc.message = 'Internal server error.'
    console.error(
        '[nog-rest] Unexpected JavaScript error: ' + err.message + '\n' +
        err.stack
      )
    if optDebug
      doc.message = 'Unexpected JavaScript error: ' + err.message
      doc.errorObject = { stack: err.stack }
  res.writeHead doc.statusCode, {'Content-Type': 'application/json'}
  res.end JSON.stringify doc
