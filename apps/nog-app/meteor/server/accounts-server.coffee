import { gitlabGetEmail, handleWellknownAccounts } from './accounts-server2.js'
import { mailToAdminsNewUser } from './email.js'


optGlobalReadOnly = Meteor.settings.optGlobalReadOnly
oauthSecretKey = Meteor.settings.oauthSecretKey
GITHUB_CLIENT_ID = Meteor.settings.GITHUB_CLIENT_ID
GITHUB_CLIENT_SECRET = Meteor.settings.GITHUB_CLIENT_SECRET
GITIMP_CLIENT_ID = Meteor.settings.GITIMP_CLIENT_ID
GITIMP_CLIENT_SECRET = Meteor.settings.GITIMP_CLIENT_SECRET
GITZIB_CLIENT_ID = Meteor.settings.GITZIB_CLIENT_ID
GITZIB_CLIENT_SECRET = Meteor.settings.GITZIB_CLIENT_SECRET


# Configure `oauth-encryption` to encrypt OAuth access tokens before storing
# them in MongoDB.  Stop if no secret key has been configured in production.

if oauthSecretKey?
  Accounts.config { oauthSecretKey }
else if Meteor.isDevelopment
  console.log '[app] OAuth token encryption disabled in development.'
else
  console.error 'Error: Missing `oauthSecretKey` in production.'
  process.exit(1)


# Encrypt secret without userId to match the `OAuth.openSecret()` call in
# `meteor/packages/github/github_server.js`.

if optGlobalReadOnly
  console.log(
    '[app] [GRO] Only checking GitHub service setup in read-only mode.'
  )
  srvcfg = ServiceConfiguration.configurations.findOne({ service: 'github' })
  check srvcfg, Match.ObjectIncluding {
    clientId: String
    secret: Object
  }
  srvcfg = ServiceConfiguration.configurations.findOne({ service: 'gitimp' })
  check srvcfg, Match.ObjectIncluding {
    clientId: String
    secret: Object
  }
  srvcfg = ServiceConfiguration.configurations.findOne({ service: 'gitzib' })
  check srvcfg, Match.ObjectIncluding {
    clientId: String
    secret: Object
  }
else
  if oauthSecretKey?
    githubSecret = OAuthEncryption.seal(GITHUB_CLIENT_SECRET)
    gitimpSecret = OAuthEncryption.seal(GITIMP_CLIENT_SECRET)
    gitzibSecret = OAuthEncryption.seal(GITZIB_CLIENT_SECRET)
  else
    githubSecret = GITHUB_CLIENT_SECRET
    gitimpSecret = GITIMP_CLIENT_SECRET
    gitzibSecret = GITZIB_CLIENT_SECRET
  ServiceConfiguration.configurations.upsert {
    service: 'github'
  }, {
    $set: { clientId: GITHUB_CLIENT_ID, secret: githubSecret }
  }
  ServiceConfiguration.configurations.upsert {
    service: 'gitimp'
  }, {
    $set: {
      clientId: GITIMP_CLIENT_ID,
      secret: gitimpSecret,
      serverUrl: 'https://git.imp.fu-berlin.de',
      authorizationEndpoint: '/oauth/authorize',
      loginStyle: 'popup',
      tokenEndpoint: '/oauth/token',
      userinfoEndpoint: '/oauth/userinfo',
      requestPermissions: ['openid', 'read_user'],
      idTokenWhitelistFields: ['name', 'email', 'profile']
    }
  }
  Oidc.registerServer('gitimp', {
    usernameFromUserinfo: (userinfo) -> userinfo.name
  })
  Oidc.registerOidcService('gitimp')

  ServiceConfiguration.configurations.upsert {
    service: 'gitzib'
  }, {
    $set: {
      clientId: GITZIB_CLIENT_ID,
      secret: gitzibSecret,
      serverUrl: 'https://git.zib.de',
      authorizationEndpoint: '/oauth/authorize',
      loginStyle: 'popup',
      tokenEndpoint: '/oauth/token',
      userinfoEndpoint: '/oauth/userinfo',
      requestPermissions: ['openid', 'read_user'],
      idTokenWhitelistFields: ['name', 'email', 'profile', 'nickname']
    }
  }
  Oidc.registerServer('gitzib', {
    usernameFromUserinfo: (userinfo) -> userinfo.nickname
  })
  Oidc.registerOidcService('gitzib')

{
  ERR_CREATE_ACCOUNT_USERNAME
  ERR_CREATE_ACCOUNT_USERNAME_TOOSHORT
  ERR_CREATE_ACCOUNT_USERNAME_INVALID
  ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED
  ERR_CREATE_ACCOUNT_USERNAME_GITHUB
  ERR_CREATE_ACCOUNT_USERNAME_GITIMP
  ERR_CREATE_ACCOUNT_USERNAME_GITZIB
  ERR_CREATE_ACCOUNT_EMAIL
  nogthrow
} = NogError


# The username is sanitized to avoid obvious problems.  The Meteor account
# system uses a unique MongoDB index on `username`, so the new username must be
# unique.  Known routes like `api`, `settings`, `admin` and so on should be
# protected.  Users should not take suspicious names like `admin` or `root`.
#
# Usernames are used as path-like identifiers (in repo names).  They should be
# simple and not contain spaces.  They must not contain slashes.  They better
# be all-lowercase.  They must not contain characters that may be confused
# (like unicode lookalikes).
#
# Names are required to have at least three chars to protected all two-char
# prefixes for future use.

