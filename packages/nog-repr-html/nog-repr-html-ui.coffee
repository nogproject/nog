import sanitizeHtml from 'sanitize-html'

# Config from scratch, tuned for pandoc output.
#
# See <https://www.npmjs.com/package/sanitize-html> for sanitizer doc.
#
# See <https://github.com/npm/marky-markdown/blob/master/lib/sanitize.js> for
# further config ideas that could be added, like YouTube iframes.

sanitizeCfgPandoc =
  allowedTags: sanitizeHtml.defaults.allowedTags.concat([
      'div', 'h1', 'h2', 'img', 'pre', 'span', 'sub', 'sup'
    ])
  nonTextTags: ['style', 'script', 'textarea', 'title', 'meta']
  allowedClasses:
    code: [
        'sourceCode',
        # Pandoc sets language classes.  We do not use them yet.  Allow them
        # nonetheless, since they seem useful in principle, and maybe we want
        # to use them later.
        'bash', 'c', 'coffee', 'coffeescript', 'cpp', 'css', 'glsl',
        'javascript', 'js', 'json', 'matlab', 'python', 'r', 'sh', 'shell',
        'xml'
      ]
    div: ['sourceCode']
    pre: ['sourceCode']
    span: [
        # The `code > span.XX` classes from `nog-repr-html-ui.less`.
        'al', 'an', 'at', 'bn', 'bu', 'cf', 'ch', 'cn', 'co', 'cv', 'do', 'dt',
        'dv', 'er', 'ex', 'fl', 'fu', 'im', 'in', 'kw', 'op', 'ot', 'pp', 'sc',
        'ss', 'st', 'va', 'vs', 'wa'
        # LaTeX
        'math', 'inline', 'display', 'citation'
      ]
    tabie: ['sourceCode']
    td: ['sourceCode', 'lineNumbers']
    tr: ['sourceCode']
  allowedAttributes:
    a: ['href', 'id', 'name', 'target']
    div: ['id']  # Id for pandoc citations.
    h1: ['id']
    h2: ['id']
    h3: ['id']
    h4: ['id']
    h5: ['id']
    h6: ['id']
    img: ['id', 'src', 'width', 'height', 'valign']
    li: ['id']  # Id for pandoc footnotes.
    pre: []
    span: []
    td: ['colspan', 'rowspan']
    th: ['colspan', 'rowspan']


isRelPath = (p) ->
  if p.match /// ^[a-z]+:// ///
    return false
  if p[0] == '/'
    return false
  if p[0] == '#'
    return false
  return true


normalizedRelPath = (p) ->
  p = p.replace /// ^./ ///, ''
  p = p.replace /// //+ /// , '/'
  p


isPandocHtml = (html) ->
  html.indexOf('meta name="generator" content="pandoc"') > -1


Template.nogReprHtmlFileView.helpers
  html: ->
    html = @last.content.text ? ''
    if isPandocHtml html
      cfg = _.clone sanitizeCfgPandoc
    else
      # XXX: We could use a stricter config for HTML that is not from Pandoc.
      # Let's keep it simple for now and use the same config.
      cfg = _.clone sanitizeCfgPandoc
    pathParams =
      ownerName: @repo.owner
      repoName: @repo.name
      refTreePath: [@commitId, _.initial(@namePath)...].join('/')
    cfg.transformTags =
      img: (tagName, attribs) ->
        if (src = attribs.src)? and isRelPath(src)
          pp = _.clone pathParams
          pp.refTreePath += '/' + normalizedRelPath(src)
          attribs.src = NogContent.resolveImgSrc(pp).href
        return {tagName, attribs}
    html = sanitizeHtml(html, cfg)
    html
