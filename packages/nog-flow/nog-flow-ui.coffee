{defaultErrorHandler} = NogError
{escapeHtml} = NogFmt
{ NogFlow } = share

WorkspaceContent = new Mongo.Collection('workspaceContent')

routerPath = (route, opts) ->
  href = FlowRouter.path route, opts
  href = href.replace /%2F/g, '/'
  href


iskind = (entry, kind) -> _.isObject(entry.meta[kind])
iskindDatalist = (tree) -> iskind tree, 'datalist'
iskindResults = (tree) -> iskind tree, 'results'
iskindPrograms = (tree) -> iskind tree, 'programs'
iskindJob = (tree) -> iskind tree, 'job'
iskindJobs = (tree) -> iskind tree, 'jobs'
iskindResults = (tree) -> iskind tree, 'results'
iskindPackage = (tree) -> iskind tree, 'package'
iskindWorkspace = (tree) -> iskind tree, 'workspace'
iskindProgramRegistry = (tree) -> iskind tree, 'programRegistry'
iskindCatalog = (tree) -> iskind tree, 'catalog'


mayModifyRepo = (repo) ->
    aopts = {ownerName: repo.owner, repoName: repo.name}
    NogAccess.testAccess 'nog-content/modify', aopts


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


updateContent = (updated) ->
  if updated
    return  NogContent.repos.findOne({owner: ownerName, name: repoName},
      {reactive: false})


Template.workspace.helpers
  ownerName: -> FlowRouter.getParam('ownerName')
  repoName: -> FlowRouter.getParam('repoName')


Template.workspaceContent.onCreated ->
  @repo = ReactiveVar()
  @counter = 0

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
    sub = @subscribe 'workspaceContent', {ownerName, repoName}
    if sub.ready()
      token = WorkspaceContent.findOne(sub.subscriptionId)
      unless @counter == token.updates
        @counter = token.updates
        @repo.set(NogContent.repos.findOne({owner: ownerName, name: repoName},
                                           {reactive: false}))


Template.workspaceContent.helpers
  ownerName: -> FlowRouter.getParam('ownerName')
  repoName: -> FlowRouter.getParam('repoName')

  viewerInfo: ->
    if (repo = Template.instance().repo.get())?
      if (commit = NogContent.commits.findOne(repo.refs['branches/master']))?
        if (tree = NogContent.trees.findOne(commit.tree))?
          {
            fullName: repo.fullName
            type: 'tree'
            treePath: ''
            iskindWorkspace: iskindWorkspace(tree)
            currentIsWorkspace: true
            iskindCatalog: iskindCatalog(tree)
          }

  repoContext: ->
    unless (ownerName = FlowRouter.getParam('ownerName'))?
      return
    unless (repoName = FlowRouter.getParam('repoName'))?
      return
    tpl = Template.instance()
    {
      repo: tpl.repo.get()
    }


Template.workspaceRepoMasterContent.helpers
  datalistInfos: ->
    dat = Template.currentData()
    commitId = dat.repo.refs['branches/master']
    if commit = NogContent.commits.findOne(commitId)
      if tree = NogContent.trees.findOne(commit.tree)
        for e, idx in tree.entries
          if e.type == 'tree'
            if subtree = NogContent.trees.findOne(e.sha1)
              if iskindDatalist subtree
                datalist = subtree
        return {
          repo: dat.repo
          datalist: datalist
        }


  programsInfos: ->
    dat = Template.currentData()
    commitId = dat.repo.refs['branches/master']
    if commit = NogContent.commits.findOne(commitId)
      if tree = NogContent.trees.findOne(commit.tree)
        for e, idx in tree.entries
          if e.type == 'tree'
            if subtree = NogContent.trees.findOne(e.sha1)
              if iskindPrograms subtree
                programs = subtree
        return {
          repo: dat.repo
          programs: programs
        }

  jobsInfos: ->
    dat = Template.currentData()
    commitId = dat.repo.refs['branches/master']
    if commit = NogContent.commits.findOne(commitId)
      if tree = NogContent.trees.findOne commit.tree
        {
          repo: @repo
          tree: tree
        }

  resultsInfos: ->
    if commit = NogContent.commits.findOne @repo.refs['branches/master']
      if tree = NogContent.trees.findOne commit.tree
        for e, idx in tree.entries
          if e.type == 'tree'
            if subtree = NogContent.trees.findOne(e.sha1)
              if iskindResults subtree
                results = subtree
        {
          repo: @repo
          results: results
        }

  oldWorkspaceVersion: ->
    if commit = NogContent.commits.findOne @repo.refs['branches/master']
      if tree = NogContent.trees.findOne commit.tree
        for e, idx in tree.entries
          if e.type == 'tree'
            if subtree = NogContent.trees.findOne(e.sha1)
              if subtree.name == 'results' and not iskindResults subtree
                return true

  errata: ->
    unless (commit = NogContent.commits.findOne @repo.refs['branches/master'])?
      return null
    unless (tree = NogContent.trees.findOne(commit.tree))?
      return null
    return tree.errata

  isWorkspace: ->
    if commit = NogContent.commits.findOne @repo.refs['branches/master']
      if tree = NogContent.trees.findOne commit.tree
        if iskindWorkspace tree
          return true


