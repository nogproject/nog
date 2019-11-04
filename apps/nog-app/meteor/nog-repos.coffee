{
  ERR_UPDATE
  nogthrow
} = NogError


@NogRepoSettings =
  call: {}


wasRemoved = (repoId) ->
  unless ( NogContent.repos.findOne(repoId) )
    return repoId


defMethod = (name, func) ->
  qualname = 'NogRepoSettings.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogRepoSettings.call[name] = (args...) -> Meteor.call qualname, args...


defMethod 'addToRecentRepos', (opts) ->
  check opts,
    repoId: String

  if Meteor.isClient
    return

  userId = Meteor.userId()
  unless userId?
    return

  Meteor.defer ->
    if (doc =
        Meteor.users.findOne({_id: userId}, {fields: {recentRepos: 1}}))?
      if (doc.recentRepos)?
        idsOfRecentRepos = doc.recentRepos.map((repo) -> repo.repoId)
        removedRecentRepos = idsOfRecentRepos.filter(wasRemoved)
        if removedRecentRepos.length == 0
          return
        Meteor.users.update(
          {
            _id: doc._id
          }, {
            $pull: {recentRepos: {repoId: {$in: removedRecentRepos}}}
          }
        )

  {repoId} = opts
  maxNumRepos = Meteor.settings.maxNumRecentRepos ? 10

  # The list of recently visited repos is not necessarily sorted by date.  A
  # final sort by date must be done somewhere else if needed.
  # The update process first checks if a `repoId` already exists. If so, it
  # updates the corresponding `date` without any sorting, which is not
  # necessary because no entry must be selected and rejected.  Otherwise, it
  # adds an entry with `repoId` and `date`, sorts all entries by most recent
  # date, and keeps only the `maxNumRepos` most recent in the list.
  n = Meteor.users.update(
      {
        _id: userId,
        'recentRepos.repoId': repoId
      }, {
      $set:
        {
          'recentRepos.$.date': new Date()
        }
    }
  )
  if n isnt 1
    n = Meteor.users.update(
      {
        _id: userId,
      }, {
        $push: {
          recentRepos: {
            $each: [ {repoId: repoId, date: new Date()} ],
            $sort: {date: -1}
            $slice: maxNumRepos
          }
        }
      }
    )
    if n isnt 1
      nogthrow ERR_UPDATE, {reason: 'Failed to update list of recent repos.'}