isBlacklisted = (name) ->
  blacklist = [
    /// ^about.* ///
    /// ^admin.* ///
    /// ^api.* ///
    /// ^blog.* ///
    /// ^contact.* ///
    /// ^help.* ///
    /// ^impressum.* ///
    /// ^nog.*bot.* ///
    /// ^root.* ///
    /// ^search.* ///
    /// ^security.* ///
    /// ^settings.* ///
    /// ^site.* ///
    /// ^tools.* ///
    /// ^user$ ///
    /// ^zib.* ///
  ]
  for b in blacklist
    if name.match b
      return true
  false


containsSpecialCharacters = (name) ->
  return !(/^[a-z0-9_-]*$/).test(name)


Accounts.onCreateUser (opts, user) ->
  handleWellknownAccounts(user)

  if opts.profile
    user.profile = opts.profile

  if not user.username?
    if (github = user.services?.github)?
      user.username = github.username
      if (existingUser = Meteor.users.findOne(username: github.username))
        unless (existingUser.services?.github?)
          nogthrow ERR_CREATE_ACCOUNT_USERNAME_GITHUB
    if (gitimp = user.services?.gitimp)?
      user.username = gitimp.username
      if (existingUser = Meteor.users.findOne(username: user.username))
        unless (existingUser.services?.gitimp?)
          nogthrow ERR_CREATE_ACCOUNT_USERNAME_GITIMP
    if (gitzib = user.services?.gitzib)?
      user.username = gitzib.username
      if (existingUser = Meteor.users.findOne(username: user.username))
        unless (existingUser.services?.gitzib?)
          nogthrow ERR_CREATE_ACCOUNT_USERNAME_GITZIB

  if not user.username?
    nogthrow ERR_CREATE_ACCOUNT_USERNAME

  if user.username.length < 3
    nogthrow ERR_CREATE_ACCOUNT_USERNAME_TOOSHORT

  if containsSpecialCharacters(user.username)
    nogthrow ERR_CREATE_ACCOUNT_USERNAME_INVALID

  if isBlacklisted user.username
    nogthrow ERR_CREATE_ACCOUNT_USERNAME_BLACKLISTED

  # The Meteor accounts system expects emails to be stored as an array of
  # objects with field `address` (see
  # `meteor/packages/accounts-base/accounts_server.js`.
  #
  # If there are no emails, use the publicly visible GitHub email if available.
  # If it is not available, request the email via the GitHub API (the OAuth
  # token has the necessary read permissions; see `requestPermissions` in
  # `accounts-client.coffee` and GitHub scopes
  # <https://developer.github.com/v3/oauth/#scopes>: `user:email` grants
  # read-only access)
  #
  # GitHub's `getEmails()` returns an array of `{email:String, primary:Boolean,
  # verified:Boolean}`.
  if not user.emails?
    if (github = user.services?.github)?
      if (email = github.email)?
        user.emails = [{address: email}]
      else
        gh = new GitHub {version: '3.0.0'}
        gh.authenticate
          type: 'oauth'
          token: OAuth.openSecret(github.accessToken, user._id)
        emails = gh.user.getEmails({})
        if not emails[0]?
          nogthrow ERR_CREATE_ACCOUNT_EMAIL
        user.emails = ({address: e.email} for e in emails)
    else if (gitimp = user.services?.gitimp)?
      email = gitlabGetEmail({
        url: 'https://git.imp.fu-berlin.de',
        token: OAuth.openSecret(gitimp.accessToken, user._id),
      })
      user.emails = [{address: email}]
    else if (gitzib = user.services?.gitzib)?
      email = gitlabGetEmail({
        url: 'https://git.zib.de',
        token: OAuth.openSecret(gitzib.accessToken, user._id),
      })
      user.emails = [{address: email}]

  if not user.emails?[0].address?
    nogthrow ERR_CREATE_ACCOUNT_EMAIL

  if Accounts.findUserByEmail(user.emails[0].address)?
    nogthrow ERR_CREATE_ACCOUNT_EMAIL, {
      reason: 'Email address already in use.  Please contact an administrator.'
    }

  # The Meteor accounts system detects and rejects accounts with a duplicate
  # email address.  There is nothing obvious that we can do here, so we accept
  # the potential problem.  The best we could probably do is register an
  # account without email and later ask for it on the settings page.

  # Explicitly store the account type and publish it, so that it can be used at
  # the client to determine whether to offer changing the password.
  #
  # `accountType` only indicates the login service that was used to create the
  # account.  More login services can be linked to the account later.
  if user.services.password?
    user.accountType = 'password'
  else if user.services.github?
    user.accountType = 'github'
  else if user.services.gitimp?
    user.accountType = 'gitimp'
  else if user.services.gitzib?
    user.accountType = 'gitzib'
  else
    user.accountType = 'unknown'

  mailToAdminsNewUser(user)
  return user


Meteor.publish null, ->
  if @userId
    # Note that `accountType` can be published as expected, but
    # `services.github.id` did not work for unknown reasons.
    Meteor.users.find {_id: @userId}, {fields: {'accountType': 1}}
  else
    null
