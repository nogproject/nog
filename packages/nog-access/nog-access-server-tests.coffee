import { expect } from 'chai'

# A testing password is required in the settings, which are never committed, to
# avoid accidentally leaking real passwords.  This could be replaced by an
# automatically generated random password.
password = Meteor.settings.public?.tests?.passwords?.user
expect(password).to.exist
username = '__testing__user'
guestname = '__testing__guest'
ldapname = '__testing_ldap'
ldapAllowGroup = 'ag_fake'
ldapOtherGroup = 'ag_hoax'
Meteor.users.remove {username: guestname}
guestuid = Accounts.createUser {username: guestname, password}
guestuser = Meteor.users.findOne guestuid

Meteor.users.remove {username}
useruid = Accounts.createUser {username, password}
Roles.addUsersToRoles useruid, ['users']
useruser = Meteor.users.findOne useruid

Meteor.users.remove {username: ldapname}
ldapuid = Accounts.createUser {username: ldapname, password}
Meteor.users.update(
  { _id: ldapuid },
  { $set: { 'services.testing.ldapgroups': [ldapAllowGroup] } },
)
ldapuser = Meteor.users.findOne ldapuid

# Add fake statements to tests whether the expected principals are passed to
# acheck.
NogAccess.addStatement
  principal: 'role:users'
  action: 'fakeAllowByRoleUser'
  effect: 'allow'
NogAccess.addStatement
  principal: 'username:' + username
  action: 'fakeAllowByUsername'
  effect: 'allow'
NogAccess.addStatement
  principal: 'userid:' + useruid
  action: 'fakeAllowByUserid'
  effect: 'allow'
NogAccess.addStatement
  principal: 'guests'
  action: 'fakeAllowByGuest'
  effect: 'allow'
NogAccess.addStatement
  principal: 'anonymous'
  action: 'fakeAllowByAnonymous'
  effect: 'allow'
NogAccess.addStatement
  principal: 'ldapgroup:' + ldapAllowGroup
  action: 'fakeAllowByLdapGroup'
  effect: 'allow'
# The following statement is for negative testing of ldap group membership.
# The testing users do not have the `ldapOtherGroup`.
NogAccess.addStatement
  principal: 'ldapgroup:' + ldapOtherGroup
  action: 'fakeAllowByOtherLdapGroup'
  effect: 'allow'

actionsLoggedOutDeny = [
    'fakeAllowByRoleUser', 'fakeAllowByUsername', 'fakeAllowByUserid',
    'fakeAllowByGuest',
    'fakeAllowByLdapGroup', 'fakeAllowByOtherLdapGroup',
  ]
actionsLoggedOutAllow = [
    'fakeAllowByAnonymous',
  ]

actionsUserDeny = [
    'fakeAllowByGuest', 'fakeAllowByAnonymous',
    'fakeAllowByLdapGroup', 'fakeAllowByOtherLdapGroup',
  ]
actionsUserAllow = [
    'fakeAllowByRoleUser', 'fakeAllowByUsername', 'fakeAllowByUserid',
  ]

actionsGuestDeny = [
    'fakeAllowByAnonymous', 'fakeAllowByRoleUser', 'fakeAllowByUsername',
    'fakeAllowByUserid',
    'fakeAllowByLdapGroup', 'fakeAllowByOtherLdapGroup',
  ]
actionsGuestAllow = [
    'fakeAllowByGuest',
  ]

actionsLdapUserDeny = [
    'fakeAllowByAnonymous',
    'fakeAllowByRoleUser', 'fakeAllowByUsername', 'fakeAllowByUserid',
    'fakeAllowByOtherLdapGroup',
  ]
actionsLdapUserAllow = [
    # The ldap testing user is a guest, because it does not have role `user`.
    'fakeAllowByGuest',
    'fakeAllowByLdapGroup',
  ]

expectDeny = (user, action) ->
  fn = -> NogAccess.checkAccess user, action
  expect(fn).to.throw 'denied'

expectAllow = (user, action) ->
  NogAccess.checkAccess user, action  # does not throw

