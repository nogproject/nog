{defaultErrorHandler} = NogError

Template.createRepoContent.events
  'click .js-create': (ev) ->
    ev.preventDefault()
    tpl = Template.instance()
    wsOption = tpl.$('input:radio[name=wsOption]:checked').val()
    opts =
      ownerName: tpl.$('#owner').val()
      repoName: tpl.$('#name').val()
    switch wsOption
      when 'files'
        opts.rootKinds = ['fileRepo']
        opts.subtrees = []
        href = "/#{opts.ownerName}/#{opts.repoName}/files"
      when 'analysis'
        opts.rootKinds = ['workspace']
        opts.subtrees = ['datalist', 'programs', 'jobs', 'results']
        href = "/#{opts.ownerName}/#{opts.repoName}/workspace"
      when 'registry'
        opts.rootKinds = ['programRegistry']
        opts.subtrees = ['programs']
        href = "/#{opts.ownerName}/#{opts.repoName}/files"
    NogFlow.call.createWorkspaceRepo opts, (err, res) ->
      if err
        return defaultErrorHandler err
      FlowRouter.go href
