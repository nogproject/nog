Meteor.publish null, ->
  if @userId?
    Meteor.users.find {_id: @userId}, {fields: {favoriteRepos: 1}}
  else
    null
