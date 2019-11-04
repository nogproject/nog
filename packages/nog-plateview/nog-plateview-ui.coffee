Meteor.startup ->
  NogTree.registerEntryRepr
    selector: (ctx) ->
      content = ctx.last.content
      if _.isObject(content.meta['plateView'])
        'plateView'
      else
        null


  if (p = Package['nog-files'])?
    p.NogFiles.registerEntryRepr
      icon: (entryCtx) ->
        if _.isObject(entryCtx.child.content.meta['plateView'])
          'nogPlateViewIcon'
        else
          null

      view: (treeCtx) ->
        unless treeCtx.last.type == 'tree'
          return null
        content = treeCtx.last.content
        if _.isObject(content.meta['plateView'])
          'plateView'
        else
          null

      # All trees below a plate view are read-only.
      treePermissions: (treeCtx) ->
        for p in treeCtx.contentPath
          if _.isObject(p.content.meta['plateView'])
            return {write: false}
        return null


routerPath = (route, opts) -> FlowRouter.path route, opts


calculateColor = (val,vmin,vmax) ->
  r = Math.round(255 - (val - vmin) / (vmax - vmin) * 255)
  r = Math.max(0,r)
  g = Math.round(255 - (val - vmin) / (vmax - vmin) * 255)
  g = Math.max(0,g)
  b = 255
  rgb = 'rgb('
  return rgb + r + ',' + g + ',' + b + ')'


Template.plateView.onCreated ->
  dat = Template.currentData()
  @plateparams = new ReactiveDict()
  @plateparams.set('show_param', dat.last.content.meta.plateView.show)
  @plateparams.set('show_vmin', dat.last.content.meta.plateView.vmin)
  @plateparams.set('show_vmax', dat.last.content.meta.plateView.vmax)
  @autorun =>
    data = Template.currentData()
    if data.last.type == 'tree'
      @subscribe 'plateViewTree',
        ownerName: data.repo.owner
        repoName: data.repo.name
        sha1: data.last.content._id


Template.plateView.helpers
  description: ->
    return @last.content.meta.plateView.description

  treeDataContext: ->
    EJSON.stringify @, {indent: true, canonical: true}

  platename: ->
    return @last.name

  getTableHeaders: ->
    headers = [' ']
    switch @last.content.meta.plateView.wells
      when 96   then cols = 12
      when 384  then cols = 24
      when 1536 then cols = 48
    for i in [1...cols+1] by 1
      headers.push i
    return headers

  hasResults: ->
    if _.isObject(@last.content.meta.plateView.results)
      return @last.content.meta.plateView.results.length > 0

  mayAutoscale: ->
    tpl = Template.instance()
    show_param = tpl.plateparams.get 'show_param'
    return show_param != 'name'

  getRows: ->
    tpl = Template.instance()
    pathParams =
      ownerName: @repo.owner
      repoName: @repo.name
      refTreePath: [@ref, @namePath...].join('/')
    wells = {}
    show_param = tpl.plateparams.get 'show_param'
    for e in @last.content.entries
      content = NogContent.trees.findOne(e.sha1)
      wells[content.name] = content
    switch @last.content.meta.plateView.wells
      when 96   then plateformat = {'rows': 8, 'cols': 12, 'wells': 96}
      when 384  then plateformat = {'rows': 16, 'cols': 24, 'wells': 384}
      when 1536 then plateformat = {'rows': 32, 'cols': 48, 'wells': 1536}
    rows = []
    for r in [0...plateformat.rows] by 1
      if plateformat.wells == 1536
        rname = String.fromCharCode(Math.floor(r/4)+65)
        rname = rname + String.fromCharCode((r % 4) + 65)
      else
        rname = String.fromCharCode(r+65)
      row = {'name': rname, 'entries': []}
      for c in [1...plateformat.cols+1] by 1
        cname = "00" + c
        wname = rname + cname.substr(cname.length-2,2)
        isThere = _.isObject(wells[wname])
        routeParams = _.clone pathParams
        routeParams.refTreePath += '/' + wname
        href = @last.name + '/' + wname
        wbackground = ''
        try wtooltip = wells[wname].meta.description
        catch e then wtooltip = ''
        if show_param == 'name'
          wcontent = wname
        else
          if isThere
            wcontent = wells[wname].meta.results[show_param]
            wcontent = wcontent.toPrecision(3)
            vmin = tpl.plateparams.get 'show_vmin'
            vmax = tpl.plateparams.get 'show_vmax'
            wbackground = calculateColor(wcontent,vmin,vmax)
          else
            wcontent = ''
        well = {
          'name': wname,
          'isThere': isThere,
          'content': wcontent,
          'href': href,
          'background': wbackground,
          'tooltip': wtooltip
        }
        row.entries.push well
      rows.push row
    return rows


Template.plateView.events
  'click .js-platecontent': (ev) ->
    ev.preventDefault()
    resname = ev.currentTarget.innerText
    tpl = Template.instance()
    tpl.plateparams.set('show_param', resname)

  'click .js-plate-autoscale': (ev) ->
    ev.preventDefault()
    dat = Template.currentData()
    tpl = Template.instance()
    param = tpl.plateparams.get 'show_param'
    vmin = Infinity
    vmax = -Infinity
    for e in dat.last.content.entries
      content = NogContent.trees.findOne(e.sha1)
      val = content.meta.results[param]
      vmin = Math.min(val,vmin)
      vmax = Math.max(val,vmax)
    tpl.plateparams.set('show_vmin', vmin)
    tpl.plateparams.set('show_vmax', vmax)


Template.contentDropdown.helpers
  entries: ->
    return @last.content.meta.plateView.results
