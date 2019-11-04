{ NogAuth, NogAuthTest } = share

# Private.
config =
  maxExpiresWithNonce: 3600
  nonceHashLen: 20
  defaultExpires: 300
  masterKeys: Meteor.settings?.NogAuthMasterKeys ? null

# Print human-friendly messages first to help an operator resolve setting
# problems.  Then apply strict checks to confirm that the config is sane.
checkConfig = (cfg) ->
  if not cfg.masterKeys?
    console.error '[nog-auth] Missing master keys. ' +
      'Provide them in `Meteor.settings.NogAuthMasterKeys` as ' +
      '`[ {keyid: String, secretkey: String} ]`.'

  check cfg.maxExpiresWithNonce, Match.Where (x) ->
    check x, Number
    x > 0

  check cfg.defaultExpires, Match.Where (x) ->
    check x, Number
    x > 0

  check cfg.masterKeys, Match.Where (x) ->
    check x, [ { keyid: String, secretkey: String } ]
    x.length > 0

checkConfig config


crypto = Npm.require 'crypto'
urlparse = Npm.require('url').parse
{
  nogthrow
  ERR_UNKNOWN_MASTER_KEY
  ERR_UNKNOWN_USERID
  ERR_UNKNOWN_KEYID
  ERR_AUTH_FIELD_MISSING
  ERR_AUTH_DATE_INVALID
  ERR_AUTH_SIG_EXPIRED
  ERR_AUTH_KEY_UNKNOWN
  ERR_AUTH_SIG_INVALID
  ERR_AUTH_EXPIRES_INVALID
  ERR_AUTH_NONCE_INVALID
} = NogError


# nonces must be unique per authdate, which is ensured by using a hash of `date
# + nonce` as the `_id`; see below.  A TTL index is used for automatic cleanup;
# see:
# <http://stackoverflow.com/questions/18290196/time-to-live-with-meteor-collection>,
# <http://docs.mongodb.org/manual/core/index-ttl/>.

nonces = new Meteor.Collection('nogauth.nonces')
nonces._ensureIndex(
  {date: 1},
  {expireAfterSeconds: config.maxExpiresWithNonce}
)

# Ensure that the index of the old implementation is removed.  `_dropIndex()`
# can be removed after we've updated all deployments.  It should be kept at
# least until June 2016.

try
  nonces._dropIndex(
    {date: 1, nonce: 1},
    {unique: true}
  )
  console.log '[nog-auth] dropped nonce `{date, nonce}` index.'
catch
  true


class Crypter
  algo: 'aes-256-cbc'
  cryptenc: 'base64'
  plainenc: 'ascii'

  constructor: (masterKeys) ->
    @primaryKey = masterKeys[0]
    @masterKeys = {}
    for k in masterKeys
      @masterKeys[k.keyid] = k.secretkey

  encrypt: (key) ->
    cipher = crypto.createCipher @algo, @primaryKey.secretkey
    c = cipher.update key.secretkey, @plainenc, @cryptenc
    c += cipher.final @cryptenc
    return {
      keyid: key.keyid
      crypt: c
      masterkeyid: @primaryKey.keyid
    }

  decrypt: (ckey) ->
    if not (msecretkey = @masterKeys[ckey.masterkeyid])?
      nogthrow ERR_UNKNOWN_MASTER_KEY, {masterkeyid: ckey.masterkeyid}
    decipher = crypto.createDecipher @algo, msecretkey
    d = decipher.update ckey.crypt, @cryptenc, @plainenc
    d += decipher.final @plainenc
    return {
      keyid: ckey.keyid
      secretkey: d
    }

class KeyKeeper
  keyIdSize_bits: 80
  secretKeySize_bits: 160

  constructor: (@users, masterKeys) ->
    @crypter = new Crypter masterKeys
    @primaryKeyId = masterKeys[0].keyid

  createKey: (opts) ->
    check opts,
      keyOwnerId: String
      comment: Match.Optional String
      scopes: Match.Optional [{action: String, opts: Object}]
    {keyOwnerId, comment, scopes} = opts
    comment ?= ''
    scopes ?= []
    key =
      keyid: crypto.randomBytes(@keyIdSize_bits / 8).toString('hex')
      secretkey: crypto.randomBytes(@secretKeySize_bits / 8).toString('hex')
    ckey = @crypter.encrypt key
    ckey.createDate = new Date()
    ckey.comment = comment
    ckey.scopes = scopes
    n = @users.update keyOwnerId, {$push: {'services.nogauth.keys': ckey}}
    if n is 0
      nogthrow ERR_UNKNOWN_USERID, {uid: keyOwnerId}
    return key

  findKey: (keyid) ->
    check keyid, String
    user = @users.findOne {'services.nogauth.keys.keyid': keyid}
    if not user?
      nogthrow ERR_UNKNOWN_KEYID, {keyid}
    ckey = (_.find user.services.nogauth.keys, (k) -> k.keyid is keyid)
    return {
      user: user
      key: @crypter.decrypt ckey
      scopes: ckey.scopes ? []
    }

  upgradeKeys: ->
    selOld = { 'services.nogauth.keys.masterkeyid': {$ne: @primaryKeyId} }
    # Use the mongo positional operator to atomically update the keys, see
    # <http://docs.mongodb.org/manual/reference/operator/update/positional/>,
    # until nothing needs to be updated.
    nUpdates = 0
    needsCheck = true
    while needsCheck
      needsCheck = false
      @users.find(selOld).forEach (user) =>
        for ckey in user.services?.nogauth?.keys ? []
          unless ckey.masterkeyid is @primaryKeyId
            needsCheck = true
            key = @crypter.decrypt(ckey)
            ckey = @crypter.encrypt(key)
            nUpdates += @users.update {
              _id: user._id
              'services.nogauth.keys.keyid': ckey.keyid
            }, {
              $set: {
                'services.nogauth.keys.$': ckey
              }
            }
    return nUpdates

