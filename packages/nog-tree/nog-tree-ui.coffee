{defaultErrorHandler} = NogError
{asXiBUnit} = NogFmt

routerPath = (route, opts) -> FlowRouter.path route, opts


NULL_SHA1 = '0000000000000000000000000000000000000000'

if (c = NogApp?.subsCache)?
  console.log 'Using main subs cache'
  subsCache = c
else
  subsCache = new SubsCache()


iskind = (entry, kind) -> _.isObject(entry.meta[kind])
iskindDatalist = (tree) -> iskind tree, 'datalist'
iskindPrograms = (tree) -> iskind tree, 'programs'
iskindJob = (tree) -> iskind tree, 'job'
iskindPackage = (tree) -> iskind tree, 'package'
iskindWorkspace = (tree) -> iskind tree, 'workspace'
iskindProgramRegistry = (tree) -> iskind tree, 'programRegistry'
iskindCatalog = (tree) -> iskind tree, 'catalog'


mayModifyRepo = (repo) ->
    aopts = {ownerName: repo.owner, repoName: repo.name}
    NogAccess.testAccess 'nog-content/modify', aopts


entryContent = (e) ->
  if e.type == 'object'
    NogContent.objects.findOne e.sha1
  else if e.type == 'tree'
    NogContent.trees.findOne e.sha1
  else
    e


# Register entry representations: object reprs and tree reprs separately.
# XXX: The nog-flow-related representation should perhaps be moved to
# nog-repr-flow, where code could be shared to implement nog-file views, in
# case such views become relevant.

Meteor.startup ->
  NogTree.registerEntryRepr
    selector: (ctx) ->
      unless ctx.last.type == 'object'
        return null
      content = ctx.last.content
      name = content.name
      if name == 'params' and iskind(content, 'program')
        'objectReprProgramParams'
      else if name == 'runtime' and iskind(content, 'program')
        'objectReprProgramRuntime'
      else
        null

isWorkspaceProgramTree = (ctx) ->
  (
    ctx.contentPath.length == 2 and
    iskindWorkspace(ctx.tree.content) and
    iskindPrograms(ctx.contentPath[0].content) and
    iskindPackage(ctx.last.content)
  )

isRegistryProgramTree = (ctx) ->
  (
    ctx.contentPath.length == 2 and
    iskindProgramRegistry(ctx.tree.content) and
    iskindPrograms(ctx.contentPath[0].content) and
    iskindPackage(ctx.last.content)
  )

Meteor.startup ->
  NogTree.registerEntryRepr
    selector: (ctx) ->
      unless ctx.last.type == 'tree'
        return null
      content = ctx.last.content
      if iskindDatalist content
        'treeReprDatalist'
      else if isWorkspaceProgramTree ctx
        'treeReprWorkspaceProgram'
      else if isRegistryProgramTree ctx
        'treeReprRegistryProgram'
      else
        null


# Provide a file scope function to reset the stable commit from anywhere until
# we have a better solution.  A possible solution would be to put the stable
# commit into the query part of the URL.  We rely on the URL for hrefs anyway.
# There can only be a single instance of treeContent at a time.  So we can
# simply use the global state directly.
treeContentInstance = null

clearStable = ->
  unless treeContentInstance
    return
  treeContentInstance.commitId.set null


Template.tree.helpers
  ownerName: -> FlowRouter.getParam('ownerName')
  repoName: -> FlowRouter.getParam('repoName')
  treeCtx: ->
    {
      ownerName: FlowRouter.getParam('ownerName')
      repoName: FlowRouter.getParam('repoName')
      refTreePath: FlowRouter.getParam('refTreePath')
    }


# Manage subscriptions explicitly in order to use `SubsCache`.
# `template.autorun` will automatically terminate when the template is
# destroyed and subscriptions inside an autorun will be automatically stopped,
# so it is nearly as good as `template.subscribe`.  But we cannot use the
# template helper `Template.subscriptionsReady`, so we manage subscriptions in
# `subs` and provide the helper `isReady`.  `subs` must be reactive in order to
# rerun `isReady` when the subscriptions change.
Template.treeContent.onCreated ->
  treeContentInstance = @
  @contextKey = null
  @commitId = new ReactiveVar null
  @subs = new ReactiveVar []

  unless (ownerName = FlowRouter.getParam('ownerName'))?
    return
  unless (repoName = FlowRouter.getParam('repoName'))?
    return

  if (repo = NogContent.repos.findOne({owner: ownerName, name: repoName}))?
    repoId = repo._id
    NogRepoSettings.call.addToRecentRepos {repoId}, (err, res) ->
      if err
        return defaultErrorHandler err

  @autorun =>
    subs = []
    subs.push subsCache.subscribe 'targetDatalists'
    subs.push subsCache.subscribe 'targetProgramWorkspaces'
    subs.push subsCache.subscribe 'targetProgramRegistries'
    data = Template.currentData()
    ownerName = data.ownerName
    repoName = data.repoName
    refTreePath = data.refTreePath
    subs.push subsCache.subscribe 'repoWithRefTreePath',
      repo:
        owner: ownerName
        name: repoName
      refTreePath: refTreePath
    @subs.set subs

    repo = NogContent.repos.findOne {owner: ownerName, name: repoName}
    unless repo?
      return
    unless (resolved = NogContent.resolveRefTreePath repo, refTreePath)?
      return

    # Reset stable commit id if the repo or ref changes.
    key = [ownerName, repoName, resolved.ref].join '/'
    if key != @contextKey
      @contextKey = key
      @commitId.set null

    # Do not capture the commit id when browsing by id, since there cannot be
    # updates.
    if resolved.refType is 'id'
      @commitId.set null
      return

    # When browsing by branch name, capture the current commit id for the
    # change warning.
    if not (@commitId.get())?
      {commitId} = resolved
      @commitId.set commitId


