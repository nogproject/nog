import { checkScopesV2 } from './scopes-v2.js'

NogAccess.config = config =
  uploadSizeLimit: Meteor.settings.public.upload.uploadSizeLimit

checkConfig = (cfg) ->
  check cfg.uploadSizeLimit, Match.Where (x) ->
    check x, Number
    x >= 0

NogAccess.configure = (cfg) ->
  cfg ?= {}
  used = {}
  for k of config
    if cfg[k]?
      config[k] = cfg[k]
      used[k] = true

  for k, v of cfg
    unless used[k]
      console.log "
          Warning: unused config in NogAccess.configure(): #{k} = #{v}.
        "

  checkConfig config

  if config.uploadSizeLimit is 0
    console.log '
        [nog-access] The upload size limit is disabled.  It can be configured
        in `Meteor.settings.public.upload.uploadSizeLimit`.
      '
  else
    console.log "
        [nog-access] The upload size limit is #{config.uploadSizeLimit} Bytes.
      "

# Call `configure()` during startup at least once to check the initial config.
Meteor.startup -> NogAccess.configure()

{
  nogthrow
  ERR_ACCESS_DENY
  ERR_ACCESS_DEFAULT_DENY
} = NogError

# Exported for testing.
NogAccessTest = {}


matchPrincipal = (p, s) ->
  if _.isString(s.principal) and (p is s.principal)
    true
  else if _.isRegExp(s.principal) and (p.match s.principal)?
    true
  else
    false


checkWithPrincipals = (principals, statements, action, opts) ->
  denied = false
  allowed = false
  denyReasons = []
  denyStatements = []
  for principal in principals
    for s in statements
      if matchPrincipal(principal, s) and (s.action is action)
        effect = s.effect
        reason = null
        if (typeof effect) is 'function'
          effect = effect _.extend {principal}, opts
          if (typeof effect) is 'object'
            {effect, reason} = effect
        switch effect
          when 'allow' then allowed = true
          when 'deny'
            denied = true
            if reason?
              denyReasons.push reason
            denyStatements.push
              principal: principal
              action: action
              statement:
                principal: s.principal
                action: s.action
                effect: effect
                reason: reason
              opts: opts
          when 'ignore' then true
          else console.error "Invalid policy effect '#{effect}'."
  if denied
    if denyReasons.length > 0
      reason = 'Access denied: ' + denyReasons.join(' ')
    else
      reason = 'Access denied by policy.'
    nogthrow ERR_ACCESS_DENY, {reason, denyStatements}
  else if allowed
    return
  else
    nogthrow ERR_ACCESS_DEFAULT_DENY


checkScopes = (scopes, action, opts) ->
  for s in scopes
    if s.action == action
      for k, v of s.opts
        unless opts?[k] == v
          nogthrow ERR_ACCESS_DENY, {reason: 'Scoped key opts mismatch.'}
      return
  nogthrow ERR_ACCESS_DENY, {reason: 'Insufficient key scope.'}


checkAccess = (user, action, opts) ->
  if _.isString user
    user = Meteor.users.findOne {_id: user}
  if user?
    if (scopesV2 = user.scopesV2)?
      checkScopesV2(scopesV2, action, opts)
    if (scopes = user.scopes)?
      checkScopes scopes, action, opts
    opts = _.extend {}, opts, {user}  # {user} last to override.
    principals = ('role:' + r for r in Roles.getRolesForUser(user))
    if principals.length is 0
      principals.push 'guests'
    principals.push 'username:' + user.username
    principals.push 'userid:' + user._id
    for k, srv of user.services
      if (ldapgroups = srv.ldapgroups)?
        for g in srv.ldapgroups
          principals.push 'ldapgroup:' + g
  else
    principals = ['anonymous']
  checkWithPrincipals principals, share.statements, action, opts

testAccess = (user, action, opts) ->
  try
    checkAccess user, action, opts
  catch err
    return false
  return true

NogAccessTest.checkWithPrincipals = checkWithPrincipals

NogAccess.checkAccess = checkAccess
NogAccess.testAccess = testAccess

Meteor.methods
  'nog-access/testAccess': (action, opts) ->
    NogAccess.testAccess Meteor.userId(), action, opts


NogAccess.removeStatements = (sel) ->
  check sel, {action: String}
  len = share.statements.length
  share.statements = _.reject share.statements, (s) -> s.action is sel.action
  return len - share.statements.length


matchEffectString = Match.Where (x) ->
  check x, String
  x is 'allow' or x is 'deny'


NogAccess.addStatement = (statement) ->
  check statement,
      principal: Match.OneOf(String, RegExp)
      action: String
      effect: Match.OneOf(matchEffectString, Function)
  share.statements.push statement
