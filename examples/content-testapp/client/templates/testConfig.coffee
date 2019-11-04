password = Meteor.settings.public?.tests?.passwords?.user
user = '__testing__user'
guest = '__testing__guest'

Template.testConfig.events

  'click .js-login-user': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: user}, password, ->
      console.log 'logged in user'

  'click .js-login-fred': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: 'fred'}, password, ->
      console.log 'logged in fred'

  'click .js-login-guest': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: guest}, password, ->
      console.log 'logged in guest'

  'click .js-logout': (ev) ->
    ev.preventDefault()
    Meteor.logout ->
      console.log 'logged out'
