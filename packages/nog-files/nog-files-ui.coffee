{ NogFiles } = share
{defaultErrorHandler} = NogError
{asXiBUnit} = NogFmt

KEY_ENTER = 13
NULL_SHA1 = '0000000000000000000000000000000000000000'

optShowRepoToolbar = Meteor.settings?.public?.optShowRepoToolbar ? true

routerPath = (route, opts) -> FlowRouter.path route, opts


iskind = (entry, kind) -> _.isObject(entry.meta[kind])
iskindPrograms = (tree) -> iskind tree, 'programs'
iskindProgramRegistry = (tree) -> iskind tree, 'programRegistry'
iskindWorkspace = (tree) -> iskind tree, 'workspace'
iskindCatalog = (tree) -> iskind tree, 'catalog'


# XXX: factor out.
entryContent = (e) ->
  if e.type == 'object'
    NogContent.objects.findOne e.sha1
  else if e.type == 'tree'
    NogContent.trees.findOne e.sha1
  else
    e


mayModifyRepo = (repo) ->
    aopts = {ownerName: repo.owner, repoName: repo.name}
    NogFiles.testAccess 'nog-content/modify', aopts


Template.nogFiles.helpers
  ownerName: -> FlowRouter.getParam('ownerName')
  repoName: -> FlowRouter.getParam('repoName')


# XXX: We should add subscription caching.
Template.nogFilesContentLoader.onCreated ->
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
    unless (ownerName = FlowRouter.getParam('ownerName'))?
      return
    unless (repoName = FlowRouter.getParam('repoName'))?
      return
    treePath = FlowRouter.getParam('treePath') ? ''
    @subscribe 'repoWithRefTreePath', {
        repo:
          owner: ownerName
          name: repoName
        refTreePath: 'master/' + treePath
      }

Template.nogFilesContentLoader.helpers
  isReady: -> Template.instance().subscriptionsReady()

  repo: ->
    unless (ownerName = FlowRouter.getParam('ownerName'))?
      return
    unless (repoName = FlowRouter.getParam('repoName'))?
      return
    return {
      ownerName
      repoName
    }

  resolvedPath: ->
    unless (ownerName = FlowRouter.getParam('ownerName'))?
      return
    unless (repoName = FlowRouter.getParam('repoName'))?
      return
    repoSel = {owner: ownerName, name: repoName}
    unless (repo = NogContent.repos.findOne repoSel)?
      return null
    treePath = FlowRouter.getParam('treePath') ? ''
    refTreePath = 'master/' + treePath
    NogContent.resolveRefTreePath repo, refTreePath


Template.nogFilesContent.helpers
  topBarArgs: ->
    viewerInfo = {
      type: @last.type
      treePath: FlowRouter.getParam('treePath') ? ''
      iskindWorkspace: iskindWorkspace(@tree.content)
      iskindCatalog: iskindCatalog(@tree.content)
      currentIsFiles: true
    }
    return {
      nogCatalog: NogCatalog
      nogContent: NogContent
      router: FlowRouter
      viewerInfo
      ownerName: FlowRouter.getParam('ownerName')
      repoName: FlowRouter.getParam('repoName')
      meteorUser: -> return Meteor.user()
      namePath: @namePath
      makeHref: (opts) -> routerPath 'files', opts
      optShowRepoToolbar
    }

  entryViewTemplate: -> NogFiles.entryView(@) ? 'nogFilesEntryDefaultView'


Template.nogFilesObjectView.helpers
  errata: -> @last.content.errata
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

  content: -> @last.content.meta.content


Template.nogFilesEntryDefaultView.helpers
  isTree: -> @last.type is 'tree'
  isObject: -> @last.type is 'object'


Template.nogFilesEntryDefaultIcon.helpers
  icon: ->
    if @child.content.entries?
      'folder'
    else
      'file-o'


