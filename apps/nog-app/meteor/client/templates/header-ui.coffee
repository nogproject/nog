Template.header.helpers
  backref: ->
    unless (href = NogModal.get 'backref')?
      return null
    {
      title: NogModal.get 'title'
    }

Template.header.events
  'click .js-backref': -> NogModal.end()
