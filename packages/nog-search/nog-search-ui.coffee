{ NogError } = require 'meteor/nog-error'
{ Template } = require 'meteor/templating'
{ searchContent } = require './nog-search.coffee'
{ easySearchInit } = require './nog-search-ui.js'
{defaultErrorHandler} = NogError

routerPath = (route, opts) -> FlowRouter.path route, opts

mayModifyRepo = (repo) ->
    aopts = {ownerName: repo.owner, repoName: repo.name}
    NogAccess.testAccess 'nog-content/modify', aopts


SEARCH_LIMIT = Meteor.settings.public.searchNumResults


Template.search.onCreated ->
  @action = new ReactiveVar()
  @updatedResults = new ReactiveVar()
  @hasMoreDocuments = new ReactiveVar(false)

  @searchInput = ''
  @inputString = new ReactiveVar('')

  easySearchInit.component.set(easySearchInit.isCreated, false)

  # Some EasySearch issues cause flickering components:
  # EasySearch returns the cursor all the time, no matter if the subscription
  # is ready or not, which alternately results in an empty and populated
  # cursor.  On the other hand, `isReady()` of the EasySearch cursor only
  # returns if `count()` is defined or not; it does not represent the exact
  # subscription state.  Flickering then appears, when the template is
  # repeatedly rendered with and without the result list (same for load-more
  # button and `NogComponent`).
  # To avoid flickering, we use the `ready()` function of Meteor subscriptions
  # to update only when the subscription is ready.  Then, `updatedResults`
  # keeps the last valid result list until the next search is ready so that
  # the template only gets the final result and is not rendered with
  # intermediate state.
  @autorun =>
    unless easySearchInit.component.get(easySearchInit.isCreated)
      return
    if @searchInput != @inputString.get()
      @searchInput = @inputString.get()
      NogContent.searchContent
        .getComponentMethods()
        .search(@inputString.get())
    if (cursor = NogContent.searchContent.getComponentMethods().getCursor())?
      if (sub = cursor._publishHandle)?
        if sub.ready()
          @updatedResults.set(cursor.mongoCursor.fetch())
          dict = NogContent.searchContent.getComponentDict()
          hasMore = dict.get('count') > dict.get('currentCount')
          @hasMoreDocuments.set(hasMore)
          if (options = dict.get('searchOptions'))?
            unless options.limit?
              options.limit = SEARCH_LIMIT
            dict.set('searchOptions', options)


Template.search.helpers
  content: ->
    NogContent.searchContent

  inputFormParams: ->
    tpl = Template.instance()
    {
      inputFormLabel: 'Search'
      onUpdateInput: (str) ->
        tpl.inputString.set(str)
      router: @router
    }

  result: ->
    Template.instance().updatedResults.get()

  action: ->
    Template.instance().action.get()

  refHref: ->
    source = Template.parentData()
    srcRepo = @
    [ns, branchName] = srcRepo.refName.split '/'
    path = source.path
    {ownerName, repoName} = srcRepo
    text = ownerName + '/' + repoName + '/' + branchName + ':' + path
    routeName = 'files'
    href = routerPath routeName, {
        ownerName, repoName,
        treePath: path
      }
    {text, href, srcRepo, source}

  nogModalMode: ->
    if (modalTargetRepo = NogModal.get('targetRepo'))?
      if (mayModifyRepo(modalTargetRepo.repo))
        if NogModal.get('addingData')
          {
            srcRepo: @srcRepo
            source: @source
            trgRepo: modalTargetRepo.repo
            mayAddData: true
          }
        else if NogModal.get('addingPrograms')
          kinds = ['programRegistry']
          if (NogContent.repos.findOne {
                  owner: @srcRepo.ownerName, name: @srcRepo.repoName,
                  kinds: kinds})
            if (@source.path.split('/').length == 2)
              {
                srcRepo: @srcRepo
                source: @source
                trgRepo: modalTargetRepo.repo
                mayAddPrograms: true
              }

  optTextSearch: ->
    Meteor.settings.public?.optTextSearch ? true

  hasMoreDocuments: ->
    Template.instance().hasMoreDocuments.get()


Template.search.events
  'click .js-add-to-target': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    ctx = @
    tpl = Template.instance()
    opts =
      src:
        ownerName: ctx.srcRepo.ownerName
        repoName: ctx.srcRepo.repoName
        commitId: ctx.srcRepo.commitId
        entries: [
            {
              type: ctx.source.type,
              sha1: ctx.source.sha1
            }
          ]
      dst:
        ownerName: ctx.trgRepo.owner
        repoName: ctx.trgRepo.name
    tpl.action.set 'adding'
    NogFiles.call.addToDatalist opts, (err, res) ->
      if err
        tpl.action.set null
        return defaultErrorHandler err
      clearAction = ->
        tpl.action.set null
      setTimeout clearAction, 1000

  'click .js-add-program-to-target': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    ctx = @
    opts =
      src:
        ownerName: ctx.srcRepo.ownerName
        repoName: ctx.srcRepo.repoName
        commitId: ctx.srcRepo.commitId
        entries: [
            {sha1: ctx.source.sha1}
          ]
      dst:
        ownerName: ctx.trgRepo.owner
        repoName: ctx.trgRepo.name
    tpl.action.set 'adding'
    NogFiles.call.addProgram opts, (err, res) ->
      if err
        tpl.action.set null
        return defaultErrorHandler err
      clearAction = ->
        tpl.action.set null
      setTimeout clearAction, 1000

  'click .js-loadMore': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    dict = NogContent.searchContent.getComponentDict()
    currentCount = dict.get('currentCount')
    options = dict.get('searchOptions')
    options.limit = currentCount + SEARCH_LIMIT
    dict.set('searchOptions', options)
