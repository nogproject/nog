# `nog-exec` implements a job execution service.
#
# Jobs are managed using the Meteor package `vsivsi:job-collection`.  Each job
# has a job-collection doc `job` with an `_id`.  It also has a nog `jobId`,
# which is created by `nog-flow` and stored in the `job.data.jobId`.
#
# Each job is run only once to completion, failure, or cancellation.  Jobs are
# not restarted.  If a job gets lost, a new job must be started.
#
# A job is processed as follows:
#
#  - `nog-flow` calls `NogExec.submit()` to create a job.  `submit()` creates a
#    scoped access key for the job, so that the job can act as the user who
#    started the job.
#
#  - job-collection promotes the job to ready.
#
#  - A worker daemon `nogexecd` (see `tools/nogexecd/`) is running as user
#    `nogexecbot1` and continuously polls for work.  When the daemon takes
#    work, the job status is changed to `running`.
#
#  - The worker daemon executes `exec-job` to run the job.  `exec-job` runs
#    with the scoped access key, so that it can act on behalf of the user who
#    started the job.  `exec-job` is started as a sub-process for testing and
#    via slurm for production.
#
#  - The program performs the work and reports progress via `POST
#    /jobs/:jobId/status`.
#
#  - `exec-job` reports the job as `completed` or `failed` via `POST
#    /jobs/:jobId/status`, which deletes the scoped access key.
#
# The job status is published to the client, which implements a status UI.
#
# Nog bots use a custom login mechanism (see below) that is based on API keys.
# Admins can manage `nogexecbot*` API keys via the admin UI.
#
# Runtime parameters such as time limits are maintained in the meta field
# `meta.runtime`.  The nog-flow method `addProgram()` initializes `runtime`
# from the base program.  The user can modify `runtime` via the GUI (similar to
# `params`).  `runtime` is copied to the job, so that it is directly available
# during job handling, specifically in `cleanupLostJobs()`
#
# Open questions:
#
#  - Final removal of jobs is not yet implemented.

{
  testAccess
  checkAccess
} = NogAccess

{
  ERR_UNKNOWN
  ERR_PARAM_INVALID
  nogthrow
} = NogError

AA_UPDATE_JOB = 'nog-exec/update-job'
AA_GET_JOB = 'nog-exec/get-job'
AA_SUBS_JOB_STATUS = 'nog-exec/subs-job-status'
AA_EXEC_WORK = 'nog-exec/worker'

config =
  cleanupInterval_s: 5 * 60

optGlobalReadOnly = Meteor.settings.optGlobalReadOnly

# Enable job-collection processing:
#
#  - Ensure that a bot user is present.
#  - Add access statement that allows nogexebots to exec work.
#  - Add access statement that allows admins to manage bot keys.
#  - Start the job server.

ensureBotUser = ->
  sel = {username: 'nogexecbot1'}
  Meteor.users.upsert sel, {$set: sel}
  bot = Meteor.users.findOne(sel)
  Roles.addUsersToRoles bot, ['nogexecbots']


if optGlobalReadOnly
  console.log('[exec] [GRO] Skipping ensure bot user in read-only mode.')
else
  Meteor.startup ensureBotUser

NogAccess.addStatement
  principal: 'role:nogexecbots'
  action: AA_EXEC_WORK
  effect: 'allow'

NogAccess.addStatement
  principal: 'role:admins'
  action: 'nog-auth/apiKey'
  effect: (opts) ->
    {keyOwnerId} = opts
    unless keyOwnerId?
      return 'deny'  # Should not happen.  Better be defensive.
    owner = Meteor.users.findOne opts.keyOwnerId, {fields: {username: 1}}
    unless owner?
      return 'ignore'  # Missing owner will be handled later.
    if owner.username.match /^nog.*bot/
      'allow'
    else
      'ignore'

NogExec.jobs.allow
  worker: (userId, method, params) ->
    aopts = {method, params}
    testAccess userId, AA_EXEC_WORK, aopts

if optGlobalReadOnly
  console.log('[exec] [GRO] Exec job server disabled in read-only mode.')
else
  Meteor.startup ->
    NogExec.jobs.startJobServer()


# Publish the jobs status to job owners.

NogAccess.addStatement
  principal: 'role:users'
  action: AA_SUBS_JOB_STATUS
  effect: 'allow'