describe 'nog-access', -> describe 'NogAccess.checkAccess()', ->

  describe 'when logged out', ->
    for action in actionsLoggedOutDeny
      do (action) ->
        it "denies action '#{action}'.", ->
          user = null
          expectDeny(user, action)

    for action in actionsLoggedOutAllow
      do (action) ->
        it "grants action '#{action}'.", ->
          user = null
          expectAllow(user, action)

  describe 'when logged in as guest', ->
    for action in actionsGuestDeny
      do (action) ->
        it "denies action '#{action}'.", ->
          expectDeny(guestuid, action)
          expectDeny(guestuser, action)

    for action in actionsGuestAllow
      do (action) ->
        it "grants action '#{action}'.", ->
          expectAllow(guestuid, action)
          expectAllow(guestuser, action)

  describe 'when logged in as user', ->
    for action in actionsUserDeny
      do (action) ->
        it "denies action '#{action}'.", ->
          expectDeny(useruid, action)
          expectDeny(useruser, action)

    for action in actionsUserAllow
      do (action) ->
        it "grants action '#{action}'.", ->
          expectAllow(useruid, action)
          expectAllow(useruser, action)

  describe 'when logged in as ldap user', ->
    for action in actionsLdapUserAllow
      do (action) ->
        it "allows action '#{action}'.", ->
          expectAllow(ldapuid, action)
          expectAllow(ldapuser, action)
    for action in actionsLdapUserDeny
      do (action) ->
        it "denies action '#{action}'.", ->
          expectDeny(ldapuid, action)
          expectDeny(ldapuser, action)

  it 'the user is available as opts.user in effect functions.', ->
    action = 'fakeEffectWithUser'
    user = {_id: 'fakeUser'}
    effectOk = false
    NogAccess.addStatement
      principal: 'userid:fakeUser'
      action: action
      effect: (opts) ->
        if opts.user._id == user._id
          effectOk = true
        'allow'
    opts = {user: {_id: 'other'}}
    NogAccess.checkAccess user, action, opts  # does not throw
    NogAccess.removeStatements {action}
    expect(effectOk).to.be.true


describe 'scope check', ->

  it 'denies with empty scopes', ->
    action = actionsUserAllow[0]
    user = _.clone useruser
    user.scopes = []
    fn = -> NogAccess.checkAccess user, action
    expect(fn).to.throw 'Insufficient key scope'

  it 'denies with action mismatch', ->
    action = actionsUserAllow[0]
    user = _.clone useruser
    user.scopes = [
      {action: action + 'postfix', opts: {}}
    ]
    fn = -> NogAccess.checkAccess user, action
    expect(fn).to.throw 'Insufficient key scope'

  it 'grants with matching scope', ->
    action = actionsUserAllow[0]
    user = _.clone useruser
    user.scopes = [
      {action: action + 'postfix', opts: {}}
      {action, opts: {}}
    ]
    NogAccess.checkAccess user, action  # Does not throw.

  it 'denies with opts mismatch', ->
    action = actionsUserAllow[0]
    user = _.clone useruser
    user.scopes = [
      {action, opts: {'foo': 'missing-opt'}}
    ]
    fn = -> NogAccess.checkAccess user, action
    expect(fn).to.throw 'key opts mismatch'


describe 'nog-access', -> describe 'NogAccess.removeStatements()', ->
  it "removes statements based on action matching.", ->
    action = 'fakeAllowByAnonymousToBeRemoved'
    user = null
    for i in [0...10]
      NogAccess.addStatement
        principal: 'anonymous'
        action: action
        effect: 'allow'
    expectAllow(user, action)
    NogAccess.removeStatements {action}
    expectDeny(user, action)


# Test only some combinations for `testAccess()`, since the implementation
# calls `checkAccess()`.
describe 'nog-access', ->
  describe 'with valid user', ->
    for action in actionsUserAllow
      do (action) ->
        it "allows action '#{action}'.", ->
          expect(NogAccess.testAccess(useruid, action)).to.be.true
          expect(useruser).to.exist
          expect(NogAccess.testAccess(useruser, action)).to.be.true


