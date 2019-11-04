{ _ } = require 'meteor/underscore'
{ NogContent } = require './nog-content.coffee'


# `findEntryQualified()` uses an explicitly qualified key type if provided.
# Otherwise, it falls back to name.  Paths can be:
#
#  - name!<string>
#  - index!<number>
#  - <name>
#
#  Bang '!' is used as the type separator instead of colon ':', since '!' may
#  be used unencoded.  See RFC 1738, quote: '''Thus, only alphanumerics, the
#  special characters "$-_.+!*'(),", and reserved characters used for their
#  reserved purposes may be used unencoded within a URL.'''

findEntryQualified = (entries, key) ->
  if (m = key.match /^name!(.*)$/)?
    findEntryByName entries, m[1]
  else if (m = key.match /^index!([0-9]+)$/)?
    findEntryByIdx entries, Number(m[1])
  else
    findEntryByName entries, key


findEntryByIdx = (entries, idx) ->
  if (e = entries[idx])?
    _.extend {idx}, e
  else
    undefined


findEntryByName = (entries, name) ->
  content = []
  for e, idx in entries
    switch e.type
      when 'object' then c = NogContent.objects.findOne(e.sha1)
      when 'tree' then c = NogContent.trees.findOne(e.sha1)
    content.push c
    if c?.name is name
      return _.extend {idx}, e

  # If no exact match has been found, try `md` instead of `html`.
  if (name.match /// [.]html$ ///)?
    mdName = name.replace /// [.]html$ ///, '.md'
    for c, idx in content
      if c?.name is mdName
        return _.extend {idx}, entries[idx]

  return undefined


NogContent.parseRefTreePath = parseRefTreePath = (repo, refTreePath) ->
  isSha1 = (x) ->
    x.match /^[0-9a-f]{40}$/

  [head, tail...] = refTreePath.split '/'
  parsed = {}
  if isSha1 head
    parsed.refType = 'id'
    parsed.ref = head
    parsed.treePath = tail.join '/'
    parsed.commitId = head
  else if repo.refs['branches/' + head]?
    parsed.refType = 'branch'
    parsed.ref = head
    parsed.treePath = tail.join '/'
    parsed.refFullName = 'branches/' + head
  else
    return null

  if (tail.length is 0) or (tail[0] is '')
    parsed.namePath = []
  else
    parsed.namePath = tail

  return parsed


NogContent.resolveRefTreePath = resolveRefTreePath = (repo, refTreePath) ->
  if not (parsed = parseRefTreePath repo, refTreePath)?
    return null

  resolved = {repo, refTreePath}
  resolved.refType = parsed.refType
  resolved.ref = parsed.ref
  resolved.treePath = parsed.treePath
  if (c = parsed.commitId)?
    resolved.commitId = c
  else if (c = repo.refs[parsed.refFullName])?
    resolved.commitId = c
  else
    return null

  if not (commit = NogContent.commits.findOne(c))?
    return null
  resolved.commit = commit

  # How to handle missing trees and objects?  This must not happen on the
  # server.  On the client, however, entries may be missing if the published
  # docs have not yet arrived.  But this should not happen if subscriptions are
  # conistently used and a path is only resolved after all subscriptions are
  # ready.
  #
  # The implementation could be optimized by avoiding duplicate findOnes()
  # during named lookup.
  walk = (id, path, findEntry) ->
    if not (tree = NogContent.trees.findOne(id))?
      return null
    res = [{idx: 0, name: tree.name, type: 'tree', content: tree}]
    if path.length is 0
      return res
    [head, tail...] = path
    if not (e = findEntry(tree.entries, head))?
      return null
    switch e.type
      when 'object'
        if tail.length > 0  # Check that the paths ends at the object.
          return null
        if not (obj = NogContent.objects.findOne(e.sha1))?
          return null
        res.push {idx: e.idx, name: obj.name, type: 'object', content: obj}
      when 'tree'
        if not (w = walk e.sha1, tail, findEntry)?
          return null
        # Fix the index, since the next level walk() does not know it.
        w[0].idx = e.idx
        res = res.concat w
      else
        console.log "Unknown type #{e.type}."
        return null
    return res

  if not (w = walk(commit.tree, parsed.namePath, findEntryQualified))?
    return null

  [resolved.tree, resolved.contentPath...] = w
  resolved.numericPath = (e.idx for e in w[1..])
  resolved.namePath = (e.name for e in w[1..])
  [h..., resolved.last] = w

  return resolved


module.exports.findEntryQualified = findEntryQualified
