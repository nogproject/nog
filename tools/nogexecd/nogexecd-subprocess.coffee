# It is started as a worker by `nogjobd`.

_ = require 'underscore'
{spawn} = require 'child_process'

python = './virtualenv/bin/python'

maxNumJobs = 1
numJobsRunning = 0

processJob = (job) ->
  console.log 'starting job', job.data.jobId

  args = ['exec-job', '--params']
  params = _.pick(
      job.data, 'jobId', 'workspaceRepo', 'commitId'
    )
  params.nogexec = {
    execJobId: job.doc._id
    retryId: job.doc.retried
  }
  args.push JSON.stringify params

  console.log 'spawn', python, args
  env = _.pick(
      process.env, 'PATH', 'NOG_CACHE_PATH', 'NOG_API_URL', 'NOG_MAX_RETRIES'
    )
  env['NOG_KEYID'] = job.data.key.keyid
  env['NOG_SECRETKEY'] = job.data.key.secretkey
  child = spawn python, args, {stdio: 'inherit', env}
  numJobsRunning++
  child.on 'close', (code) ->
    numJobsRunning--
    console.log 'Child completed with exit code', code

  job.log 'Started processing with subprocess exec-job.', {level: 'info'}

  # Do not mark the job as done.  It is the responsibility of `exec-job`.


module.exports =
  root: 'nogexec.jobs'
  type: 'run'
  processJob: processJob
  maxJobs: -> maxNumJobs - numJobsRunning
