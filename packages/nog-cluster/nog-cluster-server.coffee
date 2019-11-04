# `NogCluster.IdPartition` together with `NogCluster.registerHeartbeat()`
# implements a basic work distribution scheme for a cluster of application
# instances.
#
# Each server chooses a random id `self` during startup and registers itself
# into a TTL collection `nogcluster.members` with a regular `heartbeat()`.
#
# Other parts of the application can instantiate `IdPartition`.  `NogCluster`
# will try to acquire the responsibility for id partition parts and regularly
# renew them using the TTL collection `leases`.  It calls `onacquire` when it
# acquired a part and `onrelease` when it released a part.  The callbacks can
# start and stop background maintenance tasks for the part, such as updating
# the search index.
#
# The current scheme assigns each part to a single app instance.  Each instance
# computes the number of parts that it wants to acquired based on the number of
# available parts and the number of cluster members.  The scheme uses
# overallocation: several app instances compete for the parts.  It could later
# be extended to allow multiple instances to acquire the same part to implement
# redundant processing for failure scenarios.  The allocation scheme keeps
# allocations fixed unless the number of cluster members changes substantially,
# so that a large amount of startup work in `onacquire` should be acceptable,
# since it will be amortized over time.
#
# The background tasks must not assume that they have exclusive responsibility
# for a part.  They also should not make assumption about when `onacquire` and
# `onrelease` are called.  `onacquire` should probably do a full up-to-date
# check or schedule a full up-to-date check at regular intervals to ensure
# eventual consistency.

# Many parameters of the allocation scheme are hard-coded.  We will
# incrementally expose more parameters in the settings if needed to handle
# deployment and testing.
#
# `optSingleInstanceMode` controls whether an instance immediately takes
# responsibility for all updates, which is recommended for testing but not for
# production deployments with multiple app instances.
#
# The fields of `settings.cluster.maxIdPartitions` control the number of
# partitions for individual tasks, such as `updateKinds` and `searchIndex`; see
# other source files.
#
# The first cluster heartbeat runs after `firstHeartbeat_ms`, which should be
# small to quickly get a fully functional app for testing.
#
# Regular heartbeats run at intervals of `heartbeat_s`. `ttl_s` controls the
# lease time in MongoDB.  `ttl_s` should be a few times longer than
# `heartbeat_s` to ensure stable lease assignment.

NogCluster =
  self: Random.id()
  size: 0
  watchers: []
  registerHeartbeat: (w) -> @watchers.push w

config =
  ttl_s: 30
  heartbeat_s: 10
  firstHeartbeat_ms: 100
  optSingleInstanceMode: Meteor.settings.cluster.optSingleInstanceMode

optGlobalReadOnly = Meteor.settings.optGlobalReadOnly


members = new Mongo.Collection 'nogcluster.members'
members._ensureIndex {
  heartbeat: 1
}, {
  expireAfterSeconds: config.ttl_s
}

leases = new Mongo.Collection 'nogcluster.leases'
leases._ensureIndex {
  heartbeat: 1
}, {
  expireAfterSeconds: config.ttl_s
}


idAlphabet = do ->
  numbers = '0123456789'
  lower = 'abcdefghijklmnopqrstuvwxyz'
  upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  numbers + upper + lower


idPartitions = (max) ->
  max = Math.min(idAlphabet.length, max)
  parts = []
  part = {}
  partSize = Math.ceil idAlphabet.length / max
  for x in idAlphabet by partSize
    if part.begin?
      part.sel = {$gte: part.begin, $lt: x}
      part.selHuman = "`#{part.begin}` <= id < `#{x}`"
      parts.push _.clone(part)
    part.begin = x
  part.sel = {$gte: part.begin}
  part.selHuman = "`#{part.begin}` <= id"
  parts.push part
  parts


# See <http://stackoverflow.com/q/1985260>.
rotate = (arr, n) ->
  arr.unshift.apply(arr, arr.splice(n, arr.length))
  arr