Template.treeContent.events
  'click .js-latest': (ev, tpl) ->
    ev.preventDefault()
    tpl.commitId.set null

  'click .js-previous': (ev, tpl) ->
    ev.preventDefault()
    params =
      ownerName: @repo.owner
      repoName: @repo.name
      refTreePath: tpl.commitId.get() + '/' + @treePath
    if @last.type is 'tree'
      FlowRouter.go 'repoTree', params
    else
      FlowRouter.go 'repoObject', params


Template.treeContent.helpers
  isReady: ->
    tpl = Template.instance()
    _.all tpl.subs.get(), (s) -> s.ready()

  reprTemplate: ->
    NogTree.selectEntryRepr(@) ? 'entryReprDefault'

  failResolveReason: ->
    tpl = Template.instance()
    ownerName = @ownerName
    repoName = @repoName
    sel = {owner: ownerName, name: repoName}
    if NogContent.repos.findOne sel
      return {pathResolveFailed: true}
    sel = {oldFullNames: "#{ownerName}/#{repoName}"}
    if (r = NogContent.repos.findOne sel)?
      return {repoIsRenamed: true, newFullName: r.fullName}
    return {repoIsUnknown: true}

  resolvedPath: ->
    tpl = Template.instance()
    ownerName = @ownerName
    repoName = @repoName
    repoSel = {owner: ownerName, name: repoName}
    unless (repo = NogContent.repos.findOne repoSel)?
      return null
    refTreePath = @refTreePath
    NogContent.resolveRefTreePath repo, refTreePath

  refHasChanged: ->
    tpl = Template.instance()
    prev = tpl.commitId.get()
    (prev? and @commitId != prev)

  rootHref: ->
    href = routerPath 'repoTree',
      ownerName: @repo.owner
      repoName: @repo.name
      refTreePath: @ref
    {href, name: @repo.name}

  pathHrefs: ->
    [initial..., last] = @namePath
    hrefs = []
    path = ''
    for name in initial
      path += "/#{name}"
      hrefs.push
        name: name
        href: routerPath 'repoTree',
          ownerName: @repo.owner
          repoName: @repo.name
          refTreePath: @ref + path
    if last?
      hrefs.push {name: last}
    hrefs

  viewerInfo: ->
    ownerName = @ownerName
    repoName = @repoName
    if (repo = NogContent.repos.findOne({owner: ownerName, name: repoName}))?
      if (commit = NogContent.commits.findOne(repo.refs['branches/master']))?
        if (tree = NogContent.trees.findOne(commit.tree))?
          refTreePath = FlowRouter.getParam('refTreePath') ? ''
          resolved = NogContent.resolveRefTreePath repo, refTreePath
          {
            fullName: repo.fullName
            type: resolved.last.type
            treePath: refTreePath.replace(/^master\/?/,'')
            iskindWorkspace: iskindWorkspace(tree)
            currentIsTechnical: true
            iskindCatalog: iskindCatalog(tree)
          }

Template.entryReprDefault.helpers
  isTree: ->
    @last.type is 'tree'

  isObject: ->
    @last.type is 'object'


Template.treeEntriesWithInlineMarkdown.helpers
  resolvedInlineObject: ->
    {repo, refTreePath} = @
    for p in ['index.md', 'README.md', 'summary.md', 'report.md']
      if (child = NogContent.resolveRefTreePath repo, refTreePath + '/' + p)?
        return child
    return null


Template.treeReprDatalist.helpers
  mayUpload: ->
    aopts = {ownerName: @repo.owner, repoName: @repo.name}
    NogAccess.testAccess 'nog-content/modify', aopts


# Manage the active info tab as global session state to maintain it when
# browsing to other repos.
#
# See <http://getbootstrap.com/javascript/#tabs-events>
Session.setDefault 'treeInfoTabs.current', 'summary'

