{ createNogSyncMain } = require './nog-sync-main.coffee'


defaultAccess = ->
  if (p = Package['nog-access'])?
    console.log '[sync] using nog-access default policy.'
    checkAccess = p.NogAccess.checkAccess
    testAccess = p.NogAccess.testAccess
  else
    console.log '
      [sync] default access control disabled, since nog-access is not
      available.
    '
    checkAccess = ->
    testAccess = -> true
  return {checkAccess, testAccess}


createDefaultMain = ->
  settings = Meteor.settings
  unless settings.sync?
    console.log '[sync] no default main due to missing `settings.sync`.'
    return null
  namespace = {
    meth: 'NogSync'
    coll: 'sync'
  }
  {checkAccess, testAccess} = defaultAccess()
  return createNogSyncMain {
    namespace
    settings
    checkAccess
    testAccess
    contentStore: NogContent.store
  }


NogSyncMain = createDefaultMain()

optNogSyncAutostart = Meteor.settings.optNogSyncAutostart ? false
if optNogSyncAutostart
  NogSyncMain.ensureSyncUsers()
  username = NogSyncMain.config.ourUsername()
  euid = Meteor.users.findOne({ username })
  console.log(
    "[sync] Starting main synchro and remotes as user `#{username}`, " +
    "id `#{euid?._id}`."
  )
  NogSyncMain.ensureMainSynchro(euid)
  NogSyncMain.ensureMainSynchroSnapshot(euid)
  NogSyncMain.startMainSynchro(euid)
  NogSyncMain.connectRemotes(euid)
  console.log('[sync] Main synchro startup done.')


module.exports.createNogSyncMain = createNogSyncMain
module.exports.NogSyncMain = NogSyncMain
