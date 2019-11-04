{ createNogSyncConfig } = require './nog-sync-config.js'
{ createSyncStore } = require './nog-sync-store.coffee'
{ createSyncLoop } = require './nog-sync-run.coffee'
{
  connectRemote,
  publishNogSync,
  defNogSyncMethods,
} = require './nog-sync-remote.coffee'


# `ensureSyncUsers()` creates `nogsyncbot*` users as specified in the config.
# The local sync bot user may create and modify synchros.  The other peers may
# only get content.
#
# `nog.*bot.*` accounts are protected for internal use by nog-app
# `isBlacklisted()`.


matchSyncUsername = Match.Where (x) ->
  check x, String
  if not (x.match /// ^ nogsyncbot[a-z0-9]* $ ///)?
    throw new Match.Error 'Invalid sync username.'
  true


ensureSyncUser = (username, opts) ->
  check username, matchSyncUsername
  opts ?= {}
  sel = {username}
  Meteor.users.upsert sel, {$set: sel}
  bot = Meteor.users.findOne(sel)
  Roles.addUsersToRoles bot, ['nogsyncbots']
  if opts.local
    Roles.addUsersToRoles bot, ['noglocalsyncbots']


# `startSynchro()` starts monitoring for events and triggers synchro snapshots
# through the `syncLoop`.  It returns a `monitor`.  `monitor.ping({token})`
# sends a test event.
#
# Snapshots are triggered by changes of master in the observed repos and at
# regular intervals as a fallback if observe events get lost.

startSynchro = (euid, opts) ->
  {ownerName, synchroName, syncLoop, interval_ms} = opts

  monitor = {}

  snapfn = ->
    if (token = monitor._token)?
      delete monitor._token
      syncLoop.ping euid, {
        ownerName, synchroName, token
      }
    syncLoop.snapshot euid, {ownerName, synchroName}

  monitor.ping = (opts) ->
    {token} = opts
    @_token = token

  monitor.interval = Meteor.setInterval snapfn, opts.interval_ms

  contentStore = syncLoop.syncStore.contentStore
  monitor.observer = contentStore.repos.find(
    {},
    { fields: { 'refs.branches/master' } },
  ).observeChanges({
    added: snapfn,
    changed: snapfn,
    removed: snapfn,
  })

  monitor.stop = ->
    @observer.stop()
    Meteor.clearInterval @interval

  return monitor



createNogSyncMain = (opts) ->
  {namespace, settings, contentStore, checkAccess, testAccess} = opts

  NogSyncMain = {}

  NogSyncMain.config = config = createNogSyncConfig {settings}

  NogSyncMain.ensureSyncUsers = ->
    for peer in @config.peers
      ensureSyncUser @config.peerUsername(peer)
    ensureSyncUser @config.ourUsername(), { local: true }

  NogSyncMain.syncStore = syncStore = createSyncStore {
    namespace
    ourPeerName: config.ourPeerName()
    contentStore
    users: Meteor.users
    checkAccess
    testAccess
    caching: config.caching
  }

  publishNogSync {namespace, syncStore}
  defNogSyncMethods {namespace, syncStore}

  NogSyncMain.syncLoop = createSyncLoop {
    syncStore
    afterMergeWait: config.afterMergeWait
    afterSnapWait: config.afterSnapWait
  }

  NogSyncMain.ensureMainSynchro = (euid) ->
    ownerName = @config.ourUsername()
    synchroName = 'all'
    @syncStore.ensureSynchro euid, {ownerName, synchroName}

  NogSyncMain.ensureMainSynchroSnapshot = (euid) ->
    # XXX This should probably be replaced: take a snapshot only conditionally
    # if there is none.
    #
    # XXX A full snapshot is forced at startup as a fallback to protect against
    # inconsistencies in the mtimes, such as repos that were manually deleted
    # from MongoDB without inserting them in `deletedRepos`.  The full snapshot
    # directly after startup should probably be replaced by something smarter,
    # like background re-checking, in order to reduce startup load.
    ownerName = @config.ourUsername()
    synchroName = 'all'
    @syncStore.fullSnapshot euid, { ownerName, synchroName }

  NogSyncMain.startMainSynchro = (euid) ->
    ownerName = @config.ourUsername()
    synchroName = 'all'
    @mainMonitor = startSynchro euid, {
      ownerName, synchroName
      syncLoop: @syncLoop
      interval_ms: @config.interval_ms
    }

  NogSyncMain.stopMainSynchro = (euid) ->
    @mainMonitor.stop()

  NogSyncMain.pingMainSynchro = (euid, opts) ->
    {token} = opts
    @mainMonitor.ping {token}

  NogSyncMain.connectRemotes = (euid) ->
    syncLoop = @syncLoop
    @remotes = {}
    ourUsername = @config.ourUsername()
    fallbackPullInterval_ms = config.fallbackPullInterval_ms
    for config in @config.remotes
      if not config.namespace?
        config = _.clone config
        config.namespace = namespace
      r = connectRemote euid, {
        config, syncLoop, syncStore, ourUsername, fallbackPullInterval_ms
      }
      @remotes[r.name] = r

  NogSyncMain.disconnectRemotes = (euid) ->
    for k, remote of @remotes
      remote.disconnect()
    @remotes = {}

  return NogSyncMain


module.exports.createNogSyncMain = createNogSyncMain
