# Public API.
NogAuth =
  call: {}
  onerror: NogError.defaultErrorHandler
  checkAccess: ->

# Exported for testing.
NogAuthTest = {}

# Use nog-access if available (weak dependency).
if Meteor.isServer
  if (p = Package['nog-access'])?
    console.log '[nog-auth] using nog-access default policy.'
    NogAuth.checkAccess = p.NogAccess.checkAccess
  else
    console.log '
        [nog-auth] default access control disabled, since nog-access is not
        available.
      '

# `configure()` can be called at any time to change the active config.
NogAuth.configure = (cfg) ->
  cfg ?= {}
  used = {}
  for k in ['onerror', 'checkAccess']
    if cfg[k]?
      NogAuth[k] = cfg[k]
      used[k] = true

  for k, v of cfg
    unless used[k]
      console.log "Warning: unused config in NogAuth.configure(): #{k} = #{v}."


defMethod = (name, func) ->
  qualname = 'NogAuth.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogAuth.call[name] = (args...) -> Meteor.call qualname, args...


defMethod 'createKey', (opts) ->
  if opts?
    check opts, {keyOwnerId: String}
  unless Meteor.isServer
    return
  opts ?= {keyOwnerId: Meteor.userId()}
  NogAuth.createKey Meteor.user(), opts


defMethod 'deleteKey', (opts) ->
  check opts,
    keyid: String
    keyOwnerId: Match.Optional String
  unless Meteor.isServer
    return
  NogAuth.deleteKey Meteor.user(), opts


share.NogAuth = NogAuth
share.NogAuthTest = NogAuthTest