Template.treeInfoTabs.events
  'shown.bs.tab': (ev) ->
    tabId = $(ev.target).attr('href')[1..]
    Session.set 'treeInfoTabs.current', tabId

Template.treeInfoTabs.helpers
  description: -> @last.content.meta.description
  summaryActive: -> treeInfoTabsActive 'summary'
  metaActive: -> treeInfoTabsActive 'meta'
  historyActive: -> treeInfoTabsActive 'history'
  iskindJob: -> iskindJob @last.content


treeInfoTabsActive = (tabId) ->
  if Session.equals 'treeInfoTabs.current', tabId
    'active'
  else
    null


Template.commitInfo.helpers
  commitHref: ->
    routeParams =
      ownerName: @repo.owner
      repoName: @repo.name
      refTreePath: @commit._id
    return {
      subject: @commit.subject
      shortId: @commit._id[0...10]
      href: routerPath 'repoTree', routeParams
    }

  author: ->
    @commit.authors[0]

  authorRelDate: ->
    @commit.authorDate.fromNow()


Template.refDropdown.helpers
  titlePrefix: ->
    switch @refType
      when 'id' then 'commit'
      when 'branch' then 'branch'

  title: ->
    switch @refType
      when 'id' then @ref[0...10]
      when 'branch' then @ref

  isIdRef: -> @refType == 'id'

  entries: ->
    routeName = switch @last.type
      when 'object' then 'repoObject'
      when 'tree' then 'repoTree'
    routeParams =
      ownerName: @repo.owner
      repoName: @repo.name
    for refName of @repo.refs
      [ty, name...] = refName.split '/'
      {
        name: name.join('/')
        href: routerPath routeName, _.extend routeParams, {
            refTreePath: [name, @namePath...].join('/')
          }
      }


hasKindChild = (tree, kind) ->
  for e in tree.entries
    e = entryContent(e)
    if iskind e, kind
      return true
  return false


Template.workspaceActions.helpers
  addMenuItems: ->
    items = [
        {name: 'Datalist', kind: 'datalist'}
        {name: 'Programs', kind: 'programs'}
        {name: 'Log', kind: 'log'}
      ]
    for i in items
      if hasKindChild @last.content, i.kind
        i.disabled = 'disabled'
    items


Template.workspaceActions.events
  'click .js-add': (ev) ->
    ev.preventDefault()
    repo = Template.parentData(0).repo
    opts =
      ownerName: repo.owner
      repoName: repo.name
      kind: @kind
    NogFlow.call.addWorkspaceKindTree opts, (err, res) ->
      if err
        return defaultErrorHandler err


Template.treeEntries.onCreated ->
  @isEditing = new ReactiveVar(false)
  editingCacheKey = null
  @isSelected = new ReactiveDict()
  @action = new ReactiveVar()
  @editNames = new ReactiveDict()
  @isModifiedName = new ReactiveDict()

  @selectEntry = (idx) =>
    @isSelected.set(idx, true)

  @deselectEntry = (idx) =>
    @isSelected.set(idx, false)

  @selectAllEntries = (n) =>
    for i in [0...n]
      @selectEntry i

  @deselectAllEntries = (n) =>
    for i in [0...n]
      @deselectEntry i

  @clearAllEditNames = (n) =>
    for i in [0...n]
      @editNames.set(i, null)
      @isModifiedName.set(i, false)

  @autorun =>
    # Stop editing and clear selection when navigating to a different path.
    dat = Template.currentData()
    ownerName = dat.repo.owner
    repoName = dat.repo.name
    editingCacheKey = [ownerName, repoName, dat.refTreePath].join('/')
    if editingCacheKey != @editingCacheKey
      @isEditing.set false
      @deselectAllEntries dat.last.content.entries.length
      @clearAllEditNames dat.last.content.entries.length
      @editingCacheKey = editingCacheKey


