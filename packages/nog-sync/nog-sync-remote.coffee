{
  signRequest
} = NogAuth

{ moment } = require 'meteor/momentjs:moment'

PriorityQueue = require 'priorityqueuejs'

{
  ERR_SYNCHRO_STATE
  nogthrow
} = NogError

{ AA_GET_SYNCHRO_CONTENT } = require './nog-sync-store.coffee'
{ synchroCommitReposDiffStream } = require './nog-sync-diff.coffee'
{ mergeSynchro } = require './nog-sync-merge.coffee'

NULL_SHA1 = '0000000000000000000000000000000000000000'

remoteFnames = [
  'getPing',
  'getSynchroCommit',
  'getSynchroTree',
  'getSynchroObject',
  'getSynchroEntries',
  'getContentCommit',
  'getContentTree',
  'getContentObject',
  'getContentEntries',
]


isDuplicateMongoIdError = (err) ->
  return err.code == 11000


defNogSyncMethods = (opts) ->
  {namespace, syncStore} = opts

  nsMeth = namespace.meth
  defMethod = (name, func) ->
    qualname = nsMeth + '.' + name
    def = {}
    def[qualname] = func
    Meteor.methods def

  defMethod 'getPing', (opts) ->
    return syncStore.getPing Meteor.user(), opts

  defMethod 'getSynchroCommit', (opts) ->
    return syncStore.getCommitRaw Meteor.user(), opts

  defMethod 'getSynchroTree', (opts) ->
    return syncStore.getTreeRaw Meteor.user(), opts

  defMethod 'getSynchroObject', (opts) ->
    return syncStore.getObjectRaw Meteor.user(), opts

  defMethod 'getSynchroEntries', (opts) ->
    return syncStore.getEntriesRaw Meteor.user(), opts

  defMethod 'getContentCommit', (opts) ->
    return syncStore.getContentCommitRaw Meteor.user(), opts

  defMethod 'getContentTree', (opts) ->
    return syncStore.getContentTreeRaw Meteor.user(), opts

  defMethod 'getContentObject', (opts) ->
    return syncStore.getContentObjectRaw Meteor.user(), opts

  defMethod 'getContentEntries', (opts) ->
    return syncStore.getContentEntriesRaw Meteor.user(), opts


publishNogSync = (opts) ->
  {namespace, syncStore} = opts
  nsColl = namespace.coll

  Meteor.publish "#{nsColl}.synchros", ->
    if not syncStore.testAccess @userId, AA_GET_SYNCHRO_CONTENT, {}
      console.log "[sync] Denied publish synchros to userId #{@userId}"
      @ready()
      return null
    console.log "[sync] Published synchros to userId #{@userId}"
    syncStore.synchros.find()


defCalls = (opts) ->
  {connection, rnsMeth, fnames} = opts

  call = {}
  defCall = (name) ->
    qname = rnsMeth + '.' + name
    call[name] = (args...) -> connection.call qname, args...

  for n in fnames
    defCall n

  return call


connectRemote = (euid, opts) ->
  {config, syncLoop, syncStore, ourUsername, fallbackPullInterval_ms} = opts
  { name, url, namespace, keyid, secretkey, username: theirUsername } = config

  # It is sufficient to call login in `onReconnect()`, which will be called
  # during the initial connect.

  connection = DDP.connect url
  connection.onReconnect = ->
    console.log '[sync] Reconnect', url
    if keyid?
      ddplogin connection, {keyid, secretkey}
    else
      console.log '[sync] Skipping DDP login', url

  rnsMeth = namespace.meth
  call = defCalls {connection, rnsMeth, fnames: remoteFnames}

  rnsColl = namespace.coll
  subscriptionName = collName = "#{rnsColl}.synchros"
  remoteSynchros = new Mongo.Collection collName, {connection}

  remote = _createRemote {
    name, ourUsername, theirUsername, connection, call, syncStore,
    remoteSynchros
  }
  remote.startObserver euid, {
    subscriptionName, syncLoop, fallbackPullInterval_ms
  }
  return remote


# Handling login correctly during reconnect is tricky.  Only
# `apply...{wait:true}`, as in the accounts package, works.
#
# See SO question how accounts package handles login:
# <http://stackoverflow.com/questions/14149245/how-to-use-a-client-side-reconnection-event-in-meteor>
#
# See `meteor/packages/accounts-base`, `grep onReconnect`, `grep wait.*true`.
#
# Ideas that did not work:
#
# `asyncWrap()` of `conn.call` to make it blocking.  It worked on initial
# connect.  But it froze forever on a real reconnect.
#
# `conn.call` without `{wait: true}`.  It also froze forever on a real
# reconnect.
#
# XXX If login fails, try to reconnect after a timeout.  The subscription
# should be automatically handled.  It is unclear whether the strategy can
# handle real world problems.