Meteor.publish 'jobStatus', (jobIds) ->
  check jobIds, [String]
  if not (user = Meteor.users.findOne @userId)?
    return null
  unless testAccess user, AA_SUBS_JOB_STATUS
    @ready()
    return null
  NogExec.jobs.find {
    'data.jobId': {$in: jobIds}
    'data.ownerId': user._id
  }, {
    fields: {
      'data.jobId': 1, 'data.ownerId': 1, 'data.workspaceRepo': 1,
      'updated': 1, 'created': 1, 'status': 1, 'progress': 1,
      'failures': 1, 'log': 1
    }
  }


# `NogExec.submit()` creates a new job with a scoped access key and submits it
# to the job-collection queue.

@NogExec.submit = (opts) ->
  {jobId, ownerId, ownerName, repoName, commitId, runtime} = opts
  jobdoc = {
    jobId
    ownerId
    workspaceRepo: ownerName + '/' + repoName
    commitId
    runtime
  }
  jobdoc.key = NogAuth.createKeySudo {
    keyOwnerId: ownerId
    comment: """
      Allow job #{jobId} to modify #{jobdoc.workspaceRepo}.
    """
    scopes: [
      {action: 'nog-content/get', opts: {ownerName, repoName}}
      {action: 'nog-content/modify', opts: {ownerName, repoName}}
      {action: 'nog-blob/download', opts: {ownerName, repoName}}
      {action: 'nog-blob/upload', opts: {}}
      {action: AA_UPDATE_JOB, opts: {jobId}}
      {action: 'nog-auth/apiKey', opts: {keyOwnerId: ownerId}}
    ]
  }
  job = new Job NogExec.jobs, 'run', jobdoc
  job.save()


# `job.log()` requires job-collection@1.2.1, so that it works when the job is
# not running.

cleanupLostJobs = ->
  cutoff = new Date()
  cutoff.setSeconds(cutoff.getSeconds() - config.cleanupInterval_s)
  cursor = NogExec.jobs.find {
    status: {$nin: ['completed', 'failed', 'cancelled']}
    updated: {$lt: cutoff}
  }
  cursor.map (jobdoc) ->
    job = new Job NogExec.jobs, jobdoc
    now = new Date()
    {maxTotalDuration_s, maxHeartbeatInterval_s} = jobdoc.data.runtime
    if (now - jobdoc.created) > maxTotalDuration_s * 1000
      job.log "
        Canceled after total running time exceeded #{maxTotalDuration_s}
        seconds (runtime.maxTotalDuration_s)
      "
    else if (now - jobdoc.updated) > maxHeartbeatInterval_s * 1000
      job.log "
        Canceled after lack of progress for more than #{maxHeartbeatInterval_s}
        seconds (runtime.maxHeartbeatInterval_s)
      "
    else
      return
    job.cancel()
    if (keyid = job.data.key?.keyid)?
      NogAuth.deleteKeySudo {
        keyid, keyOwnerId: job.data.ownerId
      }


Meteor.setInterval cleanupLostJobs, config.cleanupInterval_s * 1000


# `nogjobd` uses this special login mechanism to authenticate: any valid signed
# request will be accepted.  The convention is to use `GET /ddplogin`.
#
# Connection tokens cannot be disabled, since Meteor immediately closes
# connections without them.

Accounts.registerLoginHandler 'nogauthv1', (req) ->
  unless (req = req.nogauthreq)?
    return
  NogAuth.checkRequestAuth req
  unless (user = req.auth.user)?
    return
  userId = user._id
  {userId}


# API to jobs and related access statements:
#
#  - `nogexecbots` are allowed to update any job.
#  - Users are allows to query and update their own jobs.
#  - Job collection ids and nog job ids are both accepted.
#  - Progress reporting and status changes are managed through a single POST
#    route.

NogAccess.addStatement
  principal: 'role:nogexecbots'
  action: AA_UPDATE_JOB
  effect: 'allow'

NogAccess.addStatement
  principal: 'role:users'
  action: AA_UPDATE_JOB
  effect: (opts) ->
    unless opts.job? and opts.user?
      return 'ignore'
    if opts.job.data.ownerId == opts.user._id
      return 'allow'
    else
      return 'ignore'

NogAccess.addStatement
  principal: 'role:users'
  action: AA_GET_JOB
  effect: (opts) ->
    unless opts.job? and opts.user?
      return 'ignore'
    if opts.job.data.ownerId == opts.user._id
      return 'allow'
    else
      return 'ignore'


# `matchProgress({completed, total})` requires non-negative values with
# `completed <= total`.

matchProgress = Match.Where (x) ->
  check x, {completed: Number, total: Number}
  unless x.completed >= 0
    throw new Match.Error '`completed` must be non-negative'
  unless x.total >= 0
    throw new Match.Error '`total` must be non-negative'
  unless x.total >= x.completed
    throw new Match.Error '`total` must be greater equal `completed`'
  true


