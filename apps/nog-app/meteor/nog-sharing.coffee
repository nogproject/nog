# Sharing is organized by circles.  Each circle is created with a unique id.
# Bi-directional information about circles is stored in the users collection
# and the separate collection `shares`:
#
# Stored on the sharer's user doc (the circle owner):
#
#     Meteor.users.sharing.circles: [{
#         _id:   The circle id.
#         name:  The circle name.
#       }]
#
# Stored on the circle member's user doc (who may view a repo):
#
#     Meteor.users.sharing.inCircles: [{
#         circleId:  The circle id (owned by the sharer).
#         fromId:    The sharer's user id.
#       }]
#
# In collection `shares`:
#
#     NogSharing.shares: [{
#         _id:       Automatically generated id (irrelevant).
#         fromId:    Sharer's user id.
#         circleId:  The circle id (owned by sharer).
#         toId:      Id of circle member (the user who may view a repo).
#         toName:    Username of circle member (optimization for diplaying).
#                    Based on the assumption that usernames do not change.
#       }]
#
# Access control is stored on the collection `repos`.
#
#     NogContent.repos.sharing: {
#         public:      Boolean that indicates global public visibility.
#         allCircles:  Boolean to indicate visibility to all circles.
#         circles: [
#             set of circle ids that have access.
#           ]
#       }

optNogSharing = Meteor.settings.public?.optNogSharing ? true

# Interval between pending command scans.
cronTickInterval_s = 10

# Timeout after which pending commands are restarted by a cron `_tick()`.
defaultCmdTimeout_s = 10


if not optNogSharing
  console.log '[nog-sharing] disabled by optNogSharing.'
  return


{
  ERR_CREATE
  ERR_PARAM_INVALID
  ERR_PARAM_MALFORMED
  ERR_UNKNOWN
  ERR_UPDATE
  ERR_CONFLICT
  nogthrow
} = NogError

{
  checkAccess
  testAccess
} = NogAccess

@NogSharing =
  call: {}
  shares: new Mongo.Collection 'shares'


if Meteor.isServer
  Meteor.users._ensureIndex { 'cmd.ctime': 1 }, { sparse: true }


makeCmd = (op, opts) ->
  return _.extend({ _id: Random.id(), op, ctime: new Date() }, opts)


_maybeCrash = (loc) ->
  # Random errors can be useful for manual testing:
  #
  #if Math.random() < 0.5
  # throw new Error('Simulated crash')
  #
  return


matchCircleName = Match.Where (name) ->
  check name, String
  if not name.match /// ^[a-zA-Z0-9_-]+$ ///
    throw new Match.Error "
        malformed circle name (expected only characters, numbers, dashes and
        underscores; got '#{name}')
      "
  return true


defMethod = (name, func) ->
  qualname = 'NogSharing.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogSharing.call[name] = (args...) -> Meteor.call qualname, args...


defMethod 'createCircle', (opts) ->
  check opts,
    name: String
  if Meteor.isClient
    return
  user = Meteor.user()
  checkAccess user, 'nog-sharing/manageCircles'
  if not user?
    return

  {name} = opts
  try
    check name, matchCircleName
  catch err
    nogthrow ERR_PARAM_MALFORMED, {cause: err}

  circleId = Random.id()
  n = Meteor.users.update {
      _id: user._id
      'sharing.circles.name': {$ne: name}
    }, {
      $push: {'sharing.circles': {_id: circleId, name}}
    }
  if n isnt 1
    nogthrow ERR_CREATE, {reason: 'Failed to add circle to the user profile.'}
  return circleId


defMethod 'deleteCircle', (opts) ->
  check opts,
    name: String
  if Meteor.isClient
    return
  user = Meteor.user()
  checkAccess user, 'nog-sharing/manageCircles'
  if not user?
    return

  {name} = opts
  # Do not require `matchCircleName()` here to handle legacy names that do not
  # conform to the current format spec.

  if not (circle = _.findWhere(user.sharing?.circles ? [], {name}))?
    nogthrow ERR_UNKNOWN, {reason: "Unknown circle '#{name}'."}
  circleId = circle._id

  userId = user._id
  cmd = makeCmd 'delcir', { userId, circleId }
  selXcl = { _id: userId, cmd: { $exists: false } }
  if Meteor.users.update(selXcl, { $set: { cmd } }) != 1
    nogthrow ERR_CONFLICT

  return _deleteCircle2(cmd)

