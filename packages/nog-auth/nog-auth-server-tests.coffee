import { expect } from 'chai'

crypto = Npm.require 'crypto'
urlparse = Npm.require('url').parse

# Encode without ':' and strip milliseconds, since they are irrelevant.
toISOStringUrlsafe = (date) ->
  date.toISOString().replace(/:|\.[^Z]*/g, '')

testingKey = null
testingUid = '__testing_nog-auth-user'
testingScopes = [{action: 'fakeAction', opts: {'foo': 'bar'}}]

insertUser = ->
  Meteor.users.remove testingUid
  Meteor.users.insert { _id: testingUid, username: testingUid }

insertUserWithKey = ->
  insertUser()
  testingKey = NogAuth.createKeySudo {keyOwnerId: testingUid}

insertUserWithScopedKey = ->
  insertUser()
  testingKey = NogAuth.createKeySudo {
      keyOwnerId: testingUid
      scopes: testingScopes
    }

createSignedReq = (opts) ->
  opts ?= {}
  authalgorithm = 'nog-v1'
  authkeyid = testingKey.keyid
  now = new Date()
  authdate = toISOStringUrlsafe(opts.authdate ? now)
  authexpires = opts.authexpires ? 600
  authnonce = opts.authnonce ? crypto.randomBytes(10).toString('hex')
  authsignature = null
  method = 'GET'
  url = '/api' + '?'
  if opts.query?
    url += opts.query + '&'
  url += "authalgorithm=#{authalgorithm}"
  url += '&' + "authkeyid=#{authkeyid}"
  url += '&' + "authdate=#{authdate}"
  if authexpires
    url += '&' + "authexpires=#{authexpires}"
  if authnonce
    url += '&' + "authnonce=#{authnonce}"
  hmac = crypto.createHmac 'sha256', testingKey.secretkey
  hmac.update method + "\n" + url + "\n"
  authsignature = hmac.digest 'hex'
  url += '&' + "authsignature=#{authsignature}"
  {method, url}

describe 'nog-auth', -> describe 'Crypter', ->
  {Crypter} = NogAuthTest

  masterKeys = [
    { keyid: 'master2', secretkey: 'bbbb' }
    { keyid: 'master1', secretkey: 'aaaa' }
  ]
  key = { keyid: 'user', secretkey: 'xxxx' }

  it 'encrypts with first master key.', ->
    crypter = new Crypter masterKeys
    ckey = crypter.encrypt key
    check ckey, {keyid: String, crypt: String, masterkeyid: String}
    expect(ckey.keyid).to.equal key.keyid
    expect(ckey.masterkeyid).to.equal masterKeys[0].keyid

  it 'decrypts with any master key.', ->
    crypter1 = new Crypter masterKeys[1..]
    ckey1 = crypter1.encrypt key
    expect(ckey1.masterkeyid).to.equal masterKeys[1].keyid
    crypter = new Crypter masterKeys
    ckey = crypter.encrypt key
    expect(ckey1.masterkeyid).to.not.equal ckey.masterkeyid
    expect(crypter.decrypt(ckey1)).to.deep.equal key
    expect(crypter.decrypt(ckey)).to.deep.equal key

  it 'throws if master key is unknown.', ->
    crypter = new Crypter masterKeys
    ckey = crypter.encrypt key
    ckey.masterkeyid = 'invalid'
    fn = -> crypter.decrypt ckey
    expect(fn).to.throw 'ERR_UNKNOWN_MASTER_KEY'
    expect(fn).to.throw 'master key id'
    expect(fn).to.throw 'invalid'

