describe 'nog-access', -> describe 'testAccess (client)', ->
  server = meteor()
  client = browser(server)

  userSpec =
    username: '__testing__user'
    password: 'fakePassword'

  before ->
    client.execute(((userSpec) ->
      window.userSpec = userSpec
    ), [userSpec])
    server.execute(((userSpec) ->
      {username, password} = userSpec
      Meteor.users.remove {username}
      useruid = Accounts.createUser {username, password}
      Roles.addUsersToRoles useruid, ['users']
    ), [userSpec])

  it 'evaluates access for the current user.', ->
    client.promise((resolve) ->
      {username, password} = window.userSpec
      state = {}
      Meteor.logout ->
        NogAccess.testAccess 'isUser', (err, res) ->
          state.logout = {res}
          Meteor.loginWithPassword {username}, password, ->
            NogAccess.testAccess 'isUser', (err, res) ->
              state.login = {res}
              resolve(state)
    ).then((s) ->
      expect(s.logout.res).to.be.false
      expect(s.login.res).to.be.true
    )

  it 'passes opts to the server-side access check.', ->
    client.promise((resolve) ->
      {username, password} = window.userSpec
      state = {}
      Meteor.loginWithPassword {username}, password, ->
        opts = {ownerName: username, repoName: 'bar'}
        NogAccess.testAccess 'nog-content/modify', opts, (err, res) ->
          state.known = res
          opts = {ownerName: username + 'invalid', repoName: 'bar'}
          NogAccess.testAccess 'nog-content/modify', opts, (err, res) ->
            state.unknown = res
            resolve(state)
    ).then((s) ->
      expect(s.known).to.be.true
      expect(s.unknown).to.be.false
    )

  it 'caches the access check result.', ->
    client.promise((resolve) ->
      opts = {random: Random.id()}
      expect(NogAccess.testAccess('isUser', opts)).to.be.null
      opts = {random: Random.id()}
      expect(NogAccess.testAccess_ready('isUser', opts)).to.be.false
      NogAccess.testAccess 'isUser', opts, (err, res) ->
        resolve {
          ready: NogAccess.testAccess_ready('isUser', opts)
          access: NogAccess.testAccess('isUser', opts)
        }
    ).then((s) ->
      expect(s.ready).to.be.true
      expect(s.access).to.exist
    )

  describe '{{testAccess}}', ->
    it 'handles Spacebars.kw args.', ->
      client.promise((resolve) ->
        {username, password} = window.userSpec
        Meteor.loginWithPassword {username}, password, ->
          NogAccess.testAccess 'nog-content/modify', {
            ownerName: username, repoName: 'foo'
          }, (err, res) ->
            NogAccess.testAccess 'nog-content/modify', {
              ownerName: 'other', repoName: 'foo'
            }, (err, res) ->
              div = renderToDiv Template.testingTestAccess
              resolve {
                owner: $(div).find('.owner-may-modify').length
                other: $(div).find('.other-cannot-modify').length
              }
      ).then((s) ->
        expect(s.owner).to.equal 1
        expect(s.other).to.equal 1
      )