_deleteCircle2 = (cmd) ->
  { userId, circleId } = cmd
  _maybeCrash()
  # The circle is removed from the owner as the last operation, so that the
  # circle continues to be displayed even if the operation is interrupted.
  Meteor.users.update {
      'sharing.inCircles.circleId': circleId
    }, {
      $pull: {'sharing.inCircles': {circleId}}
    }, {
      multi: true
    }
  NogSharing.shares.remove {circleId}
  NogContent.repos.update {
      'sharing.circles': circleId
    }, {
      $pull: {'sharing.circles': circleId}
    }, {
      multi: true
    }
  Meteor.users.update {
      _id: userId
    }, {
      $pull: {'sharing.circles': {_id: circleId}}
      $unset: { cmd: '' }
    }
  return


defMethod 'extendCircle', (opts) ->
  check opts,
    toName: String
    circleName: String
  if Meteor.isClient
    return
  user = Meteor.user()
  checkAccess user, 'nog-sharing/manageCircles'
  if not user?
    return

  {toName, circleName} = opts

  findCircle = (sel) -> _.findWhere(user.sharing?.circles ? [], sel)
  if not (circle = findCircle({name: circleName}))?
    nogthrow ERR_UNKNOWN, {reason: "Circle #{circleName} is unknown."}

  toUser = Meteor.users.findOne {
      username: toName
    }, {
      fields: {_id: 1, username: 1}
    }
  if not toUser?
    nogthrow ERR_UNKNOWN, {reason: "User '#{toName}' is unknown."}

  if user._id == toUser._id
    nogthrow ERR_PARAM_INVALID, {reason: 'Cannot share to self.'}

  fromId = user._id
  toId = toUser._id
  toName = toUser.username
  circleId = circle._id
  cmd = makeCmd 'extcir', { fromId, toId, circleId, toName }
  selXcl = { _id: fromId, cmd: { $exists: false } }
  if Meteor.users.update(selXcl, { $set: { cmd } }) != 1
    nogthrow ERR_CONFLICT

  return _extendCircle2(cmd)

_extendCircle2 = (cmd) ->
  { fromId, toId, circleId, toName } = cmd
  _maybeCrash()
  # `shares` is the ground truth.
  sel = { fromId, toId, circleId }
  set = _.extend { toName }, sel
  NogSharing.shares.upsert sel, {$set: set}
  Meteor.users.update(
    { _id: toId },
    { $addToSet: { 'sharing.inCircles': { circleId, fromId } } },
  )
  Meteor.users.update({ _id: fromId }, { $unset: { cmd: '' } })
  return


defMethod 'shrinkCircle', (opts) ->
  check opts,
    toName: String
    circleName: Match.Optional String
  if Meteor.isClient
    return
  user = Meteor.user()
  checkAccess user, 'nog-sharing/manageCircles'
  if not user?
    return

  {toName, circleName} = opts

  toUser = Meteor.users.findOne {username: toName}, {fields: {_id: 1}}
  if not toUser?
    nogthrow ERR_UNKNOWN, {reason: "Unknown user '#{toName}'."}

  fromId = user._id
  toId = toUser._id
  circles = user.sharing?.circles ? []
  selShare = {fromId, toId}
  if circleName?
    findCircle = (sel) -> _.findWhere(circles, sel)
    if not (circle = findCircle({name: circleName}))?
      nogthrow ERR_UNKNOWN, {reason: "Unknown circle '#{circleName}'."}
    selShare['circleId'] = circle._id
    pullInCircles = [{circleId: circle._id, fromId}]
  else
    pullInCircles = ({circleId: c._id, fromId} for c in circles)

  cmd = makeCmd 'shrcir', { selShare, pullInCircles }
  selXcl = { _id: fromId, cmd: { $exists: false } }
  if Meteor.users.update(selXcl, { $set: { cmd } }) != 1
    nogthrow ERR_CONFLICT

  return _shrinkCircle2(cmd)

_shrinkCircle2 = (cmd) ->
  { selShare, pullInCircles } = cmd
  { fromId, toId } = selShare
  _maybeCrash()
  # `shares` is the ground truth.  Use the reverse order as in `extendCircle`.
  Meteor.users.update(
    { _id: toId },
    { $pullAll: { 'sharing.inCircles': pullInCircles } },
  )
  NogSharing.shares.remove(selShare)
  Meteor.users.update(
    { _id: fromId },
    { $unset: { cmd: '' } },
  )
  return


if Meteor.isServer
  Meteor.publish null, ->
    if @userId?
      Meteor.users.find {_id: @userId}, {fields: {sharing: 1}}
    else
      null

  Meteor.publish 'circles', ->
    if @userId? and testAccess(@userId, 'nog-sharing/manageCircles')
      NogSharing.shares.find {fromId: @userId}
    else
      null


