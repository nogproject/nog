{
  defaultErrorHandler
} = NogError

KEY_ENTER = 13


Template.repoSharing.helpers
  optNogSharing: -> Meteor.settings.public?.optNogSharing ? true
  isOwnRepo: -> (@ownerName == Meteor.user().username)


Template.repoSharingContent.helpers
  isPublic: ->
    repoSel = {owner: @ownerName, name: @repoName}
    if not (repo = NogContent.repos.findOne repoSel)?
      return null
    repo.sharing?.public ? false

  isAllCircles: ->
    repoSel = {owner: @ownerName, name: @repoName}
    if not (repo = NogContent.repos.findOne repoSel)?
      return null
    repo.sharing?.allCircles ? false

  # The implementation is based on the assumption that the sharing info is only
  # displayed if the current user is the owner: The current user can be used to
  # resolve the circles names.
  circles: ->
    repoSel = {owner: @ownerName, name: @repoName}
    if not (repo = NogContent.repos.findOne repoSel)?
      return null
    if not (user = Meteor.user())?
      return null
    ownerCircles = {}
    for c in user.sharing?.circles ? []
      ownerCircles[c._id] = c
    repoCircles = []
    for cid in repo.sharing?.circles ? []
      if (c = ownerCircles[cid])?
        repoCircles.push c
    repoCircles

  ownerCircles: ->
    if not (user = Meteor.user())?
      return null
    user.sharing?.circles


Template.repoSharingContent.events

  # Don't prevent default: toggling is ok.
  'click .js-toggle-public': (ev) ->
    opts =
      ownerName: @ownerName
      repoName: @repoName
      public: ev.target.checked
    NogSharing.call.updateRepoSharing opts, (err, res) ->
      if err
        defaultErrorHandler(err)

  # Don't prevent default: toggling is ok.
  'click .js-toggle-all-circles': (ev) ->
    opts =
      ownerName: @ownerName
      repoName: @repoName
      allCircles: ev.target.checked
    NogSharing.call.updateRepoSharing opts, (err, res) ->
      if err
        defaultErrorHandler(err)

  'click .js-add-circle': (ev) ->
    ev.preventDefault()
    opts =
      ownerName: Template.currentData().ownerName
      repoName: Template.currentData().repoName
      addCircleName: @name
    NogSharing.call.updateRepoSharing opts, (err, res) ->
      if err
        defaultErrorHandler(err)


Template.repoSharingCircle.onCreated ->
  @isDeleting = new ReactiveVar false


Template.repoSharingCircle.helpers
  isDeleting: -> Template.instance().isDeleting.get()


Template.repoSharingCircle.events
  'click .js-no-action, mousedown .js-no-action': (ev) ->
    ev.preventDefault()

  'click .js-delete-start': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set true

  'click .js-delete-cancel': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false

  'click .js-delete-confirm': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false
    opts =
      ownerName: Template.parentData().ownerName
      repoName: Template.parentData().repoName
      removeCircleId: @_id
    NogSharing.call.updateRepoSharing opts, (err, res) ->
      if err
        defaultErrorHandler(err)


Template.manageCircles.helpers
  optNogSharing: -> Meteor.settings.public?.optNogSharing ? true


Template.manageCirclesContent.onCreated ->
  @subscribe 'circles'


Template.manageCirclesContent.events
  'keypress .js-create-circle-text': (ev) ->
    if ev.which == KEY_ENTER
      ev.preventDefault()
      opts =
        name: ev.target.value
      NogSharing.call.createCircle opts, (err, res) ->
        if err
          defaultErrorHandler(err)
        else
          ev.target.value = ''

  'click .js-create-circle': (ev, tpl) ->
    ev.preventDefault()
    text = tpl.$('.js-create-circle-text')
    opts = {name: text.val()}
    NogSharing.call.createCircle opts, (err, res) ->
      if err
        defaultErrorHandler(err)
      else
        text.val('')


Template.circlesList.helpers
  circles: ->
    if not (user = Meteor.user())?
      return null
    user.sharing?.circles


Template.circlesListItem.onCreated ->
  @isDeleting = new ReactiveVar false


Template.circlesListItem.helpers
  isDeleting: -> Template.instance().isDeleting.get()
  members: -> NogSharing.shares.find {circleId: @_id}


Template.circlesListItem.events
  'click .js-no-action, mousedown .js-no-action': (ev) ->
    ev.preventDefault()

  'click .js-delete-circle-start': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set true

  'click .js-delete-circle-cancel': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false

  'click .js-delete-circle-confirm': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false
    opts =
      name: @name
    NogSharing.call.deleteCircle opts, (err, res) ->
      if err
        defaultErrorHandler(err)

  'keypress .js-add-member-text': (ev) ->
    if ev.which == KEY_ENTER
      ev.preventDefault()
      opts =
        circleName: @name
        toName: ev.target.value
      NogSharing.call.extendCircle opts, (err, res) ->
        if err
          defaultErrorHandler(err)
        else
          ev.target.value = ''

  'click .js-add-member': (ev, tpl) ->
    ev.preventDefault()
    text = tpl.$('.js-add-member-text')
    opts =
      circleName: @name
      toName: text.val()
    NogSharing.call.extendCircle opts, (err, res) ->
      if err
        defaultErrorHandler(err)
      else
        text.val('')


Template.circleMembersItem.onCreated ->
  @isDeleting = new ReactiveVar false


Template.circleMembersItem.helpers
  isDeleting: -> Template.instance().isDeleting.get()


Template.circleMembersItem.events
  'click .js-no-action, mousedown .js-no-action': (ev) ->
    ev.preventDefault()

  'click .js-delete-member-start': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set true

  'click .js-delete-member-cancel': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false

  'click .js-delete-member-confirm': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false
    opts =
      circleName: Template.parentData().name
      toName: @toName
    NogSharing.call.shrinkCircle opts, (err, res) ->
      if err
        defaultErrorHandler(err)