Template.treeEntries.helpers
  action: ->
    tpl = Template.instance()
    tpl.action.get()

  isEditing: ->
    tpl = Template.instance()
    return tpl.isEditing.get()

  isWorkspaceProgram: ->
    pdat = Template.parentData(1)
    return (
      iskindWorkspace(pdat.tree.content) and
      iskindPrograms(pdat.last.content) and
      iskindPackage(@content)
    )

  isProgramPackage: ->
    parent = Template.parentData(1).last.content
    return (iskindPrograms(parent) and iskindPackage(@content))

  # Rules for data dropdown:
  #
  #  - Do not show it in a program registry.
  #  - Do not show it at the first level of a workspace.
  #  - Do not show it in the `programs` subtree of a workspace.
  #  - Show it everywhere in generic repos.
  #
  shouldShowDataEntryDropdown: ->
    pdat = Template.currentData()
    if iskindProgramRegistry(pdat.tree.content)
      return false
    if iskindWorkspace(pdat.tree.content)
      if pdat.contentPath.length < 1
        return false
      if iskindPrograms(pdat.contentPath[0].content)
        return false
    return true

  mayModify: ->
    aopts = {ownerName: @repo.owner, repoName: @repo.name}
    NogAccess.testAccess 'nog-content/modify', aopts

  isEntryNameModified: ->
    tpl = Template.instance()
    tpl.isModifiedName.get(@index)

  isAnyEntryNameModified: ->
    tpl = Template.instance()
    nEntries = @last.content.entries.length
    for i in [0...nEntries]
      if tpl.isModifiedName.get(i)
        return true
    return false

  entries: ->
    tpl = Template.instance()
    pathParams =
      ownerName: @repo.owner
      repoName: @repo.name
      refTreePath: [@ref, @namePath...].join('/')
    # hrefs default to names, and use `index!` only if necessary to
    # disambiguate identical names.  `usedNames` tracks the names that have
    # been used.  If a name is encountered again, `index!` is used instead.
    usedNames = {}
    for e, idx in @last.content.entries
      switch e.type
        when 'object'
          content = NogContent.objects.findOne(e.sha1)
          icon = 'file'
          routeName = 'repoObject'
        when 'tree'
          content = NogContent.trees.findOne(e.sha1)
          icon = 'folder-close'
          routeName = 'repoTree'
      if content
        name = content.name
        if usedNames[name]?
          tail = "index!#{idx}"
        else
          tail = name
          usedNames[name] = true
        routeParams = _.clone pathParams
        routeParams.refTreePath += '/' + tail
        treePath = @treePath + '/' + tail
        {
          name
          icon
          href: routerPath routeName, routeParams
          description: content.meta.description
          type: e.type
          sha1: e.sha1
          treePath
          content
          index: idx
          isSelected: tpl.isSelected.get(idx)
        }


Template.treeEntries.events
  'click .js-toggle-entry': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.isSelected.set(@index, !tpl.isSelected.get(@index))
    if tpl.isSelected.get(@index)
        tpl.isEditing.set(true)
    else
      pdat = Template.parentData(0)
      nEntries = pdat.last.content.entries.length
      tpl.isEditing.set(false)
      for i in [0...nEntries]
        if tpl.isSelected.get(i)
          tpl.isEditing.set(true)
    if !tpl.isEditing.get()
      tpl.clearAllEditNames @last.content.entries.length

  'click .js-select-all': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.selectAllEntries @last.content.entries.length
    tpl.isEditing.set(true)

  'click .js-deselect-all': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.deselectAllEntries @last.content.entries.length
    tpl.isEditing.set(false)
    tpl.clearAllEditNames @last.content.entries.length

  'click .js-delete': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    nEntries = @last.content.entries.length
    children = []
    for i in [0...nEntries]
      if tpl.isSelected.get(i)
        children.push(i)
    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        numericPath: @numericPath
        commitId: @commit._id
        children
      }
    tpl.action.set 'deleting'
    NogFiles.call.deleteChildren opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      tpl.deselectAllEntries nEntries
      tpl.clearAllEditNames nEntries
      clearStable()

  'keyup .js-name-val': (ev) ->
    val = $(ev.target).text()
    tpl = Template.instance()
    isModified = (val != @name)
    tpl.editNames.set(@index, val)
    tpl.isModifiedName.set(@index, isModified)

  'click .js-rename': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    nEntries = @last.content.entries.length
    children = []
    for i in [0...nEntries]
      if tpl.isModifiedName.get(i)
        children.push
          index: i
          newName: tpl.editNames.get(i)
    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        numericPath: @numericPath
        commitId: @commit._id
        children
      }
    tpl.action.set 'renaming'
    NogFiles.call.renameChildren opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      tpl.deselectAllEntries nEntries
      tpl.clearAllEditNames nEntries
      clearStable()

  'click .js-starred': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    alert 'starred datalist not yet implemented'

  # The event is emitted by `dataEntryDropdown` but handled here, since it
  # requires the selection.
  'click .js-add-to': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    dat = Template.currentData()
    nEntries = dat.last.content.entries.length
    children = []
    for i in [0...nEntries]
      if tpl.isSelected.get(i)
        entry = dat.last.content.entries[i]
        children.push
          type: entry.type
          sha1: entry.sha1
    tpl.action.set 'adding'
    srcrepo = dat.repo
    opts =
      src:
        ownerName: srcrepo.owner
        repoName: srcrepo.name
        commitId: dat.commitId
        entries: children
      dst:
        ownerName: @owner
        repoName: @name
    NogFiles.call.addToDatalist opts, (err, res) ->
      if err
        tpl.action.set null
        return defaultErrorHandler err
      tpl.deselectAllEntries nEntries
      tpl.clearAllEditNames nEntries
      # Don't clearStable(), since entries are usually added to another repo.
      # We warn the user if the current repo gets modified.
      tpl.action.set 'added'
      clearOp = -> tpl.action.set null
      setTimeout clearOp, 1000

  'click .js-new-datalist': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    tpl.$('.js-new-datalist-modal').modal()

  # The event is emitted by `dataEntryDropdown` but handled here, since it
  # requires the selection.
  'click .js-create-and-add': (ev) ->
    tpl = Template.instance()
    dat = Template.currentData()
    nEntries = dat.last.content.entries.length
    children = []
    for i in [0...nEntries]
      if tpl.isSelected.get(i)
        entry = dat.last.content.entries[i]
        children.push
          type: entry.type
          sha1: entry.sha1
    name = tpl.$('.js-new-repo-name').val()
    name = sanitizedRepoName name
    tpl.action.set 'creating'
    srcrepo = dat.repo
    opts =
      src:
        ownerName: srcrepo.owner
        repoName: srcrepo.name
        commitId: dat.commitId
        entries: children
      dst:
        create: true
        ownerName: Meteor.user().username
        repoName: name
    NogFiles.call.addToDatalist opts, (err, res) ->
      tpl.$('.js-new-datalist-modal').modal('hide')
      if err
        tpl.action.set null
        return defaultErrorHandler err
      tpl.deselectAllEntries nEntries
      tpl.clearAllEditNames nEntries
      # Don't clearStable(), since entries are usually added to another repo.
      # We warn the user if the current repo gets modified.
      tpl.action.set 'created and added'
      clearOp = -> tpl.action.set null
      setTimeout clearOp, 1000


