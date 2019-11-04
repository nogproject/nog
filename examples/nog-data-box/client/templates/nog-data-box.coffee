interval = 60000

formatDate = (date) ->
  moment(date).format('MMMM Do YYYY,  hh:mm:ss a')

{defaultErrorHandler} = NogError


Template.nogDataBoxStart.events
  'submit .js-create-repo': (ev) ->
    ev.preventDefault()
    code = ev.target.secretCodeInput.value
    opts = {code}
    Meteor.call 'addDatabox', opts, (err, res) ->
      if err?
        FlowRouter.go('/')
        return defaultErrorHandler(err)
      FlowRouter.go('/' + res.ownerName + '/' + res.repoName + '/files')


Template.nogDataBox.onCreated ->
  @autorun =>
    ownerName = FlowRouter.getParam('ownerName')
    repoName = FlowRouter.getParam('repoName')
    @subscribe 'dataBoxRepo', {ownerName, repoName}


Template.nogDataBoxContent.helpers
  repoExists: ->
    owner = FlowRouter.getParam('ownerName')
    name = FlowRouter.getParam('repoName')
    (NogContent.repos.findOne {owner, name})?

  repo: ->
    owner = FlowRouter.getParam('ownerName')
    name = FlowRouter.getParam('repoName')
    repo = NogContent.repos.findOne {owner, name}
    TimeTracker.changeIn(interval)
    now = moment()
    return {
      created: formatDate(repo.created)
      expires: formatDate(repo.expires)
      href: FlowRouter.url('/' + repo.owner + '/' + repo.name + '/files')
      isExpired: moment(repo.expires).isBefore(now)
    }