Template.nogFilesList.onCreated ->
  # `action` contains a string that describes the pending action; null if there
  # is none.
  @action = new ReactiveVar()

  # Selection handling as in Dropbox:
  #
  #  - Click selects a single row and implicitly starts a range selection.
  #  - Shift-Click updates the range selection.
  #  - Ctrl-Click adds to the selection.
  #  - Click outside clears selection.

  # The selection is maintained in the dict `selection` together with the
  # number of entries in `nEntries`.  Both are reactively updated when either
  # the content or the data context changes, which is detected based on the
  # number of entries or `contextId` (see autorun function below).
  @selection = new ReactiveDict()
  @nEntries = 0
  @contextId = null

  # `range` is used to maintain the Shift-Click range.
  @range = [0, 0]

  @isSelected = (idx) =>
    @selection.equals(idx, true)

  @getSelection = =>
    s = []
    for i in [0...@nEntries]
      if @isSelected(i)
        s.push i
    s

  @select = (idx) =>
    @selection.set(idx, true)

  @deselect = (idx) =>
    @selection.set(idx, false)

  @toggle = (idx) =>
    @selection.set(idx, !@selection.get(idx))

  @deselectRange = (rng) =>
    for i in [Math.min(rng...)..Math.max(rng...)]
      @deselect i

  @selectRange = (rng) =>
    for i in [Math.min(rng...)..Math.max(rng...)]
      @select i

  @clearSelection = =>
    for i in [0...@nEntries]
      @deselect i

  @selectOne = (idx) =>
    @clearSelection()
    @select idx

  @startSelectRange = (first) =>
    @range = [first, first]

  # XXX: The implementation could be optimized to deselect and select only the
  # diff between the old and the new range.
  @updateSelectRange = (second) =>
    @deselectRange @range
    @range[1] = second
    @selectRange @range

  # Clear selection when navigating to a different path.
  @autorun =>
    dat = Template.currentData()
    ownerName = dat.repo.owner
    repoName = dat.repo.name
    contextId = [ownerName, repoName, dat.refTreePath].join('/')
    if @nEntries != dat.last.content.entries.length
      @nEntries = dat.last.content.entries.length
      @clearSelection()
    if contextId != @contextId
      @clearSelection()
      @contextId = contextId


# Pass clicks on `<a>` to the browser.
shouldUseDefault = (ev) ->
  if ev.target.nodeName.toLowerCase() == 'a'
    true
  else
    false


shortString = (s, limit) ->
  limit ?= 30
  len = s.length
  if len <= limit
    s
  else
    n = Math.round(limit / 2) - 2
    s[0...n] + '...' + s[(len - n)...len]


