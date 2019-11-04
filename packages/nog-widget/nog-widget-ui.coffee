{defaultErrorHandler} = NogError

Template.initTooltips.onRendered ->
  @$('[data-toggle="tooltip"]').tooltip()

# To avoid hanging tooltips, call explicit `tooltip('hide')`.  It must be
# called on a jQuery object.  XXX: It is unclear why `ev.currentTarget` works,
# while `ev.target` does not work.  It is also unclear, why we need this
# workaround at all.  See related SO answer:
# <http://stackoverflow.com/questions/10545952/tooltip-remains-sometimes-on-page-on-element-inside-sliding-div>
Template.initTooltips.events
  'click [data-toggle="tooltip"]': (ev) ->
    $(ev.currentTarget).tooltip('hide')


# This template is to incorporate a bookmarking concept to create a short list
# of important repos by toggling the repo pin.
Template.repoPin.events
  'click .js-toggle': (ev) ->
    ev.preventDefault()
    repoId = ev.currentTarget.id
    NogWidget.call.toggleRepoPin {repoId}, (err, res) ->
      if err
        defaultErrorHandler(err)