Template.treeEntriesAddFolder.onCreated ->
  @inputIsEmpty = new ReactiveVar true

Template.treeEntriesAddFolder.helpers
  inputIsEmpty: ->
    tpl = Template.instance()
    tpl.inputIsEmpty.get()

Template.treeEntriesAddFolder.events
  'click .js-addFolder-start': (ev) ->
    tpl = Template.instance()
    tpl.$('.js-addFolder-modal').modal()

  'keyup .js-addFolder-name': (ev) ->
    tpl = Template.instance()
    name = tpl.$('.js-addFolder-name').val()
    if name != ""
      tpl.inputIsEmpty.set false
    else
      tpl.inputIsEmpty.set true

  'click .js-addFolder-complete': (ev) ->
    tpl = Template.instance()
    name = tpl.$('.js-addFolder-name').val()
    opts = {
      ownerName: @repo.owner
      repoName: @repo.name
      numericPath: @numericPath
      commitId: @commit._id
      folderName: name
    }
    NogFiles.call.addSubtree opts, (err, res) ->
      if err
        return defaultErrorHandler err
    tpl.$('.js-addFolder-modal').modal('hide')


suggestRepoNameForEntry = (e) ->
  parts = []
  if (study = e.meta.study)?
    parts.push study
  if (specimen = e.meta.specimen)?
    parts.push specimen
  if parts.length == 0
    parts.push e.name.split('.')[0]
  parts.join('_')


sanitizedRepoName = (n) ->
  n = n.replace /[^a-zA-Z0-9._-]/g, '_'
  n = n.replace /__+/g, '__'
  n


Template.dataEntryDropdown.helpers
  dstWorkspaces: ->
    NogContent.repos.find
      ownerId: Meteor.userId()
      kinds: {$all: ['workspace', 'datalist']}


Template.newDatalistModal.onCreated ->
  @inputIsEmpty = new ReactiveVar true


Template.newDatalistModal.events
  'keyup .js-new-repo-name': (ev) ->
    tpl = Template.instance()
    name = tpl.$('.js-new-repo-name').val()
    if name != ""
      tpl.inputIsEmpty.set false
    else
      tpl.inputIsEmpty.set true


Template.newDatalistModal.helpers
  inputIsEmpty: ->
    tpl = Template.instance()
    tpl.inputIsEmpty.get()


Template.programPackageDropdown.onCreated ->
  @operation = new ReactiveVar null


Template.programPackageDropdown.helpers
  operation: ->
    tpl = Template.instance()
    tpl.operation.get()

  dstWorkspaces: ->
    NogContent.repos.find
      ownerId: Meteor.userId()
      kinds: {$all: ['workspace', 'programs']}

  dstRegistries: ->
    NogContent.repos.find
      ownerId: Meteor.userId()
      kinds: {$all: ['programRegistry']}


