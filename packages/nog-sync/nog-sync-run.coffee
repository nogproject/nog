class SyncLoop
  constructor: (opts) ->
    { @syncStore, @afterMergeWait, @afterSnapWait } = opts
    @afterMergeWait ?= { min_ms: 2000, max_ms: 5000 }
    @afterSnapWait ?= { min_ms: 200, max_ms: 1000 }
    @pending =
      ping: {}
      fetchPing: {}
      snapshot: {}
      pull: {}
    @isTriggered = false
    @log = console.log
    @log2 = ->

  havePending: ->
    for k, v of @pending
      if not _.isEmpty(v)
        return true
    return false

  defer: (action, name, args...) ->
    @log2 '[sync] scheduling', action, name
    @pending[action][name] = args
    @trigger()

  trigger: ->
    if @isTriggered
      return
    Meteor.defer => @process()
    @isTriggered = true

  process: ->
    while @havePending()
      try
        @processOnce()
      catch err
        console.error(
          '[sync] Unexpected error in process():', err, err.stack
        )
    @isTriggered = false

  processOnce: ->
    start_ms = Date.now()

    for k in _.keys(@pending.ping)
      @log2 '[sync] processing ping', k
      args = @pending.ping[k]
      delete @pending.ping[k]
      @doPing args...

    for k in _.keys(@pending.fetchPing)
      @log2 '[sync] processing fetchPing', k
      args = @pending.fetchPing[k]
      delete @pending.fetchPing[k]
      @doFetchPing args...

    # XXX When using full snapshots, we observed spurious conflicts and
    # spurious repo deletions when placing `doSnapshot()` after `doPull()`.
    # The reason remained unclear.  One hypothesis is that a Mongo fetch does
    # not immediately see a previous update.  An indication is that the
    # spurious conflicts disappeared when we switched to incremental snapshots,
    # which change the snapshot only when they see an increased `mtime` on the
    # relevant repo docs in collection `repos` or collection `deletedRepos`.
    #
    # Leave `doSnapshot()` before `doPull()`, since we did not observe problems
    # with this order, even though other considerations, like the topology of
    # the synchro history, could be arguments for switching the order.

    willPauseAfterSnap = false
    for k in _.keys(@pending.snapshot)
      @log2 '[sync] processing snapshot', k
      args = @pending.snapshot[k]
      delete @pending.snapshot[k]
      { status } = @doSnapshot args...
      if status != 'unchanged'
        willPauseAfterSnap = true

    willPauseAfterMerge = false
    for k in _.keys(@pending.pull)
      @log2 '[sync] processing pull', k
      args = @pending.pull[k]
      delete @pending.pull[k]
      { status, commitSha } = @doPull args...
      if status not in ['up-to-date', 'fast-forward']
        willPauseAfterMerge = true
      if status != 'up-to-date'
        @log '[sync] pull', status, commitSha
      else
        @log2 '[sync] pull', status, commitSha

    duration_ms = Date.now() - start_ms
    if willPauseAfterSnap || willPauseAfterMerge
      @log "[sync] process once took #{(duration_ms / 1000).toFixed(3)}s."

    # Limit snapshot processing rate to avoid high load with frequent updates.

    if willPauseAfterSnap
      w = Math.random()
      wait_ms = (1 - w) * @afterSnapWait.min_ms + w * @afterSnapWait.max_ms
      @log "[sync] sleep #{(wait_ms / 1000).toFixed(3)}s after snap"
      Meteor._sleepForMs wait_ms

    # Limit processing rate if we created a merge commit to allow peers to
    # converge on a common commit.  Imagine two peers that pull from each other
    # as fast as possible.  They would constantly be creating the next merge
    # commit.  The result would be an infinite merge ladder:
    #
    #   o--o--o--o--o ...
    #    \/ \/ \/ \/
    #    /\ /\ /\ /\
    #   o--o--o--o--o ...
    #
    # They should better wait for another peer to merge and then fast-forward.
    # Break this race by giving the other peers a chance to complete and
    # publish their merge.

    # XXX A delay based on the latest duration_ms increases the chance that
    # peers have enough time to fast-forward; out-commented below.  The
    # alternative fixed params did not achieve that.  With quick trivial
    # merges, the fixed params seem to work, however.

    if willPauseAfterMerge
      w = Math.random()
      wait_ms = (1 - w) * @afterMergeWait.min_ms + w * @afterMergeWait.max_ms
      #wait_ms = 5 * w * duration_ms
      @log "[sync] sleep #{(wait_ms / 1000).toFixed(3)}s after merge"
      Meteor._sleepForMs wait_ms

  ping: (euid, opts) ->
    {ownerName, synchroName} = opts
    name = "#{ownerName}/#{synchroName}"
    @defer 'ping', name, euid, opts

  doPing: (euid, opts) ->
    @syncStore.pingSynchro euid, opts

  fetchPing: (euid, opts) ->
    {remote} = opts
    @defer 'fetchPing', remote.name, euid, opts

  doFetchPing: (euid, opts) ->
    {remote} = opts
    remote.fetchPing euid, opts

  snapshot: (euid, opts) ->
    {ownerName, synchroName} = opts
    name = "#{ownerName}/#{synchroName}"
    @defer 'snapshot', name, euid, opts

  doSnapshot: (euid, opts) ->
    {ownerName, synchroName} = opts
    begin_ms = Date.now()
    res = @syncStore.snapshot euid, opts
    end_ms = Date.now()
    if res.status != 'unchanged'
      @log(
        "[sync] snapshot took #{((end_ms - begin_ms) / 1000).toFixed(3)}s."
      )
      @log(
        '[sync] snapshot', res.status,
        'synchro', "#{ownerName}/#{synchroName}", 'commit', res.commit
      )
    return res

  pull: (euid, opts) ->
    {remote} = opts
    @defer 'pull', remote.name, euid, opts

  doPull: (euid, opts) ->
    {remote} = opts
    return remote.pull euid, opts


module.exports.createSyncLoop = (opts) -> new SyncLoop opts