keyKeeper = new KeyKeeper Meteor.users, config.masterKeys

Meteor.startup ->
  n = keyKeeper.upgradeKeys()
  if n
    s = if n is 1 then '' else 's'
    console.log "[nog-auth] upgraded #{n} API key#{s} to new master key."


# Shorthand for pluggable checkAccess.
checkAccess = (args...) -> NogAuth.checkAccess args...


NogAuth.createKey = (user, opts) ->
  # keyOwnerId may be `null` before the access check, since NogAuth.call, uses
  # the current userId as the default.  But after the access check, it must not
  # be `null` anymore, since the `keyKeeper` requires an owner.
  check opts, {keyOwnerId: Match.OneOf(String, null)}
  checkAccess user, 'nog-auth/apiKey', opts
  check opts, {keyOwnerId: String}
  NogAuth.createKeySudo(opts)


NogAuth.createKeySudo = (opts) ->
  keyKeeper.createKey opts


NogAuth.deleteKey = (user, opts) ->
  check opts,
    keyid: String
    keyOwnerId: Match.Optional String
  checkAccess user, 'nog-auth/apiKey', opts
  NogAuth.deleteKeySudo opts


NogAuth.deleteKeySudo = (opts) ->
  check opts,
    keyid: String
    keyOwnerId: Match.Optional String
  {keyid, keyOwnerId} = opts
  sel = {'services.nogauth.keys.keyid': keyid}
  if keyOwnerId?
    sel._id = keyOwnerId
  Meteor.users.update sel, {
      $pull: {'services.nogauth.keys': {keyid: keyid}}
    }


# Automatically publish the keyids for the logged-in user.
Meteor.publish null, ->
  if @userId
    Meteor.users.find {_id: @userId}, {
        fields:
          'services.nogauth.keys.keyid': 1
          'services.nogauth.keys.createDate': 1
          'services.nogauth.keys.comment': 1
      }
  else
    null

NogAuth.checkRequestAuth = (req) ->
  check req.method, String
  check req.url, String

  {query} = urlparse(req.url, true)
  errctx = _.pick req, 'url', 'method'
  errctx.query = query

  for v in ['authkeyid', 'authsignature', 'authdate']
    if not query[v]?
      nogthrow ERR_AUTH_FIELD_MISSING, _.extend errctx, {missing: v}

  rgxdate = /^([0-9]{4}-[0-9]{2}-[0-9]{2}T)([0-9]{2})([0-9]{2})([0-9]{2})Z$/
  if not (m = query.authdate.match rgxdate)?
    nogthrow ERR_AUTH_DATE_INVALID, errctx
  authdate = new Date(m[1] + m[2] + ':' + m[3] + ':' + m[4] + 'Z')
  expires = Number(query.authexpires ? config.defaultExpires)
  deadline = new Date(authdate)
  deadline.setSeconds(authdate.getSeconds() + expires)
  now = new Date()
  if now > deadline
    nogthrow ERR_AUTH_SIG_EXPIRED, errctx

  try
    {user, key, scopes} = keyKeeper.findKey query.authkeyid
  catch err
    nogthrow ERR_AUTH_KEY_UNKNOWN, _.extend errctx, {cause: err}

  url = req.url.replace /&authsignature=[0-9a-f]{64}$/, ''
  hmac = crypto.createHmac 'sha256', key.secretkey
  hmac.update req.method + "\n" + url + "\n"
  signature = hmac.digest 'hex'

  if query.authsignature isnt signature
    nogthrow ERR_AUTH_SIG_INVALID, errctx

  if query.authnonce?
    if expires > config.maxExpiresWithNonce
      nogthrow ERR_AUTH_EXPIRES_INVALID, errctx
    try
      idstr = authdate.toISOString() + '_' + query.authnonce
      id = crypto.createHash('sha1').update(idstr).digest('hex')[0..config.nonceHashLen]
      nonces.insert {_id: id, date: authdate}
    catch err
      nogthrow ERR_AUTH_NONCE_INVALID, errctx

  req.query = _.omit query, 'authkeyid', 'authsignature', 'authdate',
    'authalgorithm', 'authnonce', 'authexpires'
  if scopes.length > 0
    user.scopes = scopes
  req.auth = {user}
  true

# Encode without ':' and strip milliseconds, since they are irrelevant.
toISOStringUrlsafe = (date) ->
  date.toISOString().replace(/:|\.[^Z]*/g, '')

NogAuth.signRequest = (key, req) ->
  authalgorithm = 'nog-v1'
  authkeyid = key.keyid
  now = new Date()
  authdate = toISOStringUrlsafe(now)
  authexpires = config.defaultExpires
  authnonce = crypto.randomBytes(10).toString('hex')
  if urlparse(req.url).query?
    req.url += '&'
  else
    req.url += '?'
  req.url += "authalgorithm=#{authalgorithm}"
  req.url += '&' + "authkeyid=#{authkeyid}"
  req.url += '&' + "authdate=#{authdate}"
  req.url += '&' + "authexpires=#{authexpires}"
  req.url += '&' + "authnonce=#{authnonce}"
  hmac = crypto.createHmac 'sha256', key.secretkey
  hmac.update req.method + "\n" + req.url + "\n"
  authsignature = hmac.digest 'hex'
  req.url += '&' + "authsignature=#{authsignature}"
  req

NogAuthTest.Crypter = Crypter
NogAuthTest.KeyKeeper = KeyKeeper
