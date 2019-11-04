Meteor.subscribe 'repos'
Meteor.subscribe 'sharedRepos'


OWNER_NAME = 'Owner name'
REPO_NAME = 'Repo name'
RECENT = 'Recently updated'

sortOptions = [
  {
    name: 'owner'
    displayName: OWNER_NAME
  },{
    name: 'repo'
    displayName: REPO_NAME
  },{
    name: 'recent'
    displayName: RECENT
  }
]

getIcon = (repo) ->
  icon = ''
  if _.isObject(repo.kinds)
    if 'programRegistry' in repo.kinds
      icon = 'programRegistry-128.png'
    else if 'workspace' in repo.kinds
      icon = 'workspace-128.png'
    else
      icon = 'fileRepo-128.png'
  if icon
    return '/images/' + icon
  else
    return null


Template.repos.onCreated ->
  @filterText = new ReactiveVar('')
  @filterSelector = new ReactiveVar()
  @displayedTab = new ReactiveVar('')
  @selectedSort = new ReactiveVar(OWNER_NAME)

  @autorun =>
    text = @filterText.get()
    parts = text.split('/')
    sel = {}

    # `parts[]` of the filter string are used to filter owner and/or repo
    # names without any escaping, so that each part can be used as a regular
    # expression.
    # Examples:
    #   - simple substrings: 'ad', 'bar' or 'ad/bar' to match the repo
    #     'ada/foo-bar'
    #   - regex: `^[as]/^t` to match repos of users that start with either 'a'
    #     or 's' and the repo name starts with 't'.

    if parts.length > 0 and parts.length < 3
      if parts[0]? and parts[0] != ''
        sel.$or = [
          {owner: {$regex: '.*'+ parts[0] + '.*', '$options' : 'i'}},
          {name: {$regex: '.*'+ parts[0] + '.*', '$options' : 'i'}}
        ]
      if parts[1]? and parts[1] != ''
        sel.name = {$regex: '.*'+ parts[1] + '.*', '$options' : 'i'}
    @filterSelector.set(sel)


Template.repos.onRendered ->
  Template.instance().displayedTab.set(location.hash)
  if location.hash != ''
    $('a[href="'+location.hash+'"]').tab('show')


Template.repos.helpers
  displaysRecentRepos: ->
    return Template.instance().displayedTab.get() == '#recent'

  filterText: ->
    return {
        text: Template.instance().filterText.get()
      }

  sortSelection: ->
    return {
        opt: Template.instance().selectedSort.get()
      }

  displayOptions: ->
    {
      filter: Template.instance().filterSelector.get()
      selectedSort: Template.instance().selectedSort.get()
    }


Template.repos.events
  'click .js-toggle-tabs': (ev) ->
    ev.preventDefault()
    location.hash = ev.currentTarget.hash
    Template.instance().displayedTab.set(location.hash)

  'keyup .js-filter-repos': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.filterText.set(ev.target.value)

  'change .js-select-sort': (ev) ->
    ev.preventDefault()
    Template.instance().selectedSort.set(ev.currentTarget.value)


Template.registerHelper 'selectedRepo', () =>
  tpl = Template.instance()
  textFilter = tpl.data.filter
  selectedSort = tpl.data.selectedSort

  opts = Template.currentData()
  check opts,
    selector: Object
    showFullName: Boolean
    displayOptions: Boolean

  {selector, showFullName, displayOptions} = opts
  if displayOptions
    _.extend(selector, textFilter)

  if NogModal.get('addingPrograms')
    selector.kinds = "programRegistry"

  repos = NogContent.repos.find selector

  repoList = repos.fetch().map (elt) ->
    elt.showFullName = showFullName
    elt.modified = 'n/a'
    if elt.lastCommitDate?
      if (moment(elt.lastCommitDate).isValid())
        elt.modified = moment(elt.lastCommitDate).format("MMM DD, YYYY, h:mm a")
    elt.icon = getIcon(elt)
    return elt

  if displayOptions
    if selectedSort == OWNER_NAME
      repoList.sort (a, b) ->
        return a.owner.toLowerCase().localeCompare(b.owner.toLowerCase()) ||
          a.name.toLowerCase().localeCompare(b.name.toLowerCase())
    if selectedSort == REPO_NAME
      repoList.sort (a, b) ->
        return a.name.toLowerCase().localeCompare(b.name.toLowerCase()) ||
          a.owner.toLowerCase().localeCompare(b.owner.toLowerCase())
    if selectedSort == RECENT
      repoList.sort (a, b) ->
        dateA = a.lastCommitDate ? moment(0).toDate()
        dateB = b.lastCommitDate ? moment(0).toDate()
        return dateB - dateA

  return repoList


Template.ownReposList.helpers
  ownReposOpts: ->
    opts = {
      selector: {ownerId: Meteor.userId()}
      showFullName: false
      displayOptions: true
    }


Template.sharedReposList.helpers
  sharedReposOpts: ->
    opts = {
      selector: {ownerId: {$ne: Meteor.userId()}}
      showFullName: true
      displayOptions: true
    }


Template.allReposList.helpers
  allReposOpts: ->
    opts = {
      selector: {}
      showFullName: true
      displayOptions: true
    }


Template.favoriteReposList.helpers
  favoriteReposOpts: ->
    if (favoriteRepos = Meteor.user().favoriteRepos)?
      opts = {
        selector: {_id: {$in: favoriteRepos}}
        showFullName: true
        displayOptions: true
      }
    else
      return null


Template.recentReposList.onCreated ->
  # The list of recently visited repos is supposed to only update on page
  # reload or on pressend refresh button.  The list should not change, for
  # example, when the user calls several repos in a row from that list.
  # `smallReload` tracks whether an update is requested or not.  In case of an
  # update, `orderedRecentRepos` gets the new list and triggers a re-rendering
  # of the list. Otherwise, it preserves the old one.
  @orderedRecentRepos = new ReactiveVar(null)
  @smallReload = new ReactiveVar(true)


Template.recentReposList.helpers
  recentReposOpts: ->
    if (recentRepos = Meteor.user().recentRepos)?
      repoIds = recentRepos.map (elt) -> elt.repoId
      opts = {
        selector: {_id: {$in: repoIds}}
        showFullName: true
        displayOptions: false
      }
    else
      return null

  recentReposOrdered: ->
    tpl = Template.instance()
    if tpl.smallReload.get()
      if (recentRepos = Meteor.user().recentRepos)?
        touchDate = {}
        for elt in recentRepos
          touchDate[elt.repoId] = elt.date
        orderedRepos = _.sortBy @, (repo) -> -touchDate[repo._id]
        tpl.smallReload.set(false)
        tpl.orderedRecentRepos.set(orderedRepos)
    return tpl.orderedRecentRepos.get()


Template.recentReposList.events
  'click .js-reload-recent': (ev) ->
    ev.preventDefault()
    Template.instance().smallReload.set(true)


Template.reposItem.helpers
  isWorkspace: -> 'workspace' in (@kinds ? [])
  iskindCatalog: -> 'catalog' in (@kinds ? [])

  isNogModalMode: ->
    return NogModal.get('addingPrograms') or NogModal.get('addingData')

  repoFavorites: ->
    repoId = Template.currentData()._id
    favoriteRepos = Meteor.user().favoriteRepos ? []
    isFavorite = repoId in favoriteRepos
    {
      repoId
      isFavorite
    }


Template.reposSort.helpers
  sortOption: ->
    for i in sortOptions
      {
        name: i.name
        displayName: i.displayName
        selected: @opt == i.displayName
      }
