{
    nogthrow
} = NogError

NULL_SHA1 = '0000000000000000000000000000000000000000'

ERR_SECRET_CODE_NOT_SET = {
  errorCode: 'ERR_SECRET_CODE_NOT_SET'
  statusCode: 403
  sanitized: 'full'
  reason: 'Secret code has not been set.'
}

ERR_INCORRECT_SECRET_CODE = {
  errorCode: 'ERR_INCORRECT_SECRET_CODE'
  statusCode: 403
  sanitized: 'full'
  reason: 'Entered code is incorrect'
}

correctCodeList = Meteor.settings.secretCodes
if not correctCodeList or correctCodeList.length == 0
  nogthrow ERR_SECRET_CODE_NOT_SET

Meteor.startup ->
  try
    Meteor.users.insert {_id: 'zib', username: 'zib'}
    console.log "[nog-data-box] Created user 'zib'."
  catch err
    # Mongo code 11000 indicates duplicate _id; 'zib' is already available.
    unless err.code == 11000
      throw err


createRepo = (opts) ->
  {ownerName, repoName} = opts
  euid = null  # Anonymous effective user for access checks.
  repoid = NogContent.store.createRepo euid, {
    repoFullName: "#{ownerName}/#{repoName}"
  }
  root = {
    name: "#{repoName} root"
    meta: {}
    entries: []
  }
  tree = NogContent.store.createTree euid, {
    ownerName, repoName, content: root
  }
  commit = NogContent.store.createCommit euid, {
    ownerName, repoName,
    content: {
      subject: "Create repo"
      message: ''
      parents: []
      tree: tree
    }
  }
  NogContent.store.updateRef euid, {
    ownerName, repoName
    refName: 'branches/master'
    old: NULL_SHA1
    new: commit
  }
  return repoid


Meteor.methods
  addDatabox: (opts) ->
    check opts, {code: String}
    if Meteor.isClient
      return

    {code} = opts
    if code not in correctCodeList
      nogthrow ERR_INCORRECT_SECRET_CODE

    ownerName = 'zib'
    repoName = Random.id()
    opts = {ownerName, repoName}
    repoid = createRepo opts

    created = new Date()
    expires = moment(created).add(moment.duration({days: 1})).toDate()
    NogContent.repos.update {
      _id: repoid
    }, {
      $set: {created, expires}
    }

    return {ownerName, repoName}


Meteor.publish 'dataBoxRepo', (opts) ->
  check opts, {
    ownerName: String
    repoName: String
  }
  {ownerName, repoName} = opts
  now = new Date()
  NogContent.repos.find {
    owner: ownerName
    name: repoName
    expires: {$gt: now}
  }, {
    fields: {owner: 1, name: 1, created: 1, expires: 1}
  }

