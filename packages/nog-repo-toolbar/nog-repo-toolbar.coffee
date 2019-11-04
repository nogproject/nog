{defaultErrorHandler} = NogError

# Ideally `repoToolbar` and `forkedFrom` receive their dependencies
# via the data context.
# One recommended parent to load these templates is `nogRepoToolbarPresenter`.
# It manages the subscription of the current repo and injected dependencies and
# sends the required information to the template contexts.
# XXX: But there are legacy parents that do not inject dependencies (i.e., in
# file, technical and workspace view). For those parents, the template still
# needs to use globals.


# We assume that the templates has a smart parent if the template's context
# contains `router`.
# For legacy parents, globals `FlowRouter` and `NogAccess` are used.
Template.repoToolbar.onCreated ->
  @sharingIsVisible = new ReactiveVar(false)
  @action = new ReactiveVar(null)

  @router = ->
    data = Template.instance().data
    return data.router ? FlowRouter

  @testAccess = (args...) ->
    fn = Template.instance().data.testAccess ? NogAccess.testAccess
    return fn(args...)

  @meteorUser = ->
    user = Template.instance().data.meteorUser ? Meteor.user
    return user()

  @ownerName = -> @router().getParam('ownerName')
  @repoName = -> @router().getParam('repoName')

  @viewerParams = ->
    routeName = @router().getRouteName()
    if routeName == 'repoTree'
      refTreePath = 'master'
    else
      refTreePath = ''
    return {routeName, refTreePath}

  # Use `cdat.router` as an indicator that we have a smart parent that injects
  # dependencies.  If so, validate the full data context.
  @autorun ->
    cdat = Template.currentData()
    tpl = Template.instance()
    unless cdat.router?
      return
    check cdat, {
      router: Match.Any
      testAccess: Function
      meteorUser: Function
      onForkRepo: Function
      forkAction: Function
    }
    tpl.action.set(cdat.forkAction())


Template.repoToolbar.helpers
  isOwnRepo: ->
    tpl = Template.instance()
    user = tpl.meteorUser()
    if not user?
      return false
    return (tpl.ownerName() == user.username)

  sharingIsVisible: -> Template.instance().sharingIsVisible.get()

  sharingCtx: ->
    tpl = Template.instance()
    return {
      ownerName: tpl.ownerName()
      repoName: tpl.repoName()
    }

  mayAccessSetting: ->
    tpl = Template.instance()
    # `modify` is used as a substitute for a more precise access check whether
    # a user is allowed to modify the settings.
    aopts = {
      ownerName: tpl.ownerName()
      repoName: tpl.repoName()
    }
    return tpl.testAccess 'nog-content/modify', aopts

  mayFork: ->
    tpl = Template.instance()
    aopts = {
      ownerName: tpl.ownerName()
      repoName: tpl.repoName()
    }
    return tpl.testAccess 'nog-content/fork-repo', aopts

  action: ->
    tpl = Template.instance()
    tpl.action.get()


Template.repoToolbar.events
  'click .js-show-share-settings': (ev) ->
    ev.preventDefault()
    sharingIsVisible = Template.instance().sharingIsVisible
    sharingIsVisible.set(!sharingIsVisible.get())

  'click .js-repo-settings': (ev) ->
    tpl = Template.instance()
    ev.preventDefault()
    ownerName = tpl.ownerName()
    repoName = tpl.repoName()
    tpl.router().go "/#{ownerName}/#{repoName}/settings"

  'click .js-fork-repo': (ev) ->
    tpl = Template.instance()
    cdat = Template.currentData()
    ev.preventDefault()
    if cdat.onForkRepo?
      cdat.onForkRepo()
    else
      ownerName = tpl.ownerName()
      repoName = tpl.repoName()
      opts =
        old: {ownerName, repoName}
        new: {ownerName: tpl.meteorUser().username}
      # Capture state that accessed the data context via `router()` here,
      # because the data context is not available in the callback.
      router = tpl.router()
      {routeName, refTreePath} = tpl.viewerParams()
      tpl.action.set 'Forking...'
      NogContent.call.forkRepo opts, (err, res) ->
        tpl.action.set null
        if err
          return defaultErrorHandler err
        if (res)
          params = {
            ownerName: res.owner
            repoName: res.name
            refTreePath
          }
          router.go routeName, params


# We assume that the templates has a smart parent if the template's context
# contains `forkedFrom`.
# For legacy parents, the global `FlowRouter`is used.
Template.forkedFrom.onCreated ->
  data = Template.instance().data
  @routeName = ->
    if data.routeName
      return data.routeName()
    else
      return FlowRouter.getRouteName()

  unless data.forkedFrom
    @ownerName = -> FlowRouter.getParam('ownerName')
    @repoName = -> FlowRouter.getParam('repoName')

  @autorun ->
    cdat = Template.currentData()
    unless cdat.forkedFrom?
      return
    check cdat, {
      forkedFrom: Function
      routeName: Function
    }

Template.forkedFrom.helpers
  routeName: ->
    tpl = Template.instance()
    routeName = tpl.routeName()
    if routeName == 'repoTree'
      routeName = 'tree/master'
    return routeName

  forkedFrom: ->
    cdat = Template.currentData()
    if (cdat.forkedFrom)
      if (f = cdat.forkedFrom())?
        return "#{f.owner}/#{f.name}"
    else
      tpl = Template.instance()
      sel = {owner: tpl.ownerName(), name: tpl.repoName()}
      unless (repo = NogContent.repos.findOne(sel))?
        return null
      unless (f = repo.forkedFrom)?
         return null
      return "#{f.owner}/#{f.name}"
    return null
