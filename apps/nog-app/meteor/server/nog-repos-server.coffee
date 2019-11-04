Meteor.publish null, ->
  if @userId?
    Meteor.users.find {_id: @userId}, {fields: {recentRepos: 1}}
  else
    null
