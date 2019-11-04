{
  ERR_UPDATE
  nogthrow
} = NogError


@NogWidget =
  call: {}


wasRemoved = (repoId) ->
  unless (NogContent.repos.findOne(repoId))
    return repoId


defMethod = (name, func) ->
  qualname = 'NogWidget.' + name
  def = {}
  def[qualname] = func
  Meteor.methods def
  NogWidget.call[name] = (args...) -> Meteor.call qualname, args...


defMethod 'toggleRepoPin', (opts) ->
  check opts,
    repoId: String

  if Meteor.isClient
    return

  userId = Meteor.userId()
  unless userId?
    return

  Meteor.defer ->
    if (doc =
        Meteor.users.findOne({_id: userId}, {fields: {favoriteRepos: 1}}))?
      if doc.favoriteRepos?
        removedFavoriteRepos = doc.favoriteRepos.filter(wasRemoved)
        if removedFavoriteRepos.length == 0
          return
        Meteor.users.update(
          {
            _id: doc._id
          }, {
            $pull: {favoriteRepos: {$in: removedFavoriteRepos}}
          }
        )

  {repoId} = opts
  maxNumRepos = Meteor.settings.maxNumFavoriteRepos ? 100

  # The first update creates `numFavoriteRepos`, since the later conjunction of
  # $push and $inc fails to create two new fields (`favoriteRepos` and
  # `numFavoriteRepos`) at the same time.
  Meteor.users.update(
    {
      _id: userId,
      numFavoriteRepos: {$exists: false}
    }, {
      $inc: {numFavoriteRepos: 0}
    }
  )

  # The toggling process first checks if the `repoId` is already in the field
  # `favoriteRepos` and removes it if so.  If not, it then tries to add the
  # `repoId` to `favoriteRepos`.  On update failure, it checks if `maxNumRepos`
  # was reached and throws an error incl. an hint, otherwise it throws a
  # general error.
  n = Meteor.users.update(
    {
      _id: userId,
      favoriteRepos: repoId,
    }, {
      $pull: { favoriteRepos: repoId },
      $inc: { numFavoriteRepos: -1 },
    }
  )
  if n isnt 1
    n = Meteor.users.update(
      {
        _id: userId,
        favoriteRepos: { $ne: repoId },
        numFavoriteRepos: { $lt: maxNumRepos },
      }, {
        $push: { favoriteRepos: repoId },
        $inc: { numFavoriteRepos: 1 }
      }
    )
    if n isnt 1
      doc = Meteor.users.findOne({_id: userId}, {fields: {numFavoriteRepos: 1}})
      if doc.numFavoriteRepos >= maxNumRepos
        nogthrow ERR_UPDATE, {
          reason: '
            Cannot add the repo to your favorites: Maximal number of favorite
            repos exceeded. Consider reducing your favorite list.
          '
        }
      else
        nogthrow ERR_UPDATE, {
          reason: 'Failed to toggle the favorite state.'
        }