Template.programPackageDropdown.events
  'click .js-add-to-workspace': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.operation.set 'Adding...'
    pdat1 = Template.parentData(1)
    srcrepo = pdat1.repo
    entry = Template.currentData()
    opts =
      src:
        ownerName: srcrepo.owner
        repoName: srcrepo.name
        entries: [{sha1: entry.sha1}]
        commitId: pdat1.commitId
      dst:
        ownerName: @owner
        repoName: @name
    NogFiles.call.addProgram opts, (err, res) ->
      if err
        tpl.operation.set null
        return defaultErrorHandler err
      tpl.operation.set 'Added'
      clearOp = -> tpl.operation.set null
      setTimeout clearOp, 1000

  'click .js-add-to-registry': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.operation.set 'adding'
    pdat1 = Template.parentData(1)
    srcrepo = pdat1.repo
    entry = Template.currentData()
    opts =
      src:
        ownerName: srcrepo.owner
        repoName: srcrepo.name
        sha1: entry.sha1
        path: entry.treePath
        commitId: pdat1.commitId
      dst:
        ownerName: @owner
        repoName: @name
    NogFlow.call.addProgramToRegistry opts, (err, res) ->
      if err
        tpl.operation.set null
        return defaultErrorHandler err
      tpl.operation.set 'added'
      clearOp = -> tpl.operation.set null
      setTimeout clearOp, 1000


# `resolveImgSrc()` calls the server to create an S3 link, which is cached
# locally.  XXX: Consider factoring-out to a package with nog content helpers.

imgSrcs = new ReactiveDict()

NogContent.resolveImgSrc = (pathParams) ->
  hash = EJSON.stringify pathParams
  src = imgSrcs.get(hash)
  now = new Date()
  if not src? or now > src.expire
    expire = new Date()
    expire.setSeconds(expire.getSeconds() + 600)
    src = {isPlaceholder: true, expire, href: 'https://placehold.it/1x1.png'}
    imgSrcs.set(hash, src)
    Meteor.call 'resolveImgSrc', pathParams, (err, href) ->
      if href
        src.isPlaceholder = false
        src.href = href
        imgSrcs.set hash, src
  return src


Template.objectReprGeneric.helpers
  blobHref: ->
    unless (blob = @last.content.blob)?
      return
    if blob == NULL_SHA1
      return
    if (blobdoc = NogContent.blobs.findOne(blob))?
      fileSize = asXiBUnit(blobdoc.size)
    else
      fileSize = null
    return {
      blob
      name: @last.content.name
      fileSize
    }

  content: ->
    @last.content.meta.content

  previewSrc: ->
    if not (blob = @last.content.meta.preview?.blob)?
      return null
    NogContent.resolveImgSrc {
      ownerName: @repo.owner
      repoName: @repo.name
      name: @last.content.meta.preview.type ? 'png'
      blob
    }


Template.objectReprProgramParams.onCreated ->
  @inputError = new ReactiveVar()
  @action = new ReactiveVar()


Template.objectReprProgramParams.helpers
  mayModify: -> mayModifyRepo @repo
  action: -> Template.instance().action.get()
  inputError: -> Template.instance().inputError.get()

  paramsJSON: ->
    params = @last.content.meta.program.params
    EJSON.stringify params, {indent: true, canonical: true}


Template.objectReprProgramParams.events
  'click .js-save-params': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    val = tpl.$('.js-params').text()
    try
      params = JSON.parse val
    catch err
      msg = "Failed to parse JSON: #{err.message}."
      tpl.inputError.set msg
      return
    if _.isEqual(params, @last.content.meta.program.params)
      tpl.inputError.set 'Parameters unchanged.'
      return
    tpl.inputError.set null

    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        numericPath: @numericPath
        commitId: @commit._id
        params
      }
    tpl.action.set 'Saving...'
    Session.set({'blockProgramRunButton': 'Saving'})
    NogFlow.call.updateProgramParams opts, (err, res) =>
      Session.set({'blockProgramRunButton': null})
      tpl.action.set null
      if err
        return defaultErrorHandler err
      clearStable()
      if (ar = @actionRedirect)?
        FlowRouter.go ar
      else
        FlowRouter.go routerPath('repoObject', res)


Template.objectReprProgramRuntime.onCreated ->
  @inputError = new ReactiveVar()
  @action = new ReactiveVar()


Template.objectReprProgramRuntime.helpers
  mayModify: -> mayModifyRepo @repo
  action: -> Template.instance().action.get()
  inputError: -> Template.instance().inputError.get()

  runtimeJSON: ->
    runtime = @last.content.meta.program.runtime ? {}
    EJSON.stringify runtime, {indent: true, canonical: true}


Template.objectReprProgramRuntime.events
  'click .js-save-runtime': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    val = tpl.$('.js-runtime').text()
    try
      runtime = JSON.parse val
    catch err
      msg = "Failed to parse JSON: #{err.message}."
      tpl.inputError.set msg
      return
    if _.isEqual(runtime, @last.content.meta.program.runtime)
      tpl.inputError.set 'Parameters unchanged.'
      return
    tpl.inputError.set null

    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        numericPath: @numericPath
        commitId: @commit._id
        runtime
      }
    tpl.action.set 'Saving...'
    NogFlow.call.updateProgramRuntime opts, (err, res) =>
      tpl.action.set null
      if err
        return defaultErrorHandler err
      clearStable()
      if (ar = @actionRedirect)?
        FlowRouter.go ar
      else
        FlowRouter.go routerPath('repoObject', res)


