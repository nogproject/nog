#!/usr/bin/env coffee

# I failed to reliably handle errors by reconnecting (see spikes LOG entry).
#
# The solution is to die on errors and use an internal heartbeat monitoring
# (see below) to detect if the daemon got stuck; and if so, die, too.  The
# parent `nogjobd-forever` watchdog will restart the daemon.

DDP = require 'ddp'
async = require 'async'
crypto = require 'crypto'
url = require 'url'
JobQueue = require 'meteor-job'

# `config` is read from the evironment variable `NOGJOBD_CONFIG`, which can
# either be a JSON string or the name of a JSON file.
config = process.env.NOGJOBD_CONFIG
if not config?
  config = {}
else if config[0] is '{'
  config = JSON.parse config
else
  config = require config

config.NOG_URL ?= 'http://localhost:3000'
config.NOG_KEYID ?= process.env.NOG_KEYID
config.NOG_SECRETKEY ?= process.env.NOG_SECRETKEY
config.pollInverval_s ?= 5
config.heartbeatTimeout_s ?= 15
config.worker ?= './nogexecd-subprocess'

# Load worker from a separate module.
console.log 'worker', config.worker
worker = require config.worker

u = url.parse config.NOG_URL
ddp = new DDP
  host: u.hostname
  port: u.port
  ssl: u.protocol is 'https:'
  autoReconnect: true
  autoReconnectTimer: 10

JobQueue.setDDP ddp


die = (args...) ->
  console.error 'Error:', args...
  process.exit(1)


connect = ->
  ddp.connect (err, isReconnect) ->
    if err
      die 'connect failed.', err
    if isReconnect
      die 'reconnect.', err
    console.log 'connected'
    ddplogin()


# The DDP connection is authenticated with a dummy HTTP request that is
# verified using NogAuth.
ddplogin = () ->
  key = {keyid: config.NOG_KEYID, secretkey: config.NOG_SECRETKEY}
  req = {method: 'GET', url: '/ddplogin'}
  signRequest key, req
  ddp.call 'login', [{nogauthreq: req}], (err, res) ->
    if err
      die 'auth failed.', err
    console.log(
        'logged in as user id', res.id, ';'
        'token expires', res.tokenExpires
      )
    startProcessing()


# Encode without ':' and strip milliseconds, since they are irrelevant.
toISOStringUrlsafe = (date) -> date.toISOString().replace(/:|\.[^Z]*/g, '')


signRequest = (key, req) ->
  authalgorithm = 'nog-v1'
  authkeyid = key.keyid
  now = new Date()
  authdate = toISOStringUrlsafe(now)
  authexpires = 600
  authnonce = crypto.randomBytes(10).toString('hex')
  req.url += '?'
  req.url += "authalgorithm=#{authalgorithm}"
  req.url += '&' + "authkeyid=#{authkeyid}"
  req.url += '&' + "authdate=#{authdate}"
  req.url += '&' + "authexpires=#{authexpires}"
  req.url += '&' + "authnonce=#{authnonce}"
  stringToSign = req.method + "\n" + req.url + "\n"
  hmac = crypto.createHmac 'sha256', key.secretkey
  hmac.update stringToSign
  authsignature = hmac.digest 'hex'
  req.url += '&' + "authsignature=#{authsignature}"


# Poll for work and monitor heartbeat.  Before actually requesting work from
# the queue, ask the worker whether it is willing to accept work.
startProcessing = ->
  heartbeat = new Date()
  beat = -> heartbeat = new Date()

  getWorkIsPending = false

  pollOnce = ->
    if getWorkIsPending
      return
    if worker.maxJobs() < 1
      beat()
      return
    getWorkIsPending = true
    JobQueue.getWork worker.root, worker.type, (err, job) ->
      if err
        die 'getWork() failed.', err
      getWorkIsPending = false
      beat()
      unless job?
        return
      worker.processJob job
      # If there was work, immediately poll for more.
      setTimeout pollOnce, 0

  checkHeartbeat = ->
    now = new Date()
    timeout = config.heartbeatTimeout_s
    if now - heartbeat > timeout * 1000
      die "Missing heartbeat for more than #{timeout} seconds."

  interval = config.pollInverval_s
  console.log 'polling for work every', interval, 'seconds'
  interval *= 1000
  setInterval pollOnce, interval
  setInterval checkHeartbeat, interval


connect()

