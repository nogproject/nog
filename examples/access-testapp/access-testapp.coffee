if Meteor.isClient
  Session.setDefault('counter', 0)

if Meteor.isServer
  Meteor.startup ->
    password = Meteor.settings.public?.tests?.passwords?.user
    check password, String

    username = '__testing__user'
    Meteor.users.remove {username}
    uid = Accounts.createUser {username, password}
    Roles.addUsersToRoles uid, ['users']

    username = '__testing__admin'
    Meteor.users.remove {username}
    uid = Accounts.createUser {username, password}
    Roles.addUsersToRoles uid, ['users', 'admins']

    username = '__testing__guest'
    Meteor.users.remove {username}
    uid = Accounts.createUser {username, password}

FlowRouter.route '/',
  action: ->
    console.log 'home'
    BlazeLayout.render 'layout', {main: 'home'}

FlowRouter.route '/upload',
  action: ->
    console.log 'upload'
    BlazeLayout.render 'layout', {main: 'upload'}

FlowRouter.route '/download',
  action: ->
    console.log 'download'
    BlazeLayout.render 'layout', {main: 'download'}

FlowRouter.route '/admin',
  action: ->
    console.log 'admin'
    BlazeLayout.render 'layout', {main: 'admin'}


@Something = new Mongo.Collection 'something'

if Meteor.isServer
  Meteor.publish 'something', ->
    console.log 'publish: sleeping'
    Meteor._sleepForMs 1000
    console.log 'publish: continuing'
    Something.find()

if Meteor.isClient
  Template.registerHelper 'subsReady', (name) ->
    FlowRouter.subsReady name

requireUserOrGuest = (path, next) ->
  NogAccess.testAccess 'isUser', (err, isUser) ->
    NogAccess.testAccess 'isGuest', (err, isGuest) ->
      if isUser or isGuest
        next()
      else
        next('/sign-in')

FlowRouter.route '/profile',
  middlewares: [
      requireUserOrGuest
    ]

  subscriptions: ->
    # Do not use `NogAccess.testAccess()` here, since this code is not
    # reactive.  The subscription would not be updated when the permissions
    # change after the user logs in.
    @register 'something', Meteor.subscribe 'something'

  action: ->
    BlazeLayout.render 'layout', {main: 'profile'}


FlowRouter.route '/sign-in',
  action: ->
    console.log 'sign-in'
    BlazeLayout.render 'layout', {main: 'signIn'}
