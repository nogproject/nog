Meteor.subscribe 'repos'

Template.repos.helpers
  repos: -> NogContent.repos.find()

Template.repos.events
  'click .js-create-repo': (ev) ->
    ev.preventDefault()
    owner = Meteor.user()?.username ? 'anonymous'
    opts =
      repoFullName: [owner, Random.id()].join '/'
    NogContent.call.createRepo opts, (err, res) ->
      if err
        console.log err
        return alert err
      console.log 'createRepo ok:', res
