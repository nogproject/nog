{ Meteor } = require 'meteor/meteor'
{ NogError } = require 'meteor/nog-error'
{
  ERR_MIGRATION
  nogthrow
} = NogError

{ NogContent } = require './nog-content.coffee'


NogContent.migrations = {}

NogContent.migrations.addOwnerId = ->
  nErr = 0
  NogContent.repos.find({ownerId: null}).map (repo) ->
    ownerDoc = Meteor.users.findOne {username: repo.owner}, {fields: {_id: 1}}
    if ownerDoc?
      NogContent.repos.update {
          _id: repo._id
        }, {
          $set: {ownerId: ownerDoc._id}
        }
    else
      console.error 'Failed to add ownerId to repo:', repo
      nErr++
  if nErr
    msg = "addOwnerId() failed to migrate #{nErr} documents."
    console.error msg
    nogthrow ERR_MIGRATION, {details: msg}