Template.nogFilesList.events
  # Prevents text selection on shift click.
  'mousedown tr': (ev) ->
    if shouldUseDefault(ev)
      return
    ev.preventDefault()

  # `toggle(idx)` allows a CTRL deselect of a single entry during a
  # multi-selection.  A following SHIFT does a range select, including the
  # toggled entry, which is now reselected, which might be unexpected.  Apple,
  # for example, does a range select, excluding the deselected entry.  There is
  # no obvious perfect solution, so we keep this way instead of investing more
  # time.

  'click tr': (ev) ->
    if shouldUseDefault(ev)
      return
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    if ev.shiftKey
      tpl.updateSelectRange @index
    else if ev.ctrlKey or ev.metaKey
      tpl.toggle @index
      tpl.startSelectRange @index
    else
      tpl.selectOne @index
      tpl.startSelectRange @index

  # Clear selection when clicking outside the table.  XXX: The click is only
  # captured inside the div.row that contains the listing.  Ideally, it would
  # also be captured when clicking on blank space in other templates.  The
  # details, however, are not obvious: the click must only be handled here if
  # it has not been handled elsewhere.  So we first see whether the limited
  # capturing is good enough in practice.
  'click .js-files-list-container': (ev) ->
    if shouldUseDefault(ev)
      return
    tpl = Template.instance()
    tpl.clearSelection()

  'click .js-delete': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    dat = Template.currentData()
    opts = {
        ownerName: dat.repo.owner
        repoName: dat.repo.name
        numericPath: dat.numericPath
        commitId: dat.commit._id
        children: tpl.getSelection()
      }
    tpl.action.set 'deleting'
    NogFiles.call.deleteChildren opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      tpl.clearSelection()

  'click .js-download': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    dat = Template.currentData()
    sel = tpl.getSelection()
    content = entryContent dat.last.content.entries[sel[0]]
    tpl.action.set 'starting download of'
    # See comment in implementation of `aBlobHref` in package `nog-blob` for
    # technical details.
    sha1 = content.blob
    filename = content.name
    NogBlob.call.getBlobDownloadURL {sha1, filename}, (err, res) ->
      if err
        tpl.action.set null
        return defaultErrorHandler err
      link = document.createElement 'a'
      link.href = res
      link.download = ''
      e = document.createEvent 'MouseEvents'
      e.initEvent 'click', true, true
      link.dispatchEvent e
      # Clear action with a short delay to give the browser time to start the
      # actual download.
      clearAction = ->
        tpl.action.set null
        tpl.clearSelection()
      setTimeout clearAction, 2000

  'click .js-start-rename': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    dat = Template.currentData()
    sel = tpl.getSelection()
    content = entryContent dat.last.content.entries[sel[0]]
    Modal.show 'nogFilesRenameModal',
      name: content.name
      onRename: (newName) ->
        Modal.hide()
        tpl.action.set 'renaming'
        opts = {
            ownerName: dat.repo.owner
            repoName: dat.repo.name
            numericPath: dat.numericPath
            commitId: dat.commit._id
            children: [{index: sel[0], newName}]
          }
        NogFiles.call.renameChildren opts, (err, res) ->
          tpl.action.set null
          if err
            return defaultErrorHandler err
          tpl.clearSelection()

  'click .js-new-folder': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    dat = Template.currentData()
    Modal.show 'nogFilesNewFolderModal',
      onCreate: (folderName) ->
        Modal.hide()
        tpl.action.set 'creating new folder'
        opts = {
            ownerName: dat.repo.owner
            repoName: dat.repo.name
            numericPath: dat.numericPath
            commitId: dat.commit._id
            folderName
          }
        NogFiles.call.addSubtree opts, (err, res) ->
          tpl.action.set null
          if err
            return defaultErrorHandler err
          tpl.clearSelection()

  'click .js-upload': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    dat = Template.currentData()
    Modal.show 'nogFilesUploadModal', {
        ownerName: dat.repo.owner
        repoName: dat.repo.name
        numericPath: dat.numericPath
      }

  'click .js-move-to': (ev) ->
    ev.preventDefault()
    # Don't stopPropagation() to let bootstrap close the dropdown.
    ctxdat = @
    tpl = Template.instance()
    tdat = Template.currentData()
    opts =
      repo:
        ownerName: tdat.repo.owner
        repoName: tdat.repo.name
        commitId: tdat.commit._id
      src:
        numericPath: tdat.numericPath
        children: tpl.getSelection()
      dst:
        numericPath: ctxdat.numericPath
        index: 0
    tpl.action.set 'moving'
    NogFiles.call.moveInRepo opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      tpl.clearSelection()

  'click .js-copy-to': (ev) ->
    ev.preventDefault()
    # Don't stopPropagation() to let bootstrap close the dropdown.
    ctxdat = @
    tpl = Template.instance()
    tdat = Template.currentData()
    opts =
      repo:
        ownerName: tdat.repo.owner
        repoName: tdat.repo.name
        commitId: tdat.commit._id
      src:
        numericPath: tdat.numericPath
        children: tpl.getSelection()
      dst:
        numericPath: ctxdat.numericPath
        index: 0
    tpl.action.set 'copying'
    NogFiles.call.copyInRepo opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      tpl.clearSelection()

  'click .js-add-to-target': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    ctxdat = @
    tpl = Template.instance()
    tdat = Template.currentData()
    opts =
      src:
        ownerName: tdat.repo.owner
        repoName: tdat.repo.name
        commitId: tdat.commit._id
        entries: (tdat.last.content.entries[i] for i in tpl.getSelection())
      dst:
        ownerName: ctxdat.owner
        repoName: ctxdat.name
    tpl.action.set 'adding'
    NogFiles.call.addToDatalist opts, (err, res) ->
      if err
        tpl.action.set null
        return defaultErrorHandler err
      clearAction = ->
        tpl.action.set null
        tpl.clearSelection()
      setTimeout clearAction, 1000

  'click .js-add-program-to-target': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    tdat = Template.currentData()
    ctxdat = @
    entries = []
    for i in tpl.getSelection()
      entries.push {sha1: tdat.last.content.entries[i].sha1}
    opts =
      src:
        ownerName: tdat.repo.owner
        repoName: tdat.repo.name
        commitId: tdat.commitId
        entries: entries
      dst:
        ownerName: ctxdat.owner
        repoName: ctxdat.name
    tpl.action.set 'adding'
    NogFiles.call.addProgram opts, (err, res) ->
      if err
        tpl.action.set null
        return defaultErrorHandler err
      clearAction = ->
        tpl.action.set null
        tpl.clearSelection()
      setTimeout clearAction, 1000