Template.workspaceFlowData.onCreated ->
  @fileLimit = 10
  @nEntries = 0

  dat = Template.currentData()
  @filesPath = '/' + dat.repo.owner + '/' + dat.repo.name + '/files/datalist'

  @autorun =>
    if Template.currentData().datalist
      @nEntries = Template.currentData().datalist.entries.length


Template.workspaceFlowData.helpers
  mayModify: -> mayModifyRepo @repo

  hasDatalist: ->
    dat = Template.currentData()
    if dat.datalist
      return true
    else
      return false

  numberOfDataEntries: ->
    Template.instance().nEntries

  selectedFiles: ->
    tpl = Template.instance()
    dat = Template.currentData()
    count = 0
    for e, idx in dat.datalist.entries
      if count >= tpl.fileLimit
        break
      switch e.type
        when 'object'
          entry = NogContent.objects.findOne(e.sha1)
          icon = 'file-o'
          name = entry.name
        when 'tree'
          entry = NogContent.trees.findOne(e.sha1)
          icon = 'folder'
          name = entry.name
      if entry
        count += 1
        {
          icon
          name
        }

  path: ->
    tpl = Template.instance()
    return tpl.filesPath

  hasMoreFiles: ->
    tpl = Template.instance()
    if tpl.nEntries <= tpl.fileLimit
      return false
    else
      return true

  numberofShownFiles: ->
    tpl = Template.instance()
    if tpl.nEntries <= tpl.fileLimit
      return tpl.nEntries
    else
      return tpl.fileLimit

  emptyDatalist: ->
    tpl = Template.instance()
    if tpl.nEntries == 0
      return true
    else
      return false


Template.workspaceFlowData.events
  'click .js-browse-datalist': (ev) ->
    path = '/' + @repo.owner + '/' + @repo.name + '/files/datalist'
    NogModal.start path, {
      backref: FlowRouter.current().path
      title: 'View-only mode'
      viewOnly: true
    }

  'click .js-browse-add-files': (ev) ->
    repoName = @repo.owner + '/' + @repo.name
    NogModal.start '/', {
      backref: FlowRouter.current().path
      title: "Adding files to #{repoName}"
      addingData: true
      targetRepo: {owner: @repo.owner, name: @repo.name, repo: @repo}
    }

  'click .js-browse-search': (ev) ->
    repoName = @repo.owner + '/' + @repo.name
    NogModal.start '/search', {
      backref: FlowRouter.current().path
      title: "Adding files to #{repoName}"
      addingData: true
      targetRepo: {owner: @repo.owner, name: @repo.name, repo: @repo}
    }

  'click .js-upload': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    dat = Template.currentData()
    refTreePath = 'master/datalist'
    res = NogContent.resolveRefTreePath(dat.repo, refTreePath)
    Modal.show 'nogFilesUploadModal', {
        ownerName: res.repo.owner
        repoName: res.repo.name
        numericPath: res.numericPath
    }


