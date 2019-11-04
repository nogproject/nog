if not Meteor.settings?.public?.optTestingUsers
  console.log '[app] Testing users are disabled.'
  return


reassignRepos = (username) ->
  # Update repos in case the testing user got lost.
  if (user = Meteor.users.findOne {username})?
    NogContent.repos.find({
      owner: username, ownerId: {$ne: user._id}
    }).map (repo) ->
      NogContent.repos.update {_id: repo._id}, {$set: {ownerId: user._id}}


ensureUser = (username, password, email, ldapUsername, ldapgroups) ->
  if not Meteor.users.findOne({username})?
    uid = Accounts.createUser {username, password, email}
    console.log "[testinguser] Created user #{username}."
    Roles.addUsersToRoles(uid, 'users')
    Meteor.users.update({ username }, { $set: {
      'services.gittest.username': ldapUsername,
      'services.gittest.ldapgroups': ldapgroups,
    }})


if Meteor.isServer
  Meteor.startup ->
    password = Meteor.settings.public?.tests?.passwords?.user
    check password, String

    username = 'sprohaska'
    email = 'sprohaska@gmail.com'
    ldapUsername = 'bob'
    ldapgroups = []
    ensureUser username, password, email, ldapUsername, ldapgroups
    reassignRepos username

    username = 'alovelace'
    email = 'homberg@zib.de'
    ldapUsername = 'alice'
    ldapgroups = ['ou_ag-alice', 'srv_lm1']
    ensureUser username, password, email, ldapUsername, ldapgroups
    reassignRepos username
