togglesCache = new ReactiveDict()

NogAccess.testAccess = (action, opts, callback) ->
  opts ?= {}
  if opts instanceof Spacebars.kw
    opts = opts.hash
  if _.isFunction opts
    callback = opts
    opts = {}

  userId = Meteor.userId()
  cacheKey = EJSON.stringify {userId, action, opts}, {canonical: true}
  t = togglesCache.get cacheKey

  # Use the cached value; but also call to check if it has changed.
  if t?
    Meteor.call 'nog-access/testAccess', action, opts, (err, res) ->
      if res?
        togglesCache.set cacheKey, res
    callback?(null, t)
    return t

  if _.isUndefined(t) or callback?
    togglesCache.set cacheKey, null
    Meteor.call 'nog-access/testAccess', action, opts, (err, res) ->
      if res?
        togglesCache.set cacheKey, res
      callback?(err, res)

  return null

NogAccess.testAccess_ready = (action, opts) ->
  NogAccess.testAccess(action, opts)?

Template.registerHelper 'testAccess', NogAccess.testAccess
Template.registerHelper 'testAccess_ready', NogAccess.testAccess_ready