Template.workspaceFlowPrograms.onCreated ->
  @selection = new ReactiveDict()
  @nEntries = 0

  @isSelected = (idx) =>
    @selection.equals(idx, true)

  @select = (idx) =>
    @selection.set(idx, true)

  @deselect = (idx) =>
    @selection.set(idx, false)

  @clearSelection = =>
    for i in [0...@nEntries]
      @deselect i

  @selectOne = (idx) =>
    @clearSelection()
    @select idx

  @getSelection = =>
    s = []
    for i in [0...@nEntries]
      if @isSelected(i)
        s.push i
    s

  @autorun =>
    dat = Template.currentData()
    if dat.programs
      @nEntries = dat.programs?.entries.length ? 0


Template.workspaceFlowPrograms.helpers
  programInfo: ->
    tpl = Template.instance()
    dat = Template.currentData()
    progInfo = {}
    if dat.programs
      for p, idx in dat.programs.entries
        if prog = NogContent.trees.findOne(p.sha1)
          if tpl.isSelected(idx)
            progInfo = {
                program: prog
                repo: dat.repo
                commitId: dat.repo.refs['branches/master']
              }
    progInfo

  numberOfPrograms: ->
    dat = Template.currentData()
    return dat.programs.entries.length

  programList: ->
    tpl = Template.instance()
    dat = Template.currentData()

    sel = tpl.getSelection()
    if sel.length == 0
      if Session.get("selectedProgram")?
        tpl.selectOne(Session.get("selectedProgram"))
      else
        tpl.selectOne(0)
        Session.set("selectedProgram", 0)

    if dat.programs
      progList = []
      for e, idx in dat.programs.entries
        if tree = NogContent.trees.findOne(e.sha1)
          vString = ''
          if prog = NogContent.trees.findOne(tree.entries[0].sha1)
            vPatch = prog.meta['package']['frozen'][0]['patch']
            vMinor = prog.meta['package']['frozen'][0]['minor']
            vMajor = prog.meta['package']['frozen'][0]['major']
            vString = '@' + vMajor + '.' + vMinor + '.'  + vPatch
          progList.push {
            name: tree.name,
            displayName: tree.name + vString
            index: idx,
            classSelected: if tpl.isSelected(idx) then 'info' else null
          }
      {
        entries: progList
      }

  hasProgramList: ->
    if Template.currentData().programs
      return true
    else
      return false


Template.workspaceFlowPrograms.events
  'click .js-browse-add-program': (ev) ->
    repoName = @repo.owner + '/' + @repo.name
    NogModal.start '/', {
      backref: FlowRouter.current().path
      title: "Adding program to #{repoName}"
      addingPrograms: true
      targetRepo: {owner: @repo.owner, name: @repo.name, repo: @repo}
    }

  'click .js-browse-search-program': (ev) ->
    repoName = @repo.owner + '/' + @repo.name
    NogModal.start '/search', {
      backref: FlowRouter.current().path
      title: "Adding program to #{repoName}"
      addingPrograms: true
      targetRepo: {owner: @repo.owner, name: @repo.name, repo: @repo}
    }
  'click tr': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.selectOne @index
    Session.set("selectedProgram", @index)


