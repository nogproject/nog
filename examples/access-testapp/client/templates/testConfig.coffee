password = Meteor.settings.public?.tests?.passwords?.user
user = '__testing__user'
guest = '__testing__guest'
admin = '__testing__admin'

Template.testConfig.events

  'click .login-user-button': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: user}, password, ->
      console.log 'logged in user'

  'click .login-guest-button': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: guest}, password, ->
      console.log 'logged in guest'

  'click .login-admin-button': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: admin}, password, ->
      console.log 'logged in admin'

  'click .logout-button': (ev) ->
    ev.preventDefault()
    Meteor.logout ->
      console.log 'logged out'