Template.nogFilesList.helpers
  entries: ->
    tpl = Template.instance()
    routeParams =
      ownerName: @repo.owner
      repoName: @repo.name
    # hrefs default to names, and use `index!` only if necessary to
    # disambiguate identical names.  `usedNames` tracks the names that have
    # been used.  If a name is encountered again, `index!` is used instead.
    usedNames = {}
    for e, idx in @last.content.entries
      switch e.type
        when 'object'
          content = NogContent.objects.findOne(e.sha1)
        when 'tree'
          content = NogContent.trees.findOne(e.sha1)
      if content
        name = content.name
        if usedNames[name]?
          tail = "index!#{idx}"
        else
          tail = name
          usedNames[name] = true
        routeParams.treePath = @namePath.concat([tail]).join('/')

        if NogModal.get('addingPrograms') and
            iskindProgramRegistry(@tree.content) and
            iskindPrograms(@last.content)
          href = null
        else
          href = routerPath 'files', routeParams
        {
          parent: @
          child: {
            content
          }
          name
          description: content.meta.description
          href: href
          index: idx
          classSelected: if tpl.isSelected(idx) then 'info' else null
          errata: content.errata
        }

  entryIconTemplate: ->
    NogFiles.entryIcon(@) ? 'nogFilesEntryDefaultIcon'

  toolbarCtx: ->
    tpl = Template.instance()
    sel = tpl.getSelection()
    action = tpl.action.get()
    modalMayModify = true
    if NogModal.get('viewOnly')
      modalMayModify = false
    if (modalTargetRepo = NogModal.get('targetRepo'))?
      modalMayModify = false
      modalTargetRepo.Disabled = action? or (sel.length == 0)
    modalAddData = false
    if NogModal.get('addingData')
      modalAddData = true
    modalAddProgram = false
    if NogModal.get('addingPrograms')
      modalAddProgram = true
    modalMayModifyTarget = false
    if modalTargetRepo?
      modalMayModifyTarget = mayModifyRepo(modalTargetRepo.repo)
    modalSourcePrograms = false
    if iskindProgramRegistry(@tree.content) and iskindPrograms(@last.content)
      modalSourcePrograms = true
    permMayModify = true
    if (p = NogFiles.treePermissions(@))?
      permMayModify = p.write ? true
    ctx = {
      action
      mayModify: mayModifyRepo(@repo) && modalMayModify && permMayModify
      modalTargetRepo
      mayAddData: modalMayModifyTarget && modalAddData
      mayAddPrograms: modalMayModifyTarget && modalSourcePrograms &&
          modalAddProgram
      dropdownCtx:
        isDisabled: action? or (sel.length == 0)
        repo: @repo
        numericPath: @numericPath
        namePath: @namePath
        last: @last
    }
    switch sel.length
      when 0
        ctx.disable = {
          delete: true
          download: true
          rename: true
        }
      when 1
        if (entry = @last.content.entries[sel[0]])?
          content = entryContent entry
          summary = shortString content.name
          ctx.selection = {
            summary
          }
          ctx.disable = {
            delete: action
            download: (action or !(entry.type == 'object' and
                                   content.blob != NULL_SHA1))
            rename: action
          }
        else
          ctx.disable = {
            delete: true
            download: true
            rename: true
          }
      else
        ctx.selection = {
          summary: "#{sel.length} entries"
        }
        ctx.disable = {
          download: true
          rename: true
        }
    return ctx


