{
  checkAccess
  testAccess
} = NogAccess

{
  ERR_ACCOUNT_DELETE
  nogthrow
} = NogError

Meteor.methods
  removeUser: (otherUserId) ->
    check otherUserId, String
    if @userId is otherUserId
      nogthrow ERR_ACCOUNT_DELETE, {
          reason: 'Cannot delete the account of the current user.'
        }
    checkAccess @userId, 'accounts/delete'
    Meteor.users.remove {_id: otherUserId}
    return

  addRoleUsers: (otherUserId) ->
    check otherUserId, String
    checkAccess @userId, 'accounts/modifyRoles'
    Roles.addUsersToRoles otherUserId, 'users'
    return

  removeRoleUsers: (otherUserId) ->
    check otherUserId, String
    checkAccess @userId, 'accounts/modifyRoles'
    Roles.removeUsersFromRoles otherUserId, 'users'
    return

  addRoleAdmins: (otherUserId) ->
    check otherUserId, String
    checkAccess @userId, 'accounts/modifyRoles'
    Roles.addUsersToRoles otherUserId, 'admins'
    return

  removeRoleAdmins: (otherUserId) ->
    check otherUserId, String
    checkAccess @userId, 'accounts/modifyRoles'
    Roles.removeUsersFromRoles otherUserId, 'admins'
    return


# Publish all user accounts for administrative access
Meteor.publish 'accountsList', ->
  unless testAccess @userId, 'accounts/adminView'
    @ready
    return null
  Meteor.users.find {}, {
      fields:
        username: 1
        roles: 1
        emails: 1
        'services.nogauth.keys.keyid': 1
        'services.nogauth.keys.createDate': 1
        'services.nogauth.keys.comment': 1
    }