ddplogin = (conn, key) ->
  req = {method: 'GET', url: '/ddplogin'}
  signRequest key, req
  conn.apply 'login', [{nogauthreq: req}], {wait: true}, (err, res) ->
    if err
      console.error '[sync] Failed to login.  Trying to reconnect.', err
      forceReconnect = ->
        conn.disconnect()
        conn.reconnect()
      Meteor.setTimeout forceReconnect, 5 * 1000
      return
    console.log "
      [sync] Logged in as user id `#{res.id}`; token expires
      #{res.tokenExpires}.
    "


_createRemote = (opts) ->
  {
    name, theirUsername, ourUsername, connection, call, syncStore,
    remoteSynchros
  } = opts

  return {
    name
    ourUsername
    theirUsername
    connection
    call
    syncStore
    remoteSynchros
    observer:  null
    subscription: null

    startObserver: (euid, opts) ->
      {subscriptionName, syncLoop, fallbackPullInterval_ms} = opts

      triggerPull = =>
        syncLoop.pull euid, { remote: this }

      # Use `observe()` instead of `observeChanges()`, since `observeChanges()`
      # delivers the full toplevel fields as changed.  But we want to pull only
      # if the ref `branches/master` changed and ignore if `remotes/**`
      # changed.  So take the full docs and compare in detail.

      @observer = @remoteSynchros.find({
        owner: @theirUsername
        name: 'all'  # XXX The synchro name should perhaps become a parameter.
      }).observe {

        added: (doc) =>
          if (sha = doc.refs?['branches/master'])? and sha != NULL_SHA1
            triggerPull()

        changed: (newDoc, oldDoc) =>
          if newDoc._ping? and newDoc._ping != oldDoc._ping
            syncLoop.fetchPing euid, {remote: this}
          oldMaster = oldDoc.refs?['branches/master']
          newMaster = newDoc.refs?['branches/master']
          if newMaster? and newMaster != NULL_SHA1 and newMaster != oldMaster
            console.log '[sync] remote master changed', newMaster
            triggerPull()

      }

      @subscription = connection.subscribe subscriptionName

      # Trigger a pull at regular intervals as a fallback measure against
      # interrupted actions.  Specifically, a merge must be restarted that
      # failed inside apply to unblock snapshot and fetch.

      if fallbackPullInterval_ms
        @pullInterval = Meteor.setInterval triggerPull, fallbackPullInterval_ms


    fetchPing: (euid, opts) ->
      res = @call.getPing {owner: @theirUsername}
      unless (token = res[@theirUsername])?
        return
      $set = {}
      $set["_ping.#{@theirUsername}"] = token
      @syncStore.synchros.update {owner: @ourUsername, name: 'all'}, {$set}


    pull: (euid, opts) ->
      log = console.log
      ownerName = @ourUsername
      synchroName = 'all'
      if (op = @syncStore.getOp { ownerName, synchroName })?
        console.log("[sync] skipping fetch due to active op #{op.op}.")
      else
        @fetch euid
      begin_ms = Date.now()
      res = mergeSynchro euid, {
        syncStore: @syncStore
        ownerName, synchroName
        branch: 'master'
        remoteName: @name
      }
      if res.status != 'up-to-date'
        end_ms = Date.now()
        console.log(
          "[sync] merge `#{res.status}` took " +
          "#{((end_ms - begin_ms) / 1000).toFixed(3)}s."
        )
      return res


    fetch: (euid) ->
      ourSynchro = @syncStore.synchros.findOne {
        owner: @ourUsername, name: 'all'
      }
      remoteName = @name
      remoteRef = "remotes/#{remoteName}/branches/master"
      theirSynchro = @remoteSynchros.findOne {
        owner: @theirUsername, name: 'all'
      }
      unless theirSynchro?
        console.log(
          "[sync] Fetch skipped due to missing remote repo state for " +
          "peer `#{remoteName}`."
        )
        return

      from = ourSynchro.refs[remoteRef]
      if not from? or from == NULL_SHA1
        from = null

      ourMaster = ourSynchro.refs['branches/master']
      if not ourMaster? or ourMaster == NULL_SHA1
        ourMaster = null

      to = theirSynchro.refs['branches/master']
      if not to? or to == NULL_SHA1
        to = null

      # Fetch should not be called if their master is NULL_SHA1.  If it is, the
      # fetch is ignored.
      if not to?
        return

      if from == to  # already up-to-date.
        return

      # If their master points to our master, which can be the case if they
      # just fast-forward pulled from us, we do not need to actually fetch.
      # But we still need to update our remote ref.

      unless to == ourMaster
        console.log('[sync] begin fetch synchro commit', to)

        { nCommits } = @_commitWalkPaintBottom {
          tops: [to],
          bottoms: _.values(ourSynchro.refs)
        }

        console.log("[sync] fetched synchro: #{nCommits} commits.")

        # We have all synchro commits.  Now fetch content commits, and record
        # the remote ref in our repo, using `from` as a nonce to protect
        # against concurrent updates, which should not happen if there is only
        # a single app instance that is responsible for replication.

        counts = @_fetchContentForReposSnapDiff(from, to)
        { nContentCommits, nContentTrees, nContentObjects } = counts
        console.log(
          "[sync] fetched content: " +
          "#{nContentCommits} commits, " +
          "#{nContentTrees} trees, " +
          "#{nContentObjects} objects."
        )

      @syncStore.updateRef euid, {
        ownerName: @ourUsername
        synchroName: 'all'
        refName: remoteRef
        old: from, new: to
      }


    # Commits are fetched shallow.  Trees are fetched deep.
    #
    # This is a simple, synchronous reference implementation.  An optimized
    # implementation could try to hide latency by prefetching commits in
    # commit-date order.
    #
    # The naive implementation stops only at `bottoms`.  It may fetch far too
    # many commits, for example with crisscross merges.  A better way is to
    # stop fetching at all commits that can be reached from `bottoms`, too.

    _commitWalkNaive: (opts) ->
      { tops, bottoms } = opts
      nCommits = 0

      seen = {}
      for b in bottoms
        seen[b] = true
      todo = _.clone tops
      while todo.length > 0
        sha = todo.shift()
        if seen[sha]
          continue
        nCommits += 1
        commit = @call.getSynchroCommit {sha}
        treeSha = commit.tree

        @syncStore.insertCommitRawSudo { content: commit }

        seen[commit._id] = true
        for p in commit.parents
          unless seen[p]
            todo.push(p)

        @_fetchTreeDeepFirst treeSha

      return { nCommits }


    # `_commitWalkPaintBottom()` fetches remote aka top commits in commit date
    # order and simultaneously paints local aka bottom commits to limit the
    # fetch in order to avoid fetching unnecessary commits with complex
    # history, like crisscross merges.
    #
    # The child timestamp is used as a temporary substitute for the unknown
    # timestamp of unfetched parent commits.  Assuming the child timestamp is
    # larger, which should be the case for any sane history, this is a
    # reasonable priority order.  After fetching, the commit is re-queued with
    # the correct timestamp before walking to its parents, so that bottom
    # commits have a chance to walk first and prune the top walk.

    _commitWalkPaintBottom: (opts) ->
      {tops, bottoms} = opts

      # For illustration purposes, the walk is limited to one day.  A
      # production implementation should probably have a configuration option
      # for the cutoff.
      timestampCutoff = moment().unix() - 60 * 60 * 24  # unix() is in seconds.

      # `PriorityQueue` returns greatest first, like a reversed sort.
      todo = new PriorityQueue((a, b) -> a.timestamp - b.timestamp)

      BOTTOM = (1 << 0)
      FETCH_TOP = (1 << 1)
      WALK_TOP = (1 << 2)

      moreTopCommits = ->
        for c in todo._elements
          if c.type & (FETCH_TOP | WALK_TOP)
            return true
        return false

      for t in tops
        todo.enq { type: FETCH_TOP, timestamp: Number.MAX_VALUE, sha: t }

      seen = {}
      for b in bottoms
        if b == NULL_SHA1
          continue
        seen[b] |= BOTTOM
        commit = @syncStore.getCommitSudo({ sha: b })
        timestamp = commit.commitDate.unix()
        parents = commit.parents
        todo.enq { type: BOTTOM, timestamp, parents }

      nCommits = 0
      while moreTopCommits()
        t = todo.deq()

        if t.type == BOTTOM
          { parents } = t
          for p in parents
            if seen[p] & BOTTOM
              continue
            parentCommit = @syncStore.getCommitSudo({ sha: p })
            todo.enq {
              type: BOTTOM,
              timestamp: parentCommit.commitDate.unix(),
              parents: parentCommit.parents,
            }
            seen[p] |= BOTTOM
          continue

        if t.type == FETCH_TOP
          { sha } = t
          if seen[sha]
            continue

          nCommits += 1
          commit = @call.getSynchroCommit { sha }
          @_fetchTreeBulkSha commit.tree
          @syncStore.insertCommitRawSudo { content: commit }

          # Re-queue with correct timestamp - 1 to give bottom commits the
          # chance to win over top commits before walking to the parents.
          timestamp = moment.parseZone(commit.commitDate).unix() - 1
          todo.enq { type: WALK_TOP, timestamp, sha, parents: commit.parents }
          continue

        if t.type == WALK_TOP
          { timestamp, sha, parents } = t
          if seen[sha]
            continue
          if timestamp < timestampCutoff
            continue
          for p in parents
            unless seen[p]
              # Queue parents with child timestamp.
              todo.enq { type: FETCH_TOP, timestamp, sha: p }
          seen[sha] |= WALK_TOP
          continue

      return { nCommits }


    _fetchTreeDeepFirst: (sha) ->

      if @syncStore.hasTreeSudo({ sha })
        return  # Already had the tree.  Trees are deep, so stop recursion.

      tree = @call.getSynchroTree {sha}
      for ent in tree.entries
        switch ent.type
          when 'tree'
            @_fetchTreeDeepFirst ent.sha1
          when 'object'
            unless @syncStore.hasObjectSudo({ sha: ent.sha1 })
              obj = @call.getSynchroObject {sha: ent.sha1}
              if obj.blob? and obj.blob != NULL_SHA1
                nogthrow ERR_SYNCHRO_STATE, {
                  reason: "Synchro must not use blobs: object #{obj._id}."
                }
              @syncStore.insertObjectRawSudo { content: obj }

      @syncStore.insertTreeRawSudo { content: tree }


    # The bulk version fetches all missing children in a single call.

    _fetchTreeBulkSha: (sha) ->
      if @syncStore.hasTreeSudo({ sha })
        return  # Already have the tree.  Trees are deep, so stop recursion.
      tree = @call.getSynchroTree {sha}
      @_fetchTreeBulkTree tree

    _fetchTreeBulkTree: (tree) ->
      treeShas = []
      objectShas = []
      for ent in tree.entries
        switch ent.type
          when 'tree'
            if @syncStore.hasTreeSudo({ sha: ent.sha1 })
              continue
            treeShas.push ent.sha1
          when 'object'
            if @syncStore.hasObjectSudo({ sha: ent.sha1 })
              continue
            objectShas.push ent.sha1

      if (treeShas.length) > 0 or (objectShas.length > 0)
        { trees, objects } = @call.getSynchroEntries { treeShas, objectShas }
        for tr in trees
          @_fetchTreeBulkTree tr
        for obj in objects
          if obj.blob? and obj.blob != NULL_SHA1
            nogthrow ERR_SYNCHRO_STATE, {
              reason: "Synchro must not use blobs: object #{obj._id}."
            }
          @syncStore.insertObjectRawSudo { content: obj }

      @syncStore.insertTreeRawSudo { content: tree }


    _fetchContentForReposSnapDiff: (from, to) ->
      nContentCommits = 0
      nContentTrees = 0
      nContentObjects = 0

      ondeleted = ->  # Nothing to fetch.

      onadded = (ab) =>
        { refs, conflicts } = ab.b.meta.nog
        tops = _.values(refs).concat(_.flatten(_.values(conflicts)))
        tops = _.select(tops, (sha) -> sha != NULL_SHA1)
        counts = @fetchContent { tops }
        nContentCommits += counts.nContentCommits
        nContentTrees += counts.nContentTrees
        nContentObjects += counts.nContentObjects

      onmodified = (ab) =>
        { refs: oldRefs, conflicts: oldConflicts } = ab.a.meta.nog
        { refs: newRefs, conflicts: newConflicts } = ab.b.meta.nog
        bottoms = _.values(oldRefs).concat(_.flatten(_.values(oldConflicts)))
        tops = _.values(newRefs).concat(_.flatten(_.values(newConflicts)))
        tops = _.select(tops, (sha) -> sha != NULL_SHA1)
        counts = @fetchContent { tops, bottoms }
        nContentCommits += counts.nContentCommits
        nContentTrees += counts.nContentTrees
        nContentObjects += counts.nContentObjects

      synchroCommitReposDiffStream {
        aSha: from
        bSha: to
        ondeleted, onadded, onmodified
        store: {
          getCommit: (sha) => @syncStore.getCommitSudo { sha }
          getTree: (sha) => @syncStore.getTreeSudo { sha }
        }
      }

      return { nContentCommits, nContentTrees, nContentObjects }

    # `fetchContent()` performs a deep fetch of a content commit with all its
    # dependencies (except for blobs).  It uses a local walk.
    #
    # Commits are fetched shallow.  Trees are fetched deep.

    fetchContent: (opts) ->
      { tops, bottoms } = opts
      bottoms ?= []
      return @_contentCommitWalk { tops, bottoms }


    # XXX `_contentCommitWalk()` could be optimized similarly to
    # `_commitWalkPaintBottom()`.  But it is unnecessary as long as we use a
    # simple linear history for content repos.

    _contentCommitWalk: (opts) ->
      { tops, bottoms } = opts
      nContentCommits = 0
      nContentTrees = 0
      nContentObjects = 0

      seen = {}
      for b in bottoms
        seen[b] = true
      todo = _.clone tops
      while todo.length > 0
        sha = todo.shift()
        if seen[sha]
          continue
        nContentCommits += 1
        commit = @call.getContentCommit { sha }
        treeSha = commit.tree

        @syncStore.insertContentCommitRawSudo { content: commit }

        seen[commit._id] = true
        for p in commit.parents
          unless seen[p]
            todo.push(p)

        counts = @_fetchContentTreeBulkSha treeSha
        nContentTrees += counts.nContentTrees
        nContentObjects += counts.nContentObjects

      return { nContentCommits, nContentTrees, nContentObjects }

    _fetchContentTreeDeepFirst: (sha) ->
      nContentTrees = 0
      nContentObjects = 0
      contentStore = @syncStore.contentStore

      if contentStore.hasTreeSudo({ sha })
        # Already had the tree.  Trees are deep, so stop recursion.
        return { nContentTrees, nContentObjects }

      nContentTrees += 1
      tree = @call.getContentTree { sha }
      for ent in tree.entries
        switch ent.type
          when 'tree'
            counts = @_fetchContentTreeDeepFirst ent.sha1
            nContentTrees += counts.nContentTrees
            nContentObjects += counts.nContentObjects
          when 'object'
            unless contentStore.hasObjectSudo({ sha: ent.sha1 })
              nContentObjects += 1
              obj = @call.getContentObject { sha: ent.sha1 }
              if obj.blob? and obj.blob != NULL_SHA1
                @syncStore.insertContentBlobPlaceholderSudo { sha: obj.blob }
              @syncStore.insertContentObjectRawSudo { content: obj }

      @syncStore.insertContentTreeRawSudo { content: tree }

      return { nContentTrees, nContentObjects }


    _fetchContentTreeBulkSha: (sha) ->
      nContentTrees = 0
      nContentObjects = 0
      if @syncStore.contentStore.hasTreeSudo({ sha })
        # Already have the tree.  Trees are deep, so stop recursion.
        return { nContentTrees, nContentObjects }
      tree = @call.getContentTree { sha }
      nContentTrees += 1
      counts = @_fetchContentTreeBulkTree(tree)
      nContentTrees += counts.nContentTrees
      nContentObjects += counts.nContentObjects
      return { nContentTrees, nContentObjects }

    _fetchContentTreeBulkTree: (tree) ->
      nContentTrees = 0
      nContentObjects = 0

      treeShas = []
      objectShas = []
      for ent in tree.entries
        switch ent.type
          when 'tree'
            if @syncStore.contentStore.hasTreeSudo({ sha: ent.sha1 })
              continue
            treeShas.push ent.sha1
          when 'object'
            if @syncStore.contentStore.hasObjectSudo({ sha: ent.sha1 })
              continue
            objectShas.push ent.sha1

      if (treeShas.length > 0) or (objectShas.length > 0)
        { trees, objects } = @call.getContentEntries { treeShas, objectShas }
        nContentTrees += trees.length
        nContentObjects += objects.length
        for tr in trees
          counts = @_fetchContentTreeBulkTree tr
          nContentTrees += counts.nContentTrees
          nContentObjects += counts.nContentObjects
        for obj in objects
          if obj.blob? and obj.blob != NULL_SHA1
            @syncStore.insertContentBlobPlaceholderSudo { sha: obj.blob }
          @syncStore.insertContentObjectRawSudo { content: obj }

      @syncStore.insertContentTreeRawSudo { content: tree }

      return { nContentTrees, nContentObjects }

    disconnect: ->
      if @pullInterval?
        Meteor.clearInterval @pullInterval
        @pullInterval = null
      @subscription.stop()
      @observer.stop()
      @connection.close()

  }



module.exports.connectRemote = connectRemote
module.exports.publishNogSync = publishNogSync
module.exports.defNogSyncMethods = defNogSyncMethods
