{defaultErrorHandler} = NogError
{testAccess} = NogAccess


Template.accountsAdminContent.onCreated ->
  @subscribe 'accountsList'

Template.accountsAdminContent.onRendered ->
  if location.hash != ''
    $('a[href="'+location.hash+'"]').tab('show')

Template.accountsAdminContent.events
  'click .js-toggle-tabs': (ev) ->
    ev.preventDefault()
    location.hash = ev.currentTarget.hash

Template.userList.onCreated ->
  @roles = [
    {label: 'Not assigned', selector: {$or: [
      {roles: {$exists: false}},
      {roles: {$size: 0}}
    ]}},
    {label: 'Admins', selector: {roles: 'admins'}},
    {label: 'Users', selector: {roles: 'users'}},
    {label: 'Others', selector: {roles: 'nogexecbots'}}
  ]

  @selection = new ReactiveDict [@roles.length]
  for idx in [0...@roles.length]
    @selection.set(idx, false)
  @filterText = new ReactiveVar
  @selector = new ReactiveDict {}
  @nUsersFiltered = new ReactiveVar

  @autorun =>
    selectors = []
    for idx in[0...@roles.length]
      if @selection.get(idx)
        selectors.push(@roles[idx].selector)
    @selector.set(roles: selectors)

    selText = []
    if text = @filterText.get()
      selText.push({'username': {$regex: '.*'+ text + '.*'}})
      selText.push({'emails.address': {$regex: '.*'+ text + '.*'}})
    @selector.set(text: selText)


Template.userList.helpers
  users: ->
    tpl = Template.instance()
    roles = [{}]
    text = [{}]
    if tpl.selector.get('roles').length > 0
      roles = tpl.selector.get('roles')
    if tpl.selector.get('text').length > 0
      text = tpl.selector.get('text')
    sel = {$and: [{$or: roles}, {$or: text}]}
    users = Meteor.users.find(sel)
    tpl.nUsersFiltered.set(users.count())
    return users

  filterOptions: ->
    tpl = Template.instance()
    {
      roles: tpl.roles
      selection: tpl.selection
      nUsers: Meteor.users.find().count()
      nUsersFiltered: tpl.nUsersFiltered.get()
    }


Template.userList.events
  'click .js-toggle-role': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    for idx in [0...tpl.roles.length]
      if tpl.roles[idx].label == @label
        tpl.selection.set(idx, !tpl.selection.get(idx))

  'keyup .js-filter-text': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.filterText.set(ev.target.value)


Template.userListFilter.helpers
  roles: ->
    for idx in [0...@roles.length]
      {
        label: @roles[idx].label,
        selected: @selection.get(idx)
      }


Template.userListItem.onCreated ->
  @isDeletingUser = new ReactiveVar false


# Don't delete self.
canDeleteUser = (uid) ->
  if uid is Meteor.userId()
    false
  else
    testAccess 'accounts/delete'


canModifyRoles = (uid) ->
  testAccess 'accounts/modifyRoles'


# Don't remove self from admins.
canRemoveRoleAdmins = (uid) ->
  if uid is Meteor.userId()
    false
  else
    canModifyRoles uid


Template.userListItem.helpers
  email: -> @emails?[0].address

  userRoles: ->
    if @roles
      return @roles.sort().join(', ')
    else
      return "-"

  isCurrentUser: -> @_id is Meteor.userId()
  isUser: -> Roles.userIsInRole(@_id, 'users')
  isAdmin: -> Roles.userIsInRole(@_id, 'admins')
  cannotDeleteUser: -> not canDeleteUser @_id
  cannotModify: -> not canModifyRoles @_id
  cannotRemoveRoleAdmins: -> not canRemoveRoleAdmins @_id

  isDeletingUser: -> Template.instance().isDeletingUser.get()


Template.userListItem.events
  'click .js-delete-user-start': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeletingUser.set true

  'click .js-delete-user-cancel': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeletingUser.set false

  'click .js-delete-user-confirm': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeletingUser.set false
    Meteor.call 'removeUser', @_id, (err) ->
      if err?
        defaultErrorHandler(err)

  'click .js-add-role-users': (ev) ->
    ev.preventDefault()
    Meteor.call 'addRoleUsers', @_id, (err) ->
      if err?
        defaultErrorHandler(err)

  'click .js-remove-role-users': (ev) ->
    ev.preventDefault()
    Meteor.call 'removeRoleUsers', @_id, (err) ->
      if err?
        defaultErrorHandler(err)

  'click .js-add-role-admins': (ev) ->
    ev.preventDefault()
    Meteor.call 'addRoleAdmins', @_id, (err) ->
      if err?
        defaultErrorHandler(err)

  'click .js-remove-role-admins': (ev) ->
    ev.preventDefault()
    Meteor.call 'removeRoleAdmins', @_id, (err) ->
      if err?
        defaultErrorHandler(err)


Template.botKeys.helpers
  bots: ->
    Meteor.users.find {username: /^nog.*bot/}