Template.workspaceFlowProgramsSel.helpers
  isValid: ->
    dat = Template.currentData()
    if !dat.program
      return false
    if !dat.program.meta.package
      return false
    if !dat.program.entries
      return false
    return true

  name: ->
    dat = Template.currentData()
    name = ''
    if dat.program?.name?
      name = dat.program.name
    return name

  latestVersion: ->
    dat = Template.currentData()
    if dat.program.entries[0]
      return NogContent.trees.findOne(dat.program.entries[0].sha1).name

  resolvedParams: ->
    dat = Template.currentData()
    repo = dat.repo
    refTreePath = 'master/programs/' + dat.program.name
    resolved = NogContent.resolveRefTreePath(
        repo, refTreePath + '/index!0/params'
      )
    unless resolved?
      return
    # Redirect 'Save Parameter' back to here.
    resolved.actionRedirect = routerPath 'workspace', {
        ownerName: repo.owner
        repoName: repo.name
        refTreePath
      }
    resolved

  resolvedRuntime: ->
    dat = Template.currentData()
    repo = dat.repo
    refTreePath = 'master/programs/' + dat.program.name
    resolved = NogContent.resolveRefTreePath(
        repo, refTreePath + '/index!0/runtime'
      )
    unless resolved?
      return
    # Redirect 'Save Runtime Setting' back to here.
    resolved.actionRedirect = routerPath 'workspace', {
        ownerName: repo.owner
        repoName: repo.name
        refTreePath
      }
    resolved

  resolvedReadme: ->
    dat = Template.currentData()
    repo = dat.repo
    refTreePath = 'master/programs/' + dat.program.name
    NogContent.resolveRefTreePath(
        repo, refTreePath + '/index!0/index!0/README.md'
      )

  mayRunProgram: -> mayModifyRepo @repo


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


Template.workspaceFlowProgramsSelDeps.helpers
  mayUpdateDep: ->
    mayModifyRepo Template.parentData().repo

  deps: ->
    dat = Template.currentData()
    if dat.program.entries[0]
      if latest = NogContent.trees.findOne(dat.program.entries[0].sha1)
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
                  '/files/programs/' + origin.name
            }
          dep


Template.workspaceFlowProgramsSelDepsUpdate.onCreated ->
  @action = new ReactiveVar()


Template.workspaceFlowProgramsSelDepsUpdate.helpers
  action: -> Template.instance().action.get()


Template.workspaceFlowProgramsSelDepsUpdate.events
  'click .js-update-dep': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    pdat = Template.parentData()
    refTreePath = 'master/programs/' + pdat.program.name
    resolved = NogContent.resolveRefTreePath pdat.repo, refTreePath
    opts =
      ownerName: pdat.repo.owner
      repoName: pdat.repo.name
      commitId: pdat.commitId
      package:
        numericPath: resolved.numericPath
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


Template.workspaceFlowProgramsSelParams.onCreated ->
  @inputError = new ReactiveVar()
  @action = new ReactiveVar()


Template.workspaceFlowProgramsSelParams.helpers
  mayModify: -> mayModifyRepo @repo
  action: -> Template.instance().action.get()
  inputError: -> Template.instance().inputError.get()

  # This helper passes raw html to the template as a workaround for the known
  # issue of Blaze wrongly rendering reactive contenteditables
  # (https://github.com/Swavek/contenteditable).  In our case this caused
  # multiply rendered parts of string when pressing return and delete during
  # parameter manipulation.
  editable: ->
    params = @last.content.meta.program.params
    params = EJSON.stringify params, {indent: true, canonical: true}
    params = escapeHtml params
    '<pre class="js-params" contenteditable="true">' + params + '</pre>'


Template.workspaceFlowProgramsSelParams.events
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


 Template.workspaceFlowProgramsSelRuntime.onCreated ->
  @inputError = new ReactiveVar()
  @action = new ReactiveVar()


Template.workspaceFlowProgramsSelRuntime.helpers
  mayModify: -> mayModifyRepo @repo
  action: -> Template.instance().action.get()
  inputError: -> Template.instance().inputError.get()

  # This helper passes raw html to the template as a workaround as described in
  # template 'workspaceFlowProgramsSelParams'.
  editable: ->
    runtime = @last.content.meta.program.runtime
    runtime = EJSON.stringify runtime, {indent: true, canonical: true}
    runtime = escapeHtml runtime
    '<pre class="js-runtime" contenteditable="true">' + runtime + '</pre>'


Template.workspaceFlowProgramsSelRuntime.events
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


Template.workspaceFlowProgramsSelReadme.helpers
  programName: ->
    Template.parentData().program.name


Template.workspaceFlowProgramsSelRunButton.onCreated ->
  @action = new ReactiveVar()


Template.workspaceFlowProgramsSelRunButton.helpers
  action: -> Template.instance().action.get()
  blocked: -> Session.get('blockProgramRunButton')


