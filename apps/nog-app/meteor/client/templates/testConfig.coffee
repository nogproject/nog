password = Meteor.settings.public?.tests?.passwords?.user

Template.testConfig.events

  'click .js-login-sprohaska': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: 'sprohaska'}, password, ->
      console.log 'logged in sprohaska'

  'click .js-login-alovelace': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: 'alovelace'}, password, ->
      console.log 'logged in alovelace'

  'click .js-logout': (ev) ->
    ev.preventDefault()
    Meteor.logout ->
      console.log 'logged out'
