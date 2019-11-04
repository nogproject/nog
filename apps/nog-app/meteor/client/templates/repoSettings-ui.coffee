{defaultErrorHandler} = NogError


Template.repoSettings.helpers
  ownerName: -> FlowRouter.getParam('ownerName')
  repoName: -> FlowRouter.getParam('repoName')
  repoSettingsCtx: ->
    {
      ownerName: FlowRouter.getParam('ownerName')
      repoName: FlowRouter.getParam('repoName')
    }


Template.repoSettingsRename.onCreated ->
  @error = new ReactiveVar()
  @action = new ReactiveVar()

Template.repoSettingsRename.helpers
  error: ->
    tpl = Template.instance()
    tpl.error.get()

  action: ->
    tpl = Template.instance()
    tpl.action.get()

Template.repoSettingsRename.events
  'click .js-rename': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    newName = tpl.$('.js-repo-name-text').val()
    if newName == @repoName
      tpl.error.set 'Repo name unchanged.'
      return
    tpl.error.set null
    ownerName = @ownerName
    repoName = @repoName
    opts =
      old: {ownerName, repoName}
      new: {repoFullName: "#{ownerName}/#{newName}"}
    tpl.action.set 'renaming...'
    NogContent.call.renameRepo opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      FlowRouter.go "/#{ownerName}/#{newName}/files"


Template.repoSettingsDelete.onCreated ->
  @disabled = new ReactiveVar 'disabled'

Template.repoSettingsDelete.helpers
  disabled: ->
    tpl = Template.instance()
    tpl.disabled.get()

Template.repoSettingsDelete.events
  'click .js-start-delete': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.$('.js-delete-modal').modal()

  'keyup .js-confirm-repo-name': (ev) ->
    ownerName = @ownerName
    repoName = @repoName
    tpl = Template.instance()
    if ev.target.value == "#{ownerName}/#{repoName}"
      tpl.disabled.set null
    else
      tpl.disabled.set 'disabled'

  'click .js-delete-forever': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    ownerName = @ownerName
    repoName = @repoName
    opts = {ownerName, repoName}
    NogContent.call.deleteRepo opts, (err, res) ->
      modal = tpl.$('.js-delete-modal')
      modal.on 'hidden.bs.modal', ->
        if err
          return defaultErrorHandler err
        FlowRouter.go('/')
      modal.modal('hide')