matchLogLevel = Match.Where (x) ->
  check x, String
  levels = ['info', 'success', 'warning', 'danger']
  unless x in levels
    throw new Match.Error 'Invalid log level.'
  true


# `findJob()` accepts either type of job id.  It checks access and returns both
# types of ids together with the job doc.

findJob = (req) ->
  {jobId} = req.params
  if (job = NogExec.jobs.findOne(jobId))?
    execJobId = jobId
    jobId = job.data.jobId
  else if (job = NogExec.jobs.findOne({'data.jobId': jobId}))?
    execJobId = job._id

  aopts = {job, jobId}
  checkAccess req.auth?.user, AA_UPDATE_JOB, aopts
  unless job?
    nogthrow ERR_UNKNOWN, {reason: "Unknown job id #{jobId}."}
  unless job.retried == req.body.retryId
    nogthrow ERR_UNKNOWN, {reason: "`retryId` mismatch."}

  return {jobId, execJobId, job}


post_job_status = (req) ->
  check req.params, Match.ObjectIncluding {jobId: String}
  check req.body,
    retryId: Number
    status: String
    reason: Match.Optional String

  {job} = findJob req

  unless job.status == 'running'
    nogthrow ERR_PARAM_INVALID, {reason: 'Job is not running.'}

  qjob = new Job(NogExec.jobs, job)
  {status, progress, reason} = req.body

  switch status
    when 'completed'
      qjob.done()
    when 'failed'
      reason ?= 'Unknown reason.'
      qjob.fail({reason})
    else
      nogthrow ERR_PARAM_INVALID, {
        reason: "Unknown status value `#{status}`."
      }
  if (keyid = qjob.data.key?.keyid)?
    NogAuth.deleteKey req.auth?.user, {
      keyid, keyOwnerId: qjob.data.ownerId
    }

  return {}


post_job_progress = (req) ->
  check req.params, Match.ObjectIncluding {jobId: String}
  check req.body,
    retryId: Number
    progress: matchProgress

  {job} = findJob req

  unless job.status == 'running'
    nogthrow ERR_PARAM_INVALID, {reason: 'Job is not running.'}

  qjob = new Job(NogExec.jobs, job)
  {progress} = req.body

  qjob.progress(progress.completed, progress.total)

  return {}


post_job_log = (req) ->
  check req.params, Match.ObjectIncluding {jobId: String}
  check req.body,
    retryId: Number
    message: String
    level: Match.Optional matchLogLevel

  {job} = findJob req
  qjob = new Job(NogExec.jobs, job)

  {message, level} = req.body
  level ?= 'info'
  qjob.log message, {level}

  return {}


# `get_job_status()` does not use `findJob()`, because `retryId` is not
# available.  Instead, it returns status/progress of the job with the greatest
# `retryId`.

get_job_status = (req) ->
  check req.params, Match.ObjectIncluding {jobId: String}

  {jobId} = req.params
  job = NogExec.jobs.findOne {'data.jobId': jobId}, {'sort': {'retried': -1}}
  unless job?
    nogthrow ERR_UNKNOWN, {reason: "Unknown job id #{jobId}."}

  aopts = {job, jobId}
  checkAccess req.auth?.user, AA_GET_JOB, aopts
  status = job.status
  return {status}


get_job_progress = (req) ->
  check req.params, Match.ObjectIncluding {jobId: String}

  {jobId} = req.params
  job = NogExec.jobs.findOne {'data.jobId': jobId}, {'sort': {'retried': -1}}
  unless job?
    nogthrow ERR_UNKNOWN, {reason: "Unknown job id #{jobId}."}

  aopts = {job, jobId}
  checkAccess req.auth?.user, AA_GET_JOB, aopts
  progress = job.progress
  return {progress}


actions = [
  {
    method: 'POST'
    path: '/:jobId/status'
    action: post_job_status
  }
  {
    method: 'POST'
    path: '/:jobId/progress'
    action: post_job_progress
  }
  {
    method: 'POST'
    path: '/:jobId/log'
    action: post_job_log
  }
  {
    method: 'GET'
    path: '/:jobId/status'
    action: get_job_status
  }
  {
    method: 'GET'
    path: '/:jobId/progress'
    action: get_job_progress
  }
]
NogRest.actions '/api/jobs', actions
NogRest.actions '/api/v0/jobs', actions
NogRest.actions '/api/v1/jobs', actions