Template.workspaceFlowProgramsSelRunButton.events
  'click .js-run': (ev) ->
    ev.preventDefault()
    opts = {
        ownerName: @repo.owner
        repoName: @repo.name
        commitId: @repo.refs['branches/master']
        sha1: @program._id
      }
    tpl = Template.instance()
    tpl.action.set 'Submitting Job...'
    NogFlow.call.runProgram opts, (err, res) ->
      tpl.action.set null
      if err
        return defaultErrorHandler err
      clearStable()


Template.workspaceFlowJobs.onCreated ->
  Session.set("workspaceShowAllJobs", false)

  @isDeleting = new ReactiveVar(false)

  @jobIds = []
  @jobEntries = {}
  @resultJobIds = []

  @autorun =>
    @jobIds = []
    @jobEntries = {}
    @resultJobIds = []
    dat = Template.currentData()
    for entry in dat.tree.entries
      if entry.type == 'tree'
        if subtree = NogContent.trees.findOne(entry.sha1)
          if iskindJobs subtree
            for jobTree in subtree.entries
              if jobTree.type == 'tree'
                if jobEntry = NogContent.trees.findOne(jobTree.sha1)
                  if iskindJob jobEntry
                    if (jobId = jobEntry.meta?.job?.id)?
                      @jobIds.push(jobId)
                      @jobEntries[jobId] = jobEntry
          else
            if iskindResults subtree
              for resultTree in subtree.entries
                if resultTree.type == 'tree'
                  if resultEntry = NogContent.trees.findOne(resultTree.sha1)
                    if (jobId = resultEntry.meta?.jobResult?.jobId)?
                      @resultJobIds.push(jobId)
    @subscribe 'jobStatus', @jobIds


Template.workspaceFlowJobs.helpers
  jobs: ->
    templateInstance = Template.instance()
    NogExec.jobs.find {'data.jobId': {$in: Template.instance().jobIds}},
      sort: {updated:-1},
      transform: (item) =>
          jobId = item.data.jobId
          item.programName =
              templateInstance.jobEntries[jobId]?.meta?.job?.program?.name
          item

  showJob: ->
    if Session.equals('workspaceShowAllJobs', true)
      return true

    templateInstance = Template.instance()
    switch @status
      when 'running', 'ready' then return true

      when 'failed', 'cancelled'
        # Show only if job is most recent for the particular program.
        return Template.workspaceFlowJobs.isMostRecentJobOfProgram(@, templateInstance.jobEntries)

      when 'completed'
        # Show if there is result with corresponding job id
        # Show only if job is most recent for the particular program.
        return (@data.jobId in templateInstance.resultJobIds) or
        Template.workspaceFlowJobs.isMostRecentJobOfProgram(@, templateInstance.jobEntries)

      else return false

  showAllJobs: ->
    Session.equals('workspaceShowAllJobs', true)

  isDeleting: ->
    Template.instance().isDeleting.get()


Template.workspaceFlowJobs.isMostRecentJobOfProgram = (job, jobEntries) ->
  for k, entry of jobEntries
    if entry.meta?.job?.program?.name is job.programName
      if entry.meta?.job?.id?
        otherJob = NogExec.jobs.findOne {'data.jobId': entry.meta.job.id}
        if otherJob? and otherJob.updated > job.updated
          return false
  return true


Template.workspaceFlowJobs.events
  'click .js-show-all-jobs-toggle': (ev) ->
    ev.preventDefault()
    currentValue = Session.get("workspaceShowAllJobs")
    Session.set("workspaceShowAllJobs", !currentValue)

  'click .js-delete-all-jobs': (ev) ->
    ev.preventDefault()
    ev.stopImmediatePropagation()
    templateInstance = Template.instance()
    dat = Template.currentData()

    for entry,idx in dat.tree.entries
      if entry.type == 'tree'
        if subtree = NogContent.trees.findOne(entry.sha1)
          if iskindJobs subtree
            jobsTree = subtree
            numericPath = [idx]

    opts = {
        ownerName: dat.repo.owner
        repoName: dat.repo.name
        numericPath: numericPath
        commitId: dat.repo.refs['branches/master']
        children: [0...jobsTree.entries.length]
      }
    templateInstance.isDeleting.set(true)
    NogFiles.call.deleteChildren opts, (err, res) ->
      templateInstance.isDeleting.set(false)
      if err
        return defaultErrorHandler err