Template.metaView.onCreated ->
  @inputError = new ReactiveVar()
  @action = new ReactiveVar()


Template.metaView.events
  'click .js-toggle-raw-meta': (ev, tpl) ->
    ev.preventDefault()

  'click .js-toggle-editing-meta': (ev) ->
    ev.preventDefault()
    Session.set('isRawMetaEditing', !Session.get('isRawMetaEditing'))

  'click .js-save-meta': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    text = tpl.$('.js-meta-text').text()
    try
      proposed = JSON.parse text
    catch err
      msg = "Failed to parse JSON: #{err.message}."
      tpl.inputError.set msg
      return
    old = @last.content.meta
    if (e = NogFlow.metaChangeViolation(old, proposed))?
      tpl.inputError.set e
      return
    tpl.inputError.set null
    if _.isEqual(old, proposed)
      return
    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        numericPath: @numericPath
        commitId: @commit._id
        meta: proposed
      }
    tpl.action.set 'Saving...'
    NogFlow.call.setMeta opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      clearStable()


Template.metaView.helpers
  isEditing: -> Session.get('isRawMetaEditing')

  inputError: ->
    tpl = Template.instance()
    tpl.inputError.get()

  action: ->
    tpl = Template.instance()
    tpl.action.get()

  meta: ->
    EJSON.stringify @last.content.meta, {indent: true, canonical: true}

  mayModify: ->
    aopts = {ownerName: @repo.owner, repoName: @repo.name}
    NogAccess.testAccess 'nog-content/modify', aopts


Template.jobInfo.onCreated ->
  @subs = new ReactiveVar []

  @autorun =>
    subs = []
    dat = Template.currentData()
    if (jobId = dat.last.content.meta?.job?.id)?
      subs.push subsCache.subscribe 'jobStatus', [jobId]
    @subs.set subs

Template.jobInfo.helpers
  isReady: ->
    tpl = Template.instance()
    _.all tpl.subs.get(), (s) -> s.ready()

  job: ->
    unless (id = @last.content.meta?.job?.id)?
      return null
    NogExec.jobs.findOne({'data.jobId': id})

  progressBarClass: ->
    switch @status
      when 'completed' then 'progress-bar-success'
      when 'failed' then 'progress-bar-danger'
      else null

  progressPct: ->
    Math.round(@progress.percent)

  showProgressPct: ->
    @progress.percent >= 15

  # `jobExecutionRepo` returns an href to the repo to which the job results
  # will be posted.
  jobExecutionRepo: ->
    pdat = Template.parentData()
    thisRepo = "#{pdat.repo.owner}/#{pdat.repo.name}"
    jobRepo = @data.workspaceRepo
    if jobRepo == thisRepo
      return null
    [ownerName, repoName] = jobRepo.split('/')
    {refTreePath} = pdat
    href = routerPath 'repoTree', {ownerName, repoName, refTreePath}
    return {
      fullName: jobRepo
      href
    }

  failure: ->
    if @failures?.length
      {
        reason: (f.reason for f in @failures).join('. ')
      }
    else if @status == 'cancelled'
      {
        reason: 'Cancelled due to lack of progress.'
      }
    else
      null


# FIXME: Notice that master has updated causes flickering.
Template.uploadToDatalist.events
  'change .js-upload-files': (e) ->
    e.preventDefault()
    dat = Template.currentData()
    for f in e.target.files
      id = NogBlob.uploadFile f, (err, res) ->
        if err
          return defaultErrorHandler err
        opts = {
          ownerName: dat.repo.owner
          repoName: dat.repo.name
          numericPath: dat.numericPath
          name: res.filename
          blob: res.sha1
        }
        _id = res._id
        NogFiles.call.addBlobToDatalist opts, (err, res) ->
          if err
            return defaultErrorHandler err
          cleanup = -> NogBlob.files.remove _id
          setTimeout cleanup, 1000
          clearStable()

Template.uploadToDatalist.helpers
  uploads: -> NogBlob.files.find()

Template.uploadToDatalist.helpers _.pick(
  NogBlob.fileHelpers, 'name', 'progressWidth', 'uploadCompleteClass',
  'sha1Progress', 'sha1', 'haveSha1'
)


Template.treeReprRegistryProgram.helpers
  name: ->
    @last.content.name

  authors: ->
    as = @last.content.meta.package?.authors ? []
    (a.name for a in as).join(', ')

  latestVersion: ->
    entryContent(@last.content.entries[0])?.name

  resolvedReadme: ->
    {repo, refTreePath} = @
    NogContent.resolveRefTreePath repo, refTreePath + '/index!0/README.md'