class IdPartition
  constructor: (opts) ->
    @name = opts.name
    @overacquireFactor = 2

    @donthave = idPartitions(opts.max)
    rotate @donthave, Math.floor(Math.random() * @donthave.length)
    @nParts = @donthave.length
    @acquired = {}

    @onacquire = ->
    @onrelease = ->

  nAcquired: -> _.keys(@acquired).length

  heartbeat: ->
    wantMin = Math.min(
      @nParts,
      Math.ceil(@overacquireFactor * @nParts / Math.max(1, NogCluster.size))
    )
    wantMax = Math.min(@nParts, 2 * wantMin)
    nOld = @nAcquired()

    if @nAcquired() < wantMin
      @tryAcquire()
    else if @nAcquired() > wantMax
      @releaseOne()
    @confirm()

    if @nAcquired() != nOld
      console.log "
        [nog-cluster] Member #{NogCluster.self} now holds #{@nAcquired()} of
        #{@nParts} leases of `#{@name}`; target: #{wantMin} to #{wantMax}.
      "

  tryAcquire: ->
    self = NogCluster.self
    if @donthave.length == 0
      return
    part = @donthave.shift()
    id = @name + '.' + part.begin
    if config.optSingleInstanceMode
      if leases.remove({_id: id, owner: {$ne: self}}) > 0
        console.log(
          "[nog-cluster] Force acquire lease `#{id}` in single instance mode."
        )
    try
      leases.insert {_id: id, heartbeat: new Date(), owner: self}
    catch err
      @donthave.push(part)
      # Mongo code 11000 indicates duplicate _id.
      unless err.code == 11000
        throw err
      console.log "[nog-cluster] Member #{self} did not acquire lease `#{id}`."
      return
    console.log "[nog-cluster] Member #{self} acquired lease `#{id}`."
    @acquired[part.begin] = part
    @onacquire part

  releaseOne: ->
    unless @nAcquired() > 0
      return
    part = _.values(@acquired)[0]
    id = @name + '.' + part.begin
    self = NogCluster.self
    leases.remove {_id: id, owner: self}
    console.log "[nog-cluster] Member #{self} released lease `#{id}`."
    @releasePart part

  confirm: ->
    self = NogCluster.self
    for begin, part of @acquired
      id = @name + '.' + part.begin
      nup = leases.update {
        _id: id
        owner: self
      }, {
        $currentDate: {heartbeat: true}
      }
      unless nup == 1
        console.log "[nog-cluster] Member #{self} lost lease `#{id}`."
        @releasePart part

  releasePart: (part) ->
    delete @acquired[part.begin]
    @donthave.push part
    @onrelease part


# Don't wait for Mongo TTL to delete doc, but select by cutoff to calculate
# cluster size to achieve a more predictable failover time.

heartbeat = ->
  members.upsert {
    _id: NogCluster.self
  }, {
    $currentDate: {heartbeat: true}
  }

  cutoff = new Date()
  cutoff.setSeconds(cutoff.getSeconds() - config.ttl_s)
  sel = {heartbeat: {$gt: cutoff}}
  NogCluster.size = members.find(sel).count()

  for w in NogCluster.watchers
    w.heartbeat()


foreverHeartbeat = ->
  nextHeartbeat = ->
    # Handle all errors to ensure that the next call is scheduled even if
    # `heartbeat()` throws an unexpected error.
    try
      heartbeat()
    catch err
      console.error(
        '[nog-cluster] Unexpected error in `heartbeat()`.', err.stack
      )
    Meteor.setTimeout nextHeartbeat, config.heartbeat_s * 1000

  Meteor.setTimeout nextHeartbeat, config.firstHeartbeat_ms

if optGlobalReadOnly
  console.log(
    '[cluster] [GRO] Disabling cluster heartbeats in read-only mode.'
  )
else
  Meteor.startup foreverHeartbeat


NogCluster.IdPartition = IdPartition

module.exports.NogCluster = NogCluster