Template.workspaceFlowJobInfo.helpers
  progressPercent: ->
    Math.round(@progress.percent)

  jobInProgress: ->
    @status == 'running'

  jobId: ->
      @data.jobId

  lastUpdate: ->
      moment(@updated).fromNow()

  createdDate: ->
      @created

  statusClass: ->
    switch @status
      when 'completed' then 'text-success'
      when 'failed' then 'text-danger'
      when 'cancelled' then 'text-muted'
      else null

  reasonLines: ->
      @reason.split('\n')


Template.workspaceFlowResults.onCreated ->
  @selection = new ReactiveDict()
  @nEntries = 0

  @isSelected = (idx) =>
    @selection.equals(idx, true)

  @select = (idx) =>
    @selection.set(idx, true)

  @deselect = (idx) =>
    @selection.set(idx, false)

  @clearSelection = =>
    for i in [0...@nEntries]
      @deselect i

  @selectOne = (idx) =>
    @clearSelection()
    @select idx

  @getSelection = =>
    s = []
    for i in [0...@nEntries]
      if @isSelected(i)
        s.push i
    s

  @autorun =>
    dat = Template.currentData()
    if dat.results
      @nEntries = dat.results?.entries.length ? 0


Template.workspaceFlowResults.helpers
  resultsExists: ->
    tpl = Template.instance()
    hasResults = false
    unless t = @results?.entries[tpl.getSelection()]
      return
    if res = NogContent.trees.findOne(t.sha1)
      hasResult = true
    hasResult


  resultsList: ->
    tpl = Template.instance()
    dat = Template.currentData()
    sel = tpl.getSelection()
    if sel.length == 0
      if Session.get("selectedResult")?
        tpl.selectOne(Session.get("selectedResult"))
      else
        tpl.selectOne(0)
        Session.set("selectedResult", 0)

    if dat.results
      resList = []
      for e, idx in dat.results.entries
        tree = NogContent.trees.findOne(e.sha1)
        vString = ''
        if tree.meta['programVersion']
          vString = tree.meta['programVersion']
        resList.push {
          name: tree.name
          displayName: tree.name + vString
          index: idx,
          classSelected: if tpl.isSelected(idx) then 'info' else null
        }
      {
        entries: resList
      }

  resultSet: ->
    dat = Template.currentData()
    tpl = Template.instance()
    idx = tpl.getSelection()
    unless t = dat.results?.entries[idx]
      return
    children = []
    varList = []
    if res = NogContent.trees.findOne(t.sha1)
      resName = res.name
      repo = dat.repo
      resPath = 'master/results/' + resName
      varList.push {
          description: res.meta.description
          path: resPath
        }
      for v, idx in res.entries
        if v.type == 'tree'
          if variant = NogContent.trees.findOne(v.sha1)
            varList.push {
                description: variant.meta.description
                path: resPath + '/' + variant.name
              }

      id = 0
      for item in varList
        for p in ['index.md', 'README.md', 'summary.md', 'report.md']
          treePath = item.path + '/' + p
          if (child = NogContent.resolveRefTreePath repo, treePath)?
            children.push {
                name: item.path.split('/').reverse()[0]
                child: child
                description: item.description
                id: id
              }
            id = id + 1
    return {
      children: children,
      isSingleResult: children.length is 1
    }


Template.workspaceFlowResults.events
  'click .js-browse-results': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    t = @results.entries[tpl.getSelection()]
    res = NogContent.trees.findOne(t.sha1)
    path = '/' + @repo.owner + '/' + @repo.name + '/files/results/' + res.name
    NogModal.start path, {
      backref: FlowRouter.current().path
      title: 'View-only mode'
      viewOnly: true
    }

  'click tr': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    tpl.selectOne @index
    Session.set("selectedResult", @index)
