password = Meteor.settings.public?.tests?.passwords?.user
user = '__testing__user'
guest = '__testing__guest'

Template.testConfig.events

  'click .login-user-button': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: user}, password, ->
      console.log 'logged in user'

  'click .login-guest-button': (ev) ->
    ev.preventDefault()
    Meteor.loginWithPassword {username: guest}, password, ->
      console.log 'logged in guest'

  'click .logout-button': (ev) ->
    ev.preventDefault()
    Meteor.logout ->
      console.log 'logged out'

  'blur .uploadSizeLimit-text': (ev) ->
    ev.preventDefault()
    limit = Number(ev.target.value)
    console.log 'Setting uploadSizeLimit:', limit
    Meteor.call 'setUploadSizeLimit', limit, (err, res) ->
      if err?
        console.error 'Error setting uploadSizeLimit:', err

  'submit form': (ev) ->
    ev.preventDefault()
