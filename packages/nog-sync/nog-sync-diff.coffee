{
  ERR_LOGIC
  ERR_PARAM_INVALID
  nogthrow
} = NogError


# `synchroTreeReposDiffStream({...})` recursively traverses two prefix sync
# repo snapshot trees `aSha` and `bSha` and reports differences on the repo
# level to callbacks `onadded({b})`, `ondeleted({a})`, `onmodified({a, b})`
# with trees `a` and `b`.  Trees are accessed through `store.getTree(sha)`;
# `store` must be provided by the caller.

synchroTreeReposDiffStream = (opts) ->
  { aSha, bSha, store, onadded, ondeleted, onmodified } = opts

  # Internally, `treeDiff()` operates on expanded nog trees, so that the names
  # are readily available.

  treeDiff = (a, b) ->
    abByName = {}
    if a?
      for e in a.entries
        child = store.getTree(e.sha1)
        abByName[child.name] ?= {}
        abByName[child.name].a = child
    if b?
      for e in b.entries
        child = store.getTree(e.sha1)
        abByName[child.name] ?= {}
        abByName[child.name].b = child

    names = _.keys abByName
    names.sort()
    for name in names
      kind = name.split(':')[0]
      ab = abByName[name]
      switch kind
        when 'repo'
          if ab.a? and ab.b?
            if ab.a._id != ab.b._id
              onmodified ab
          else if ab.a?
            ondeleted ab
          else if ab.b?
            onadded ab
          else
            nogthrow ERR_LOGIC
        when 'repos'
          treeDiff ab.a, ab.b
        else
          nogthrow ERR_PARAM_INVALID, {
            reason: "Invalid tree name `#{name}`."
          }

  getTreeOrNull = (sha) ->
    unless sha?
      return null
    return store.getTree(sha)

  treeDiff getTreeOrNull(aSha), getTreeOrNull(bSha)


snapTreeShaForCommitSha = (store, commitSha) ->
  unless commitSha?
    return null
  commit = store.getCommit commitSha
  tree = store.getTree commit.tree
  return tree.entries[0].sha1


synchroCommitReposDiffStream = (opts) ->
  { aSha, bSha, store, ondeleted, onadded, onmodified } = opts
  synchroTreeReposDiffStream {
    aSha: snapTreeShaForCommitSha(store, aSha),
    bSha: snapTreeShaForCommitSha(store, bSha),
    store,
    ondeleted, onadded, onmodified,
  }


# `synchroTreeReposDiff3Stream({...})` simultaneously computes diffs from a
# repo snapshot tree `baseSha` to two trees `aSha` and `bSha`.  It reports
# differences on the repo level by calling `onchanged({base, a, b})`.
#
# - `a` is `undefined`: `a` is unchanged.
# - `a` is `D`: `a` has been deleted.
# - `base` is `undefined`, `a` is Object: `a` has been added.
# - `base` is Object, `a` is Object: `a` has been modified.

synchroTreeReposDiff3Stream = (opts) ->
  { baseSha, aSha, bSha, store, onchanged } = opts

  treeDiff3 = (base, a, b) ->
    ab3ByName = {}
    if base?
      for e in base.entries
        child = store.getTree(e.sha1)
        ab3ByName[child.name] ?= {}
        ab3ByName[child.name].base = child
    if a?
      for e in a.entries
        child = store.getTree(e.sha1)
        ab3ByName[child.name] ?= {}
        ab3ByName[child.name].a = child
    if b?
      for e in b.entries
        child = store.getTree(e.sha1)
        ab3ByName[child.name] ?= {}
        ab3ByName[child.name].b = child

    names = _.keys ab3ByName
    names.sort()
    for name in names
      kind = name.split(':')[0]
      ab3 = ab3ByName[name]
      switch kind
        when 'repo'
          change = {}
          if ab3.base?
            if ab3.a?
              if ab3.a._id != ab3.base._id
                change.base = ab3.base
                change.a = ab3.a
            else
              change.base = ab3.base
              change.a = 'D'
            if ab3.b?
              if ab3.b._id != ab3.base._id
                change.base = ab3.base
                change.b = ab3.b
            else
              change.base = ab3.base
              change.b = 'D'
          else
            if ab3.a?
              change.a = ab3.a
            if ab3.b?
              change.b = ab3.b
          unless _.isEmpty change
            onchanged change
        when 'repos'
          treeDiff3 ab3.base, ab3.a, ab3.b
        else
          nogthrow ERR_PARAM_INVALID, {
            reason: "Invalid tree name `#{name}`."
          }

  getTreeOrNull = (sha) ->
    unless sha?
      return null
    return store.getTree(sha)

  treeDiff3 getTreeOrNull(baseSha), getTreeOrNull(aSha), getTreeOrNull(bSha)


synchroCommitReposDiff3Stream = (opts) ->
  { aSha, bSha, baseSha, store, onchanged } = opts
  synchroTreeReposDiff3Stream({
    aSha: snapTreeShaForCommitSha(store, aSha),
    bSha: snapTreeShaForCommitSha(store, bSha),
    baseSha: snapTreeShaForCommitSha(store, baseSha),
    store,
    onchanged,
  })


module.exports.synchroTreeReposDiffStream = synchroTreeReposDiffStream
module.exports.synchroCommitReposDiffStream = synchroCommitReposDiffStream

module.exports.synchroTreeReposDiff3Stream = synchroTreeReposDiff3Stream
module.exports.synchroCommitReposDiff3Stream = synchroCommitReposDiff3Stream

module.exports.snapTreeShaForCommitSha = snapTreeShaForCommitSha