Template.nogFilesBundleView.helpers
  hasEntries: -> @last.content.entries.length

  entries: ->
    entries = []
    for e in @last.content.entries
      unless (content = entryContent(e))?
        return
      entries.push {
        name: content.name
        description: content.meta.description
      }
    entries

  resolvedInlineMarkdown: ->
    # The Markdown template is defined in `nog-repr-markdown`.  Skip markdown
    # rendering if it is not available.
    unless Template.nogReprMarkdownFileView?
      return null
    {repo, refTreePath} = @
    for p in ['index.md', 'README.md', 'summary.md', 'report.md']
      if (child = NogContent.resolveRefTreePath repo, refTreePath + '/' + p)?
        return child
    return null


treeIsReadOnly = (repo, numericPath) ->
  treeCtx = NogContent.resolveRefTreePath(repo,
    ['master'].concat("index!#{i}" for i in numericPath).join('/'))
  writable = true
  if (p = NogFiles.treePermissions(treeCtx))?
    writable = p.write ? true
  !writable


Template.nogFilesMoveDropdown.helpers
  targets: ->
    indent = ''
    incIndent = ->
      indent += '&nbsp;&nbsp;'
    ts = []
    ts.push {
        name: '/' + @repo.name + '/'
        indent
        numericPath: []
        isDisabled: (@namePath.length == 0) || treeIsReadOnly(@repo, [])
      }
    incIndent()
    for p, i in @namePath
      np = @numericPath[0..i]
      ts.push {
          name: p + '/'
          indent
          numericPath: np
          isDisabled: (i == @namePath.length - 1) || treeIsReadOnly(@repo, np)
        }
      incIndent()
    for e, i in @last.content.entries
      if e.type == 'tree' and (content = entryContent e)?
        np = @numericPath.concat([i])
        ts.push {
            name: './' + content.name
            indent
            numericPath: np
            isDisabled: treeIsReadOnly(@repo, np)
          }
    ts


Template.nogFilesCopyDropdown.helpers
  targets: ->
    indent = ''
    incIndent = ->
      indent += '&nbsp;&nbsp;'
    ts = []
    ts.push {
        name: '/' + @repo.name + '/'
        indent
        numericPath: []
        isDisabled: treeIsReadOnly(@repo, [])
      }
    incIndent()
    for p, i in @namePath
      np = @numericPath[0..i]
      ts.push {
          name: p + '/'
          indent
          numericPath: np
          isDisabled: treeIsReadOnly(@repo, np)
        }
      incIndent()
    for e, i in @last.content.entries
      if e.type == 'tree' and (content = entryContent e)?
        np = @numericPath.concat([i])
        ts.push {
            name: './' + content.name
            indent
            numericPath: np
            isDisabled: treeIsReadOnly(@repo, np)
          }
    ts


Template.nogFilesRenameModal.onCreated ->
  @cannotRename = new ReactiveVar true

Template.nogFilesRenameModal.events
  'keyup .js-new-name': (ev) ->
    tpl = Template.instance()
    val = tpl.$('.js-new-name').val()
    if val == '' or val == @name
      tpl.cannotRename.set true
    else
      tpl.cannotRename.set false

  'keypress .js-new-name': (ev) ->
    if ev.which == KEY_ENTER
      ev.preventDefault()
      ev.stopImmediatePropagation()
      tpl = Template.instance()
      newName = tpl.$('.js-new-name').val()
      @onRename newName

  'click .js-rename': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    newName = tpl.$('.js-new-name').val()
    @onRename newName

Template.nogFilesRenameModal.helpers
  cannotRename: -> Template.instance().cannotRename.get()


Template.nogFilesNewFolderModal.onCreated ->
  @cannotCreate = new ReactiveVar true

