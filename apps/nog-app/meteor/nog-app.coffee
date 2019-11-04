import { initRouterV2 } from '/imports/ui-v2/nog-app-v2.js'
import { createFsoModuleClient } from 'meteor/nog-fso'
import { createSuggestModuleClient } from 'meteor/nog-suggest'
import { NsFso, NsSuggest } from '/imports/namespace.js'

if Meteor.isServer
  # See `./server/rest-server.js` for more.

  NogRest.actions '/api/repos',
    NogContent.api.repos.actions_v1()
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.blobs.actions()
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.uploads.actions()

  NogRest.actions '/api/v1/repos',
    NogContent.api.repos.actions_v1()
  NogRest.actions '/api/v1/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.blobs.actions()
  NogRest.actions '/api/v1/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.uploads.actions()


if Meteor.isServer
  Meteor.publish 'repos', ->
    if not (user = Meteor.users.findOne @userId)?
      return null
    if not NogAccess.testAccess(
        user, 'nog-content/get', {ownerName: user.username})
      @ready()
      return null
    NogContent.repos.find {owner: user.username}

  # Use package `peerlibrary:reactive-publish` to track updates of the circle
  # information.  <https://github.com/peerlibrary/meteor-reactive-publish>
  #
  # Do not use `reactivePublish` from package `lepozepo:reactive-publish`,
  # which is mentioned in
  # <https://www.discovermeteor.com/blog/reactive-joins-in-meteor/>, since it
  # caused problems with Meteor 1.2; see
  # <https://forums.meteor.com/t/cant-load-any-pages-on-meteor-1-2/10203>.
  Meteor.publish 'sharedRepos', ->
    @autorun =>
      if not @userId?
        return null
      user = Meteor.users.findOne @userId
      if not NogAccess.testAccess user, 'nog-content/get', {style: 'loose'}
        @ready()
        return null
      inCircles = user.sharing?.inCircles ? []
      fromUserIds = _.unique _.pluck inCircles, 'fromId'
      circleIds = _.pluck inCircles, 'circleId'
      sel =
        owner: {$ne: user.username}
        $or: [
          {'sharing.public': true}
          {'sharing.allCircles': true, ownerId: {$in: fromUserIds}}
          {'sharing.circles': {$in: circleIds}}
        ]
      NogContent.repos.find sel
    return


if Meteor.isClient
  NogFso = createFsoModuleClient({
    namespace: NsFso,
    testAccess: NogAccess.testAccess,
    subscriber: Meteor,
  })

  NogSuggest = createSuggestModuleClient({
    namespace: NsSuggest,
  })

  # `NogHome` combines the home links subscription from NogFso into a separate
  # object that could later be refactored into a separate module.
  NogHome = {
    subscribeHome: NogFso.subscribeHome,
    homeLinks: NogFso.homeLinks,
  }

  # Useful for debugging.
  window.NogFso = NogFso
  window.NogSuggest = NogSuggest

  initRouterV2({
    user: Meteor.user.bind(Meteor),
    router: FlowRouter,
    nogFso: NogFso,
    nogSuggest: NogSuggest,
    nogHome: NogHome,
    nogCatalog: NogCatalog,
    optShowVersions: !!Meteor.settings.public.optShowVersions,
    versions: Meteor.settings.public.versions || {},
  })


FlowRouter.route '/',
  action: ->
    BlazeLayout.render 'layout', {main: 'home'}


FlowRouter.route '/help',
  name: 'help'
  action: (params) ->
    BlazeLayout.render 'layout', {main: 'help'}



FlowRouter.route '/:ownerName/:repoName/settings',
  name: 'repoSettings'
  action: (params) ->
    BlazeLayout.render 'layout', {main: 'repoSettings'}


FlowRouter.route '/:ownerName/:repoName/tree/:refTreePath+',
  name: 'repoTree'
  action: (params) ->
    BlazeLayout.render 'layout', {main: 'tree'}


FlowRouter.route '/:ownerName/:repoName/object/:refTreePath+',
  name: 'repoObject'
  action: (params) ->
    BlazeLayout.render 'layout', {main: 'tree'}


FlowRouter.route '/:ownerName/:repoName/files/:treePath*',
  name: 'files'
  action: () ->
    BlazeLayout.render 'layout', {main: 'nogFiles'}


FlowRouter.route '/:ownerName/:repoName/workspace',
  name: 'workspace'
  action: ->
    BlazeLayout.render 'layout', {main: 'workspace'}


if Meteor.settings.public.optCatalog isnt 'disabled'
  FlowRouter.route '/:ownerName/:repoName/catalog',
    name: 'catalog'
    action: () ->
      BlazeLayout.render 'layout', { main: 'nogCatalogDiscoverGate' }


FlowRouter.route '/search',
  name: 'search'
  action: (params) ->
    BlazeLayout.render 'layout', {main: 'search'}


FlowRouter.route '/new',
  name: 'createRepo'
  action: (params) ->
    BlazeLayout.render 'layout', {main: 'createRepo'}


FlowRouter.route '/denied',
  name: 'denied'
  action: ->
    BlazeLayout.render 'layout', {main: 'denied'}


FlowRouter.route '/admin',
  name: 'admin'
  action: ->
    BlazeLayout.render 'layout', {main: 'accountsAdmin'}


FlowRouter.route '/settings',
  name: 'setting'
  action: ->
    BlazeLayout.render 'layout', {main: 'settings'}


# Patch `FlowRouter.path()` to undo flow-router's strange urlencoding (see
# commit [750321]) to get real slashes.
#
# [750321]: flow-router@750321ca9f76a52dacf8b8db7df80f44eb295378 'Encoding
# params 2times to fix #168'

orig_FlowRouter_path = FlowRouter.path

FlowRouter.path = () ->
  href = orig_FlowRouter_path.apply(FlowRouter, arguments)
  href = href.replace /%252F/g, '/' # for flow-router >= v1.17.1
  href = href.replace /%2F/g, '/'  # for flow-router <= v1.15.0, with our patch
  href
