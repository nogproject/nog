# XXX The template helpers to test access are missing, because the package
# `nog-access` is not part of `nog-data-box-app`.  Similarly to the other
# access checks, the template helpers should probably be injected, too.  As a
# quicker workaround, define global replacements here.
if Meteor.isClient
  Template.registerHelper 'testAccess', -> true
  Template.registerHelper 'testAccess_ready', -> true


FlowRouter.route '/',
  action: ->
    BlazeLayout.render 'layout', {main: 'nogDataBoxStart'}


FlowRouter.route '/:ownerName/:repoName/files/:treePath*',
  name: 'files'
  action: () ->
    BlazeLayout.render 'layout', {main: 'nogDataBox'}


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
