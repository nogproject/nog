Template.errorDisplay.helpers
  haveErrors: -> Session.get("errors")?
  errors: -> Session.get("errors")
  details: ->
    if @sanitizedError?
      @sanitizedError.details
    else if _.isString @details
      @details
    else
      undefined

Template.errorDisplay.events
  'click .js-clear-errors': (e) ->
    e.preventDefault()
    Session.set("errors", null)