describe 'nog-auth', -> describe 'KeyKeeper', ->
  {KeyKeeper} = NogAuthTest

  fakeUsers = null
  before ->
    fakeUsers = new Mongo.Collection null
    for uid in ['user1', 'user2']
      fakeUsers.insert { _id: uid, username: uid }

  masterKeys = [
    { keyid: 'master2', secretkey: 'bbbb' }
    { keyid: 'master1', secretkey: 'aaaa' }
  ]
  masterKeys1 = masterKeys[1..1]

  it "createKey() throws if user is unknown.", ->
    keeper = new KeyKeeper fakeUsers, masterKeys1
    fn = -> keeper.createKey {keyOwnerId: 'invalidUser'}
    expect(fn).to.throw 'ERR_UNKNOWN_USERID'
    expect(fn).to.throw 'user id'
    expect(fn).to.throw 'invalidUser'

  it "createKey() inserts crypted key into 'users' and returns plain key.", ->
    uid = 'user1'
    keeper = new KeyKeeper fakeUsers, masterKeys1
    key = keeper.createKey {keyOwnerId: uid}
    check key, {keyid: String, secretkey: String}
    user = fakeUsers.findOne {'services.nogauth.keys.keyid': key.keyid}
    expect(user._id).to.equal uid
    ckey = user.services.nogauth.keys[0]
    check ckey, {
        keyid: String,
        crypt: String,
        masterkeyid: String,
        createDate: Date
        comment: String,
        scopes: [{action: String, opts: Object}]
      }
    expect(ckey.keyid).to.equal key.keyid
    expect(ckey.masterkeyid).to.equal masterKeys1[0].keyid

  it "createKey() inserts key with comment.", ->
    uid = 'user1'
    keeper = new KeyKeeper fakeUsers, masterKeys1
    fakeComment = 'fake comment'
    key = keeper.createKey {
        keyOwnerId: uid
        comment: fakeComment
      }
    user = fakeUsers.findOne {'services.nogauth.keys.keyid': key.keyid}
    ukey = user.services.nogauth.keys.pop()
    expect(ukey.comment).to.equal fakeComment

  it "createKey() inserts key with scopes.", ->
    uid = 'user1'
    keeper = new KeyKeeper fakeUsers, masterKeys1
    fakeScopes = [
      {action: 'fakeAction', opts: {}}
      {action: 'fakeAction2', opts: {ownerName: 'userFoo'}}
    ]
    key = keeper.createKey {
        keyOwnerId: uid
        scopes: fakeScopes
      }
    user = fakeUsers.findOne {'services.nogauth.keys.keyid': key.keyid}
    ukey = user.services.nogauth.keys.pop()
    expect(ukey.scopes).to.deep.equal fakeScopes

  it "findKey() throws if keyid is unknown.", ->
    uid = 'user2'
    keeper = new KeyKeeper fakeUsers, masterKeys1
    fn = -> keeper.findKey 'unknownKeyId'
    expect(fn).to.throw 'ERR_UNKNOWN_KEYID'
    expect(fn).to.throw 'key id'
    expect(fn).to.throw 'unknownKeyId'

  it "
    findKey() returns a decrypted key, the corresponding user, and scopes.
  ", ->
    uid = 'user2'
    keeper = new KeyKeeper fakeUsers, masterKeys1
    # Create multiple keys and search for middle one to test that search works.
    keeper.createKey {keyOwnerId: uid}
    key = keeper.createKey {keyOwnerId: uid}
    keeper.createKey {keyOwnerId: uid}
    match = keeper.findKey key.keyid
    check match, {
      user: Object,
      key: {keyid: String, secretkey: String}
      scopes: [Object]
    }

  it "upgradeKeys() re-encrypts keys with the primary master key " +
  "and returns the number of re-encrypted keys.", ->
    selOld = { 'services.nogauth.keys.masterkeyid': masterKeys1[0].keyid }
    selNew = { 'services.nogauth.keys.masterkeyid': masterKeys[0].keyid }
    nOld = fakeUsers.find(selOld).count()
    expect(nOld).to.be.above 0
    expect(fakeUsers.find(selNew).count()).to.equal 0
    keeper = new KeyKeeper fakeUsers, masterKeys
    nUpdates = keeper.upgradeKeys()
    # At least one key must have changed for every user that selOld matched.
    expect(nUpdates).to.be.at.least nOld
    expect(fakeUsers.find(selOld).count()).to.equal 0
    expect(fakeUsers.find(selNew).count()).to.be.above 0

describe 'nog-auth', -> describe 'NogAuth.createKeySudo()', ->
  before insertUser

  it 'creates a key in Meteor.users.', ->
    key = NogAuth.createKeySudo {keyOwnerId: testingUid}
    check key, {keyid: String, secretkey: String}
    user = Meteor.users.findOne {'services.nogauth.keys.keyid': key.keyid}
    expect(user._id).to.equal testingUid

