crypto = require('crypto')
sha1_hex = (d) -> crypto.createHash('sha1').update(d, 'utf8').digest('hex')


class StagingPrefixTree
  constructor: (opts) ->
    @store = opts.store
    @nRepoPrefixLevels = 2

    if opts.rootSha?
      @root = @getTree opts.rootSha
    else
      @root = {
        name: 'repos',
        meta: {},
        entries: [],
      }

  getTree: (sha) ->
    t = @store.getTree sha
    unless t?
      return
    t = _.clone(t)
    delete t._id
    t.entries = _.clone(t.entries)
    return t

  getEntryName: (e) ->
    if e.name?
      e.name
    else
      @getTree(e.sha1).name

  # `expandInnerDefault()` expands and returns an existing entry or inserts the
  # provided default.  It maintains `tree.entries` in sorted order.

  expandInnerDefault: (tree, def) ->
    idx = _.sortedIndex tree.entries, def, (e) => @getEntryName(e)

    if (e = tree.entries[idx])? and @getEntryName(e) == def.name
      if e.sha1?  # Expand if necessary.
        e = @getTree(e.sha1)
        tree.entries[idx] = e
      return e

    tree.entries.splice idx, 0, def
    return def

  expandPath: (sha) ->
    tree = @root
    for lv in [0...@nRepoPrefixLevels]
      prefix = sha[0..(lv * 2 + 1)]
      tree = @expandInnerDefault tree, {
        name: "repos:#{prefix}",
        meta: {},
        entries: [],
      }
    return tree

  set: (rsnap) ->
    sha = sha1_hex(rsnap.name)
    tree = @expandPath sha
    # Entries are sorted by sha1(repo snap name).
    idx = _.sortedIndex tree.entries, rsnap, (e) => sha1_hex(@getEntryName(e))
    if (e = tree.entries[idx])? and @getEntryName(e) == rsnap.name
      tree.entries[idx] = rsnap
    else
      tree.entries.splice idx, 0, rsnap

  del: (rsnap) ->
    sha = sha1_hex(rsnap.name)
    tree = @expandPath sha
    # Entries are sorted by sha1(repo snap name).
    idx = _.sortedIndex tree.entries, rsnap, (e) => sha1_hex(@getEntryName(e))
    if (e = tree.entries[idx])? and @getEntryName(e) == rsnap.name
      tree.entries.splice idx, 1
    else
      true  # Ignore del missing.

  asNogTree: ->
    rejectEmptyInner = (tree) ->
      entries = []
      for e in tree.entries
        if e.sha1
          entries.push e
        else
          child = rejectEmptyInner(e)
          ty = child.name.split(':')[0]
          if ty == 'repos' and child.entries.length == 0
            continue
          entries.push child
      return {
        name: tree.name,
        meta: tree.meta,
        entries,
      }
    return rejectEmptyInner @root


createStagingPrefixTree = (opts) -> new StagingPrefixTree(opts)


module.exports.createStagingPrefixTree = createStagingPrefixTree
