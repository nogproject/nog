# It is started as a worker by `nogjobd`.

_ = require 'underscore'
{exec} = require 'child_process'
{
  mkdirSync, existsSync
} = require 'fs'

config =
  timeout_s:
    sbatch: 20
  appid: 'nogexecd'
  maxNumJobsSubmitting: 8
  defaultMaxTotalDuration_s: 3000
  defaultMaxMem_MB: 8 * 1024
  partitions:
    short: {account: 'short', partition: 'short', nodelist: 'vsl4'}
    vsl4long: {account: 'long', partition: 'vsl4long'}
    nice: {account: 'nice', partition: 'nice', nodelist: 'vsl4'}


python = process.cwd() + '/virtualenv/bin/python'
exec_job = process.cwd() + '/exec-job'

numJobsSubmitting = 0

# Create jobdir and use submit a script to slurm by running `sbatch` inside the
# jobdir, so that the logs appear there.
processJob = (job) ->
  jobId = job.data.jobId
  console.log 'Submitting job to slurm:', jobId

  jobsdir = process.env.NOG_CACHE_PATH + '/jobs'
  unless existsSync jobsdir
    mkdirSync jobsdir
  jobdir = jobsdir + '/' + jobId
  unless existsSync jobdir
    mkdirSync jobdir

  params = _.pick(
      job.data, 'jobId', 'workspaceRepo', 'commitId'
    )
  params.nogexec = {
    execJobId: job.doc._id
    retryId: job.doc.retried
  }
  params = JSON.stringify params
  params = escapeShellQuoted params
  shenv = _.pick(
      process.env, 'NOG_CACHE_PATH', 'NOG_API_URL', 'NOG_MAX_RETRIES'
    )
  shenv['NOG_KEYID'] = job.data.key.keyid
  shenv['NOG_SECRETKEY'] = job.data.key.secretkey
  script = '' +
    """
    #!/bin/bash

    echo 'starting exec-job'

    """ +
    ("export #{k}='#{v}'" for k, v of shenv).join('\n') + '\n' +
    """

    #{python} #{exec_job} --params '#{params}'

    """

  parseSbatch = (err, stdout, stderr) ->
    numJobsSubmitting--
    if err
      job.fail({reason: 'Failed to sbatch to slurm: ' + err.message})
      return
    if not (m = stdout.match /Submitted batch job ([0-9]+)/)?
      job.fail({reason: 'Failed to parse sbatch output:\n' + stdout})
      return
    slurmid = m[1]
    console.log 'Submitted as slurm job', slurmid
    job.log "Submitted as slurm job #{slurmid}", {level: 'info'}
    # Do not mark the job as done.  It is the responsibility of `exec-job`.

  # Propagate limits to slurm (see `man sbatch`).
  if not (d = job.data.runtime?.maxTotalDuration_s)?
    d = config.defaultMaxTotalDuration_s
  sTime_min = Math.ceil(d / 60)

  if not (sMem_MB = job.data.runtime?.maxMem_MB)?
    sMem_MB = config.defaultMaxMem_MB

  unless (p = job.data.runtime?.partition)?
    p = 'short'
  unless config.partitions[p]?
    p = 'short'
  slurmArgs = for k, v of config.partitions[p]
    "--#{k}=#{v}"

  cmd = [
    'sbatch',
    "--comment=#{config.appid}/jobId/#{jobId}",
    "--time=#{sTime_min}",
    "--mem=#{sMem_MB}"
  ].concat(
    slurmArgs
  ).join(' ')

  console.log 'executing:', cmd

  numJobsSubmitting++
  p = exec(
    cmd,
    {
      env: _.pick(process.env, 'PATH')
      cwd: jobdir
      timeout: config.timeout_s.sbatch * 1000
    },
    parseSbatch
  )
  p.stdin.write script
  p.stdin.end()


# `escapeShellQuoted()` escapes a string for passing it through the shell with
# single quotes; like "program '<string>'".
escapeShellQuoted = (s) ->
  s.replace "'", "'\"'\"'"


module.exports =
  root: 'nogexec.jobs'
  type: 'run'
  processJob: processJob
  maxJobs: -> config.maxNumJobsSubmitting - numJobsSubmitting