describe 'nog-auth', -> describe 'NogAuth.checkRequestAuth()', ->
  before insertUserWithKey

  {checkRequestAuth} = NogAuth

  describe 'with valid signature', ->
    it "returns true", ->
      req = createSignedReq()
      expect(checkRequestAuth(req)).to.equal true

    it "adds req.auth.user.", ->
      req = createSignedReq()
      checkRequestAuth(req)
      expect(req.auth.user._id).to.equal testingUid

    it "removes auth fields from query.", ->
      req = createSignedReq {query: 'foo=bar'}
      checkRequestAuth(req)
      expect(req.query).to.exist
      expect(req.query).to.deep.equal {foo: 'bar'}

  describe 'with valid signature, without expires', ->
    it "returns true", ->
      req = createSignedReq {authexpires: false}
      expect(checkRequestAuth(req)).to.equal true

  describe 'with valid signature, without nonce', ->
    it "returns true", ->
      req = createSignedReq {authnonce: false}
      expect(checkRequestAuth(req)).to.equal true

  describe 'with malformed request', ->
    it "throws match error (missing method)", ->
      wellformed = createSignedReq()
      fn = -> checkRequestAuth _.omit(wellformed, 'method')
      expect(fn).to.throw 'Match error'

    it "throws match error (missing url)", ->
      wellformed = createSignedReq()
      fn = -> checkRequestAuth _.omit(wellformed, 'url')
      expect(fn).to.throw 'Match error'

    for f in ['authkeyid', 'authsignature', 'authdate']
      do (f) ->
        it "throws 401 (missing #{f})", ->
          req = createSignedReq()
          req.url = req.url.replace(new RegExp('&' + f + '=[^&]*'), '')
          fn = -> checkRequestAuth req
          expect(fn).to.throw 'ERR_AUTH_FIELD_MISSING'
          expect(fn).to.throw "missing #{f}"

  describe 'with invalid date string', ->
    it "throws 'invalid authdate'", ->
      req = createSignedReq()
      req.url = req.url.replace /authdate=[^&]*/, 'authdate=20010101T010101'
      fn = -> checkRequestAuth req
      expect(fn).to.throw 'ERR_AUTH_DATE_INVALID'
      expect(fn).to.throw 'Invalid authdate'

  describe 'with unknown key', ->
    it "throws 'unknown key'", ->
      req = createSignedReq()
      req.url = req.url.replace /authkeyid=[^&]*/, 'authkeyid=unknownkey'
      fn = -> checkRequestAuth req
      expect(fn).to.throw 'ERR_AUTH_KEY_UNKNOWN'
      expect(fn).to.throw 'Unknown key'
      expect(fn).to.throw 'Cause'

  describe 'with invalid signature', ->
    it "throws 'invalid'", ->
      req = createSignedReq()
      req.url = req.url.replace /authsignature=[^&]*/,
        ('authsignature=' + Array(64).join '0')
      fn = -> checkRequestAuth req
      expect(fn).to.throw 'ERR_AUTH_SIG_INVALID'
      expect(fn).to.throw 'Invalid signature'

  describe 'with expired signature', ->
    it "throws 'expired'", ->
      req = createSignedReq { authdate: new Date('2001-01-01') }
      fn = -> checkRequestAuth req
      expect(fn).to.throw 'ERR_AUTH_SIG_EXPIRED'
      expect(fn).to.throw 'Expired signature'

  describe 'with duplicate nonce', ->
    it "throws 'invalid'", ->
      req = createSignedReq()
      expect(checkRequestAuth(req)).to.equal true
      fn = -> checkRequestAuth req
      expect(fn).to.throw 'ERR_AUTH_NONCE_INVALID'
      expect(fn).to.throw 'Invalid nonce'

  describe 'with nonce and long expires', ->
    it "throws 'invalid'", ->
      req = createSignedReq {authexpires: 2 * 24 * 3600}
      fn = -> checkRequestAuth req
      expect(fn).to.throw 'ERR_AUTH_EXPIRES_INVALID'
      expect(fn).to.throw 'Invalid expires'

describe 'nog-auth', -> describe 'NogAuth.checkRequestAuth()', ->
  before insertUserWithScopedKey

  {checkRequestAuth} = NogAuth

  describe 'with scoped key', ->
    it "adds req.auth.user.scopes.", ->
      req = createSignedReq()
      checkRequestAuth(req)
      expect(req.auth.user._id).to.equal testingUid
      expect(req.auth.user.scopes).to.deep.equal testingScopes

describe 'nog-auth', -> describe 'NogAuth.signRequest()', ->
  {signRequest, checkRequestAuth} = NogAuth

  before insertUserWithKey

  it "signs a request.", ->
    req = signRequest testingKey, { method: 'GET', url: '/path' }
    expect(checkRequestAuth(req)).to.equal true

    req = signRequest testingKey, { method: 'POST', url: '/path' }
    expect(checkRequestAuth(req)).to.equal true

  it "handles an existing query part.", ->
    req = signRequest testingKey, { method: 'GET', url: '/path?x=1' }
    expect(urlparse(req.url, true).query.x).to.equal '1'
    expect(checkRequestAuth(req)).to.equal true

# Add a test that marks completion to detect side-effect that cause missing
# result lists.
describe 'nog-auth', ->
  it 'server tests completed.', ->