Template.nogFilesNewFolderModal.events
  'keyup .js-folder-name': (ev) ->
    tpl = Template.instance()
    val = tpl.$('.js-folder-name').val()
    if val == ''
      tpl.cannotCreate.set true
    else
      tpl.cannotCreate.set false

  'keypress .js-folder-name': (ev) ->
    if ev.which == KEY_ENTER
      ev.preventDefault()
      ev.stopImmediatePropagation()
      tpl = Template.instance()
      name = tpl.$('.js-folder-name').val()
      @onCreate name

  'click .js-create': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    tpl = Template.instance()
    name = tpl.$('.js-folder-name').val()
    @onCreate name

Template.nogFilesNewFolderModal.helpers
  cannotCreate: -> Template.instance().cannotCreate.get()


Template.nogFilesUploadModal.onCreated ->
  @_errors = new ReactiveVar([])
  @_nErrors = new ReactiveVar(0)
  @_warnings = new ReactiveVar([])
  @_nWarnings = new ReactiveVar(0)

  @addError = (err) =>
    @_nErrors.set(@_nErrors.get() + 1)
    errs = @_errors.get()
    errs.push(err)
    @_errors.set(errs)

  @addWarning = (err) =>
    @_nWarnings.set(@_nWarnings.get() + 1)
    errs = @_warnings.get()
    errs.push(err)
    @_warnings.set(errs)


# See <http://www.abeautifulsite.net/whipping-file-inputs-into-shape-with-bootstrap-3/>
# for styling files input as bootstrap button.
Template.nogFilesUploadModal.events
  'change .js-files': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    dat = Template.currentData()
    {ownerName, repoName, numericPath} = dat
    for f in ev.target.files
      NogBlob.uploadFile f, {
        done: (err, res) ->
          if err
            tpl.addError(err)
            return
          opts = {
            ownerName, repoName, numericPath,
            name: res.filename, blob: res.sha1
          }
          _id = res._id
          NogFiles.call.addBlobToDatalist opts, (err, res) ->
            if err
              return defaultErrorHandler err
            cleanup = -> NogBlob.files.remove _id
            setTimeout cleanup, 2000
        onerror: (err) -> tpl.addError(err)
        onwarning: (err) -> tpl.addWarning(err)
      }
    # Replace input element to clear files list.  See
    # <http://stackoverflow.com/a/1043969>.
    el = tpl.$('.js-files')
    el.replaceWith(el.clone(true))

Template.nogFilesUploadModal.helpers
  errorCounts: ->
    tpl = Template.instance()
    return {
      nErrors: tpl._nErrors.get()
      nWarnings: tpl._nWarnings.get()
    }

  errors: -> Template.instance()._errors.get()
  clearErrorsFn: ->
    tpl = Template.instance()
    fn = -> tpl._errors.set([])
    return fn

  warnings: -> Template.instance()._warnings.get()
  clearWarningsFn: ->
    tpl = Template.instance()
    fn = -> tpl._warnings.set([])
    return fn

  uploads: -> NogBlob.files.find()
  cannotClose: -> NogBlob.files.find().count() > 0
  uploadLimit: ->
    limit = Meteor.settings.public.upload.uploadSizeLimit
    if limit == 0
      return null
    return {
      limit: fmtMemSize(limit)
    }

Template.nogFilesUploadModal.helpers _.pick(
  NogBlob.fileHelpers, 'name', 'progressWidth', 'uploadCompleteClass',
  'sha1Progress', 'sha1', 'haveSha1'
)


fmtMemSize = (s) ->
  units = [
    {size: 1024, suffix: 'KB'}
    {size: 1024 * 1024, suffix: 'MB'}
    {size: 1024 * 1024 * 1024, suffix: 'GB'}
    {size: 1024 * 1024 * 1024 * 1024, suffix: 'TB'}
  ]
  val = s
  suffix = 'Bytes'
  for u in units
    if s > u.size
      val = s / u.size
      suffix = u.suffix
  # Round down to a value that is <= the limit.
  val = Math.floor(val * 10) / 10
  return "#{val.toFixed(1)} #{suffix}"
