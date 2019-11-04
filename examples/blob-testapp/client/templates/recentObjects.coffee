Meteor.subscribe 'objects'

Template.recentObjects.helpers
  objects: ->
    Objects.find {}, {sort: {createDate: -1}}
