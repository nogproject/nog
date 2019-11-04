{ NogContent } = require('meteor/nog-content')


decodeMore = (encMore) ->
  more = {}
  for r in encMore
    more[r.key] = r.val
  more


NogContent.contentIndex = contentIndex = new Mongo.Collection 'content_index',
  transform: (d) ->
    if d.more?
      d.more = decodeMore d.more
    d


optTextSearch = Meteor.settings.public?.optTextSearch ? true
optNogSharing = Meteor.settings.public?.optNogSharing ? true


# `createQuery()` implements a mini language for searching.  It parses the
# search string and returns a MongoDB selector object.  See help text in
# `search.jade` for an explanation on the search string syntax.
#
# `opts.search.userId` is provided by easy search in publish-based search to
# get the user. `return false` tells easy search to publish nothing.
createQuery =  (searchString, opts) ->
  # console.log 'query:', searchString
  userId = opts.search.userId
  if not NogAccess.testAccess userId, 'nog-content/get', {style: 'loose'}
    return false

  tokenize = (str) ->
    for tok in str.match(/// (\S+:)?"[^"]+" | \S+ ///g)
      isQuoted = tok.match(/"/)?
      tok = tok.replace /"/g, ''  # Remove quotes.
      parts = tok.split ':'
      if parts.length is 1
        {
          type: 'text', text: tok, isQuoted
        }
      else
        {
          type: 'prefix', text: parts[1..].join(':'), prefix: parts[0]
          isQuoted
        }

  # console.log 'tokens', tokenize(searchString)

  fieldMatch = (field, tok) ->
    q = {}
    if tok.isQuoted
      q[field] = {$regex: '^' + tok.text + '$'}

    else
      q[field] = {$regex: '.*' + tok.text + '.*', $options: 'i'}
    q

  text = []
  more = []
  for tok in tokenize(searchString)
    switch tok.type
      when 'text'
        if optTextSearch
          if tok.isQuoted
            text.push '"' + tok.text + '"'
          else
            text.push tok.text
        else
          if tok.isQuoted
            text.push {'text': {$regex: tok.text, $options: 'i'}}
            text.push {'description': {$regex: tok.text, $options: 'i'}}
            text.push {'path': {$regex: tok.text, $options: 'i'}}
          else
            for t in tok.text.split(' ')
              text.push {'text': {$regex: t, $options: 'i'}}
              text.push {'description': {$regex: t, $options: 'i'}}
              text.push {'path': {$regex: t, $options: 'i'}}
      when 'prefix'
        switch tok.prefix
          when 'path'
            more.push fieldMatch 'path', tok
          when 'text', 'description'
            more.push fieldMatch tok.prefix, tok
          when 'repo'
            more.push fieldMatch 'refs.repoName', tok
          when 'owner'
            more.push fieldMatch 'refs.ownerName', tok
          else
            if tok.isQuoted
              more.push {
                'more':
                  $elemMatch:
                    key: {$regex: '^' + tok.prefix + '$'}
                    val: {$regex: '^' + tok.text + '$'}
              }
            else
              more.push {
                'more':
                  $elemMatch:
                    key: {$regex: '.*' + tok.prefix + '.*', $options: 'i'}
                    val: {$regex: '.*' + tok.text + '.*', $options: 'i'}
              }

  sel = {}
  if text.length > 0
    if optTextSearch
      sel['$text'] = {$search: text.join(' ')}
    else
      sel['$or'] = text
  if more.length > 0
    sel['$and'] = more

  if optNogSharing
    user = Meteor.users.findOne userId
    inCircles = user.sharing?.inCircles ? []
    sharerIds = _.unique(c.fromId for c in inCircles)
    circleIds = _.unique(c.circleId for c in inCircles)
    sel['$and'] ?= []
    sel['$and'].push
      $or: [
        # Search own repos.
        {'refs.ownerId': userId}

        # Search public repos.
        {'refs.sharing.public': true}

        # Search repos that are shared with all circles.  Use $elemMatch to
        # match refs that are shared with all circles *and* the searcher is in
        # the circle of the owner.
        {
          refs:
            $elemMatch: {
              'sharing.allCircles': true
              'ownerId': {$in: sharerIds}
            }
        }

        # Search circles that are shared by id.
        {
          'refs.sharing.circles': {$in: circleIds}
        }
      ]

  # contentIndex.rawCollection().find(sel).explain (err, res) ->
  #   console.log 'search: ', EJSON.stringify(sel, {indent: 2})
  #   console.log 'explanation', res

  return sel


# Easy search would automatically create the text index; but it does not create
# normal indexes.
NogContent.searchContent = searchContent = new EasySearch.Index({
  collection: NogContent.contentIndex,
  fields: ['text', 'description', 'path'],
  engine: new EasySearch.MongoDB({
    selector: (searchDefinition, options, aggregation) ->
      if searchDefinition.text == ''
        return searchDefinition
      else
        return createQuery(searchDefinition.text, options)
  })
})


module.exports.contentIndex = NogContent.contentIndex
module.exports.searchContent = NogContent.searchContent