# Test the default policy statements.
describe 'nog-access', -> describe 'default policy', ->
  {checkWithPrincipals, statements} = NogAccessTest
  roleUsers = ['role:users']
  roleAdmins = ['role:admins']
  roleNogSyncBots = ['role:nogsyncbots']
  roleNogLocalSyncBots = ['role:noglocalsyncbots']
  principalGuest = ['guests']

  it "denies uploads that are larger than the limit and explains reason.", ->
    fn = ->
      checkWithPrincipals roleUsers, statements, 'nog-blob/upload', {size: 100}
    NogAccess.configure {uploadSizeLimit: 0}
    fn()  # does not throw.
    NogAccess.configure {uploadSizeLimit: 1000}
    fn()  # does not throw.
    NogAccess.configure {uploadSizeLimit: 99}
    expect(fn).to.throw 'denied'
    expect(fn).to.throw 'size limit'
    expect(fn).to.throw '99'
    try
      fn()
    catch err
      errmsg = JSON.stringify(err.context.denyStatements)
      expect(errmsg).to.contain 'size limit'

  it "grants access to users to upload blob", ->
    fn = -> checkWithPrincipals roleUsers, statements, 'nog-blob/upload', {}
    fn()  # does not throw.

  it "grants access to users to download blob", ->
    fn = -> checkWithPrincipals roleUsers, statements, 'nog-blob/download', {}
    fn()  # does not throw.

  it "allows users to get content", ->
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = -> checkWithPrincipals roleUsers, statements, 'nog-content/get', opts
    fn()  # does not throw.

  it "denies guests to get content", ->
    prins = ['username:foo', 'guests']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = -> checkWithPrincipals prins, statements, 'nog-content/get', opts
    expect(fn).to.throw 'denied'

  it "allows users to fork a repo", ->
    prins = roleUsers
    opts = {ownerName: 'foo', repoName: 'bar'}
    action = 'nog-content/fork-repo'
    fn = -> checkWithPrincipals prins, statements, action, opts
    fn()  # does not throw.

  it "denies guests to fork a repo", ->
    prins = ['username:foo', 'guests']
    opts = {ownerName: 'foo', repoName: 'bar'}
    action = 'nog-content/fork-repo'
    fn = -> checkWithPrincipals prins, statements, action, opts
    expect(fn).to.throw 'denied'

  it "grants owner access to create a repo.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/create-repo', opts
    fn()  # does not throw.

  it "denies non-owner to create a repo.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo2', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/create-repo', opts
    expect(fn).to.throw 'denied'

  it "denies guests to create a repo.", ->
    prins = ['username:foo', 'guests']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/create-repo', opts
    expect(fn).to.throw 'denied'

  it "grants owner access to delete a repo.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/delete-repo', opts
    fn()  # does not throw.

  it "denies non-owner to delete a repo.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo2', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/delete-repo', opts
    expect(fn).to.throw 'denied'

  it "denies guests to delete a repo.", ->
    prins = ['username:foo', 'guests']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/delete-repo', opts
    expect(fn).to.throw 'denied'

  it "grants owner to rename a repo.", ->
    prins = ['username:foo']
    opts =
      old: {ownerName: 'foo', repoName: 'bar'}
      new: {ownerName: 'foo', repoName: 'baz'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/rename-repo', opts
    fn()  # does not throw.

  it "denies non-owner to rename a repo.", ->
    prins = ['username:foo']
    specs = [
        {
          old: {ownerName: 'foo', repoName: 'bar'}
          new: {ownerName: 'foo2', repoName: 'baz'}
        }
        {
          old: {ownerName: 'foo2', repoName: 'bar'}
          new: {ownerName: 'foo', repoName: 'baz'}
        }
      ]
    for opts in specs
      fn = ->
        checkWithPrincipals prins, statements, 'nog-content/rename-repo', opts
      expect(fn).to.throw 'denied'

  it "denies guests to rename a repo.", ->
    prins = ['username:foo', 'guests']
    opts =
      old: {ownerName: 'foo', repoName: 'bar'}
      new: {ownerName: 'foo', repoName: 'baz'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/rename-repo', opts
    expect(fn).to.throw 'denied'

  it "allows owner to create repo content.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/modify', opts
    fn()  # does not throw.

  it "denies non-owner to create repo content.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo2', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/modify', opts
    expect(fn).to.throw 'denied'

  it "denies guests to create repo content.", ->
    prins = ['username:foo', 'guests']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-content/modify', opts
    expect(fn).to.throw 'denied'


  it "grants owner access to configure a catalog.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-catalog/configure', opts
    fn()  # does not throw.

  it "denies non-owner to configure a catalog.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo2', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-catalog/configure', opts
    expect(fn).to.throw 'denied'

  it "grants owner access to update a catalog.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-catalog/update', opts
    fn()  # does not throw.

  it "denies non-owner to update a catalog.", ->
    prins = ['username:foo']
    opts = {ownerName: 'foo2', repoName: 'bar'}
    fn = ->
      checkWithPrincipals prins, statements, 'nog-catalog/update', opts
    expect(fn).to.throw 'denied'


  it "grants user to manage API keys.", ->
    prins = ['username:foo']
    opts = {
      user: {_id: 'fooId'}
      keyOwnerId: 'fooId'
    }
    fn = -> checkWithPrincipals prins, statements, 'nog-auth/apiKey', opts
    fn()  # does not throw.

  it "denies guests to manage API keys.", ->
    prins = ['username:foo', 'guests']
    opts = {
      user: {_id: 'fooId'}
      keyOwnerId: 'fooId'
    }
    fn = -> checkWithPrincipals prins, statements, 'nog-auth/apiKey', opts
    expect(fn).to.throw 'denied'

  it 'has placeholder action `isUser`.', ->
    fn = -> checkWithPrincipals roleUsers, statements, 'isUser', {}
    fn()  # does not throw.
    fn = -> checkWithPrincipals roleUsers, statements, 'isAdmin', {}
    expect(fn).to.throw 'denied'

  it "has placeholder action `isAdmin`.", ->
    fn = -> checkWithPrincipals roleAdmins, statements, 'isUser', {}
    expect(fn).to.throw 'denied'
    fn = -> checkWithPrincipals roleAdmins, statements, 'isAdmin', {}
    fn()  # does not throw.

  it "has placeholder action `isGuest`.", ->
    fn = -> checkWithPrincipals principalGuest, statements, 'isUser', {}
    expect(fn).to.throw 'denied'
    fn = -> checkWithPrincipals principalGuest, statements, 'isAdmin', {}
    expect(fn).to.throw 'denied'
    fn = -> checkWithPrincipals principalGuest, statements, 'isGuest', {}
    fn()  # does not throw.

  it 'grants nogsyncbots to get synchros', ->
    fn = -> checkWithPrincipals roleNogSyncBots, statements, 'nog-sync/get', {}
    fn()  # does not throw.

  it 'grants noglocalsyncbots to create and modify synchros', ->
    fn = ->
      checkWithPrincipals(
        roleNogLocalSyncBots, statements, 'nog-sync/create', {}
      )
    fn()  # does not throw.
    fn = ->
      checkWithPrincipals(
        roleNogLocalSyncBots, statements, 'nog-sync/modify', {}
      )
    fn()  # does not throw.

  it 'denies ordinary users access to synchros', ->
    prins = ['username:foo', 'guests']
    fn = -> checkWithPrincipals prins, statements, 'nog-sync/get', {}
    expect(fn).to.throw 'denied'
    fn = -> checkWithPrincipals prins, statements, 'nog-sync/modify', {}
    expect(fn).to.throw 'denied'
    fn = -> checkWithPrincipals prins, statements, 'nog-sync/create', {}
    expect(fn).to.throw 'denied'


# Test the low-level statement processing.
describe 'nog-access', -> describe 'checkWithPrincipals', ->

  specs = [
    {
      statements: [
        { principal: 'user', action: 'do', effect: 'allow' }
      ]
      expectations: [
        {
          name: 'deny access by default.'
          principals: ['foo']
          action: 'do'
          opts: null
          effect: 'deny'
        }
        {
          name: 'allow access by a matching statement.'
          principals: ['user']
          action: 'do'
          opts: null
          effect: 'allow'
        }
      ]
    }
    {
      statements: [
        { principal: 'user', action: 'do', effect: 'allow' }
        { principal: 'foo', action: 'do', effect: 'deny' }
      ]
      expectations: [
        {
          name: 'deny statement overrides allow statement.'
          principals: ['user', 'foo']
          action: 'do'
          opts: null
          effect: 'deny'
        }
      ]
    }
    {
      statements: [
        { principal: 'user', action: 'do', effect: (opts) -> opts.res }
      ]
      expectations: [
        {
          name: 'effect can be specified as a function (allow).'
          principals: ['user']
          action: 'do'
          opts: { res: 'allow' }
          effect: 'allow'
        }
        {
          name: 'effect can be specified as a function (deny).'
          principals: ['user']
          action: 'do'
          opts: { res: 'deny' }
          effect: 'deny'
        }
      ]
    }
    {
      statements: [
        {
          principal: 'user'
          action: 'do'
          effect: () -> {effect: 'deny', reason: 'testingReason'}
        }
      ]
      expectations: [
        {
          name: 'effect function can return object to describe reason.'
          principals: ['user']
          action: 'do'
          opts: { res: 'deny' }
          effect: 'deny'
          messages: ['testingReason']
        }
      ]
    }
    {
      statements: [
        {
          principal: /// ^user ///
          action: 'do'
          effect: (opts) ->
            if opts.principal is 'user'
              {effect: 'deny', reason: 'exact match'}
            else
              {effect: 'deny', reason: 'prefix match'}
        }
      ]
      expectations: [
        {
          name: 'principal can be a RegExp.'
          principals: ['user']
          action: 'do'
          opts: {}
          effect: 'deny'
          messages: ['exact match']
        }
        {
          name: 'principal can be a RegExp.'
          principals: ['user2']
          action: 'do'
          opts: {}
          effect: 'deny'
          messages: ['prefix match']
        }
      ]
    }
  ]

  for s in specs
    for e in s.expectations
      do (s, e) -> it e.name, ->
        {checkWithPrincipals} = NogAccessTest
        if e.effect is 'deny'
          fn = ->
            checkWithPrincipals e.principals, s.statements, e.action, e.opts
          expect(fn).to.throw 'denied'
          if e.messages
            for msg in e.messages
              expect(fn).to.throw msg
        else
          # must not throw
          checkWithPrincipals e.principals, s.statements, e.action, e.opts