Template.treeReprWorkspaceProgram.helpers
  mayRunProgram: -> mayModifyRepo @repo

  name: ->
    @last.content.name

  latestVersion: ->
    entryContent(@last.content.entries[0])?.name

  resolvedReadme: ->
    {repo, refTreePath} = @
    NogContent.resolveRefTreePath(
        repo, refTreePath + '/index!0/index!0/README.md'
      )

  resolvedParams: ->
    {repo, refTreePath} = @
    resolved = NogContent.resolveRefTreePath(
        repo, refTreePath + '/index!0/params'
      )
    unless resolved?
      return
    # Redirect 'Save Parameter' back to here.
    resolved.actionRedirect = routerPath 'repoTree', {
        ownerName: repo.owner
        repoName: repo.name
        refTreePath
      }
    resolved

  resolvedRuntime: ->
    {repo, refTreePath} = @
    resolved = NogContent.resolveRefTreePath(
        repo, refTreePath + '/index!0/runtime'
      )
    unless resolved?
      return
    # Redirect 'Save Runtime Setting' back to here.
    resolved.actionRedirect = routerPath 'repoTree', {
        ownerName: repo.owner
        repoName: repo.name
        refTreePath
      }
    resolved


Template.treeReprWorkspaceProgramRunButton.onCreated ->
  @action = new ReactiveVar()

Template.treeReprWorkspaceProgramRunButton.helpers
  action: -> Template.instance().action.get()

  blocked: -> Session.get('blockProgramRunButton')

Template.treeReprWorkspaceProgramRunButton.events
  'click .js-run': (ev) ->
    ev.preventDefault()
    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        commitId: @commit._id
        sha1: @last.content._id
      }
    tpl = Template.instance()
    tpl.action.set 'Submitting Job...'
    NogFlow.call.runProgram opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      clearStable()
      FlowRouter.go routerPath('repoTree', res)


versionString = (v) ->
  if v.major?
    "#{v.major}.#{v.minor}.#{v.patch}"
  else if v.date?
    v.date
  else if v.sha1?
    v.sha1
  else
    'unknown'


upstreamVersionsCache = new ReactiveDict


getLatestUpstreamVersion = (origin) ->
  [ownerName, repoName] = origin.repoFullName.split('/')
  unless (repo = NogContent.repos.findOne {owner: ownerName, name: repoName})?
    return null
  fixedOrigin = {
    ownerName
    repoName,
    packageName: origin.name
    commitId: repo.refs['branches/master']
  }
  cacheKey = EJSON.stringify fixedOrigin, {canonical: true}
  ver = upstreamVersionsCache.get cacheKey
  unless _.isUndefined(ver)
    return ver

  getVersion = (opts) ->
    NogFlow.call.getPackageVersion opts, (err, res) ->
      if err
        console.log 'Failed to get version', opts, err
        return
      key = EJSON.stringify _.pick(
          res, 'ownerName', 'repoName', 'packageName', 'commitId'
        ), {
          canonical: true
        }
      upstreamVersionsCache.set key, res.version

  getVersion {ownerName, repoName, packageName: origin.name}
  return null


Template.treeReprWorkspaceProgramDeps.helpers
  mayUpdateDep: -> mayModifyRepo Template.parentData().repo

  deps: ->
    latest = entryContent(@last.content.entries[0])
    if latest
      pkg = latest.meta.package
      deps = {}
      for d in pkg.dependencies
        deps[d.name] = d
      for f in pkg.frozen
        version = versionString(f)
        dep = {
            name: f.name
            version
            sha1: f.sha1
          }
        if (origin = deps[f.name])?
          upstream = getLatestUpstreamVersion origin
          if upstream?
            dep.upstreamVersion = versionString upstream
            dep.upstreamSha1 = upstream.sha1
          dep.isUpdateAvailable = (
              dep.upstreamVersion? and version != dep.upstreamVersion
            )
          [ownerName, repoName] = origin.repoFullName.split('/')
          dep.origin = {
            ownerName
            repoName
            name: origin.repoFullName
            href: '' +
              '/' + origin.repoFullName +
              '/tree/master/programs/' + origin.name
          }
        dep


Template.treeReprWorkspaceProgramDepsUpdateButton.onCreated ->
  @action = new ReactiveVar()

Template.treeReprWorkspaceProgramDepsUpdateButton.helpers
  action: -> Template.instance().action.get()

Template.treeReprWorkspaceProgramDepsUpdateButton.events
  'click .js-update-dep': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    pdat = Template.parentData()
    opts =
      ownerName: pdat.repo.owner
      repoName: pdat.repo.name
      commitId: pdat.commitId
      package:
        numericPath: pdat.numericPath
      dependency:
        name: @name
        oldSha1: @sha1
        newSha1: @upstreamSha1
      origin:
        ownerName: @origin.ownerName
        repoName: @origin.repoName
    tpl.action.set 'Updating...'
    NogFlow.call.updatePackageDependency opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