defMethod 'updateRepoSharing', (opts) ->
  check opts,
    ownerName: String
    repoName: String
    public: Match.Optional Boolean
    allCircles: Match.Optional Boolean
    addCircleName: Match.Optional String
    addCircleId: Match.Optional String
    removeCircleName: Match.Optional String
    removeCircleId: Match.Optional String
  if Meteor.isClient
    return
  user = Meteor.user()
  checkAccess user, 'nog-sharing/updateRepoSharing', opts
  if not user?
    return

  findCircle = (sel) -> _.findWhere(user.sharing?.circles ? [], sel)

  sel =
    owner: opts.ownerName
    name: opts.repoName

  mod = {}

  if opts.public?
    mod.$set ?= {}
    mod.$set['sharing.public'] = opts.public
  if opts.allCircles?
    mod.$set ?= {}
    mod.$set['sharing.allCircles'] = opts.allCircles

  if (name = opts.addCircleName)?
    if not (c = findCircle({name}))?
      nogthrow ERR_UNKNOWN, {reason: "Unknown circle '#{name}'."}
    mod.$addToSet ?= {}
    mod.$addToSet['sharing.circles'] = c._id
  if (_id = opts.addCircleId)?
    if not (c = findCircle({_id}))?
      nogthrow ERR_UNKNOWN, {reason: "Unknown circle id '#{_id}'."}
    mod.$addToSet ?= {}
    mod.$addToSet['sharing.circles'] = c._id

  if (name = opts.removeCircleName)?
    if not (c = findCircle({name}))?
      nogthrow ERR_UNKNOWN, {reason: "Unknown circle '#{name}'."}
    mod.$pull ?= {}
    mod.$pull['sharing.circles'] = c._id
  if (_id = opts.removeCircleId)?
    if not (c = findCircle({_id}))?
      nogthrow ERR_UNKNOWN, {reason: "Unknown circle id '#{_id}'."}
    mod.$pull ?= {}
    mod.$pull['sharing.circles'] = c._id

  # Check that `mod` contains something, since `mod={}` would delete the repo.
  if not _.keys(mod).length
    nogthrow ERR_PARAM_INVALID, {reason: 'No changes specified.'}

  n = NogContent.repos.update sel, mod
  if n isnt 1
    nogthrow ERR_UPDATE, {reason: 'Failed to update repo.'}
  return


collectionContains = (coll, sel) -> coll.findOne sel, {_id: 1}


if Meteor.isServer
  NogAccess.removeStatements {action: 'nog-content/get'}
  NogAccess.addStatement
    principal: 'role:users'
    action: 'nog-content/get'
    effect: (opts) ->
      {ownerName, repoName, user, repo, style} = opts

      # The owner always has access.
      if ownerName == user.username
        return 'allow'

      # 'loose' indicates that the caller only wants to know whether the user
      # has access to some content at all.  The caller probably applies
      # stricter checks later, e.g. as part of a query.
      if style == 'loose'
        return 'allow'

      # Check based on repo sharing.
      if not repo? and ownerName? and repoName?
        repo = NogContent.repos.findOne {owner: ownerName, name: repoName}
      if repo?
        if not (sharing = repo.sharing)?
          return 'ignore'
        if sharing.public
          return 'allow'
        if sharing.allCircles
          owner = Meteor.users.findOne {username: repo.owner}, {_id: 1}
          if not owner?
            nogthrow ERR_UNKNOWN, {
                reason: "Unknown owner '#{repo.owner}'."
              }
          sel = {fromId: owner._id, toId: user._id}
          if collectionContains(NogSharing.shares, sel)
            return 'allow'
        if sharing.circles?
          sel = {toId: user._id, circleId: {$in: sharing.circles}}
          if collectionContains(NogSharing.shares, sel)
            return 'allow'
        return 'ignore'

      console.log 'Warning: nog-content/get missing opts.'
      return 'ignore'

  NogAccess.addStatement
    principal: 'role:users'
    action: 'nog-sharing/manageCircles'
    effect: 'allow'

  NogAccess.addStatement
    principal: 'role:users'
    action: 'nog-sharing/updateRepoSharing'
    effect: (opts) ->
      if opts.ownerName? and opts.ownerName == opts.user.username
        'allow'
      else
        'ignore'


_tick = ->
  timeout_s = defaultCmdTimeout_s
  cutoff = new Date()
  cutoff.setSeconds(cutoff.getSeconds() - timeout_s)
  handlers =
    delcir: _deleteCircle2
    extcir: _extendCircle2
    shrcir: _shrinkCircle2
  Meteor.users.find(
    {
      'cmd.ctime': { $lte: cutoff },
      'cmd.op': { $in: _.keys(handlers) },
    },
    { fields: { cmd: 1 } }
  ).forEach ({ cmd }) ->
    console.log '[sharing] Restart cmd', JSON.stringify(cmd)
    handlers[cmd.op](cmd)


startCron = ->
  Meteor.setInterval(_tick, cronTickInterval_s * 1000)


Meteor.startup startCron
