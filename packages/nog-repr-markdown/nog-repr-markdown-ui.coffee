import sanitizeHtml from 'sanitize-html'


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


# Override marked renderer methods (see
# <https://github.com/chjj/marked#overriding-renderer-methods>) to add
# bootstrap table classe.
#
# Handle image links by post-processing the HTML, so that img tags that are
# already in the input are correctly handled.
#
# `opts.abspath(parts)` controls conversion of relative links to absolutes
# links.  It allows correctly resolving relative paths when markdown for child
# entries is rendered inline.  It also allows handling the different types of
# paths (objects and trees) with nog-tree.
#
# Relative links to HTML like README.html are handled by a fuzzy match in
# `resolveRefTreePath()` that tries `md` instead of `html`.
#
# Some pandoc features are handled by preprocessing:
#
#   - title `%`
#   - backslash end-of-line -> `<br>`
#
# A table of content is inserted by pre-processing the markdown with
# markdown-toc.  The links and anchors are tweaked as detailed below.
#
# mathjax is handle by the package `mrt:mathjax`.
#
# Missing features that we use with pandoc:
#
#  - Footnotes.
#  - BibTeX citations.

render = (opts) ->
  text = @last.content.text ? ''

  # Emulate Pandoc's backslash at end of line means <br>.
  text = text.replace /// \\\n ///g, '<br>\n'

  # Try pandoc `%` title block from more to less specific: First two line
  # version with byline, then without byline.
  text = text.replace(
      /// ^ % ([^\r\n]*) \r?\n % ([^\r\n]*) ///, '# $1\nby $2'
    )
  text = text.replace /// ^ % ([^\r\n]*) ///, '# $1'

  # Drop stdtool specific token.
  text = text.replace /// @@VERSIONINC@@ ///g, ''

  # Insert TOC if requested.
  text = MarkdownToc.insert(text)

  # Configure renderer with custom handlers for some tags.
  renderer = new marked.Renderer()

  renderer.heading = (text, level) ->
    # Tweak the marked anchors to match the markdown-toc targets.  Backticks
    # in `text` are already replaced by <code>...</code>, so replace the tags
    # by a single dash, as markdown-toc does for the original backtick.  Also
    # replace encoded characters like `&gt;` by a single dash.  markdown-toc
    # deletes dots but keeps multiple dashes without collapsing them into a
    # single one.
    name = text
    name = name.replace(/<\/?code>/g, '-')
    name = name.replace(/&[^;]+;/g, '-')
    name = name.replace(/[.]/g, '')
    name = name.toLowerCase()
    name = name.replace(/[^a-z0-9]/g, '-')
    return '' +
      "<h#{level} " +
        'id="' + name + '"' +
      '>' +
        text +
      "</h#{level}>"

  renderer.table = (header, body) ->
    '<table class="table">' +
      '<thead>' +
        header +
      '</thead>' +
      '<tbody>' +
        body +
      '</tbody>' +
    '</table>'

  renderer.link = (href, title, text) =>
    if isRelPath href
      if href.match /// /$ ///
        entryType = 'tree'
      else
        entryType = 'object'
      dir = _.initial(@treePath.split('/')).join('/')
      href = opts.abspath {
          repoOwner: @repo.owner
          repoName: @repo.name
          entryType
          ref: @ref
          dir
          relpath: href
        }
    return '' +
      '<a ' +
        'href="' + href + '"' +
        (if title? then ' title="' + title + '"' else '') +
      '>' +
        text +
      '</a>'

  # Enable highlight.js.
  highlight = (code, lang) ->
    unless lang
      return code
    # Extract lang from CSS style blocks like `{.coffee}`.
    if (m = lang.match /// ^ {[.] ([^}]+) } $ ///)?
      lang = m[1]
    try
      return hljs.highlight(lang, code).value
    catch err
      return code

  html = marked(text, {renderer, highlight})

  # Post-process img src using sanitizeHtml.  It is currently only used to
  # munge the src urls to point to S3.  XXX: The sanitizer should probably be
  # extended to also restrict the HTML.
  #
  # See <https://www.npmjs.com/package/sanitize-html> for sanitizer doc.

  pathParams =
    ownerName: @repo.owner
    repoName: @repo.name
    refTreePath: [@commitId, _.initial(@namePath)...].join('/')

  cfg =
    # `false` means do not filter, i.e. allow all.
    allowedTags: false
    allowedAttributes: false
    transformTags:
      img: (tagName, attribs) ->
        if (src = attribs.src)? and isRelPath(src)
          pp = _.clone pathParams
          pp.refTreePath += '/' + normalizedRelPath(src)
          attribs.src = NogContent.resolveImgSrc(pp).href
        return {tagName, attribs}

  html = sanitizeHtml(html, cfg)
  return html


Template.objectReprMarkdown.helpers
  htmlFromMarkdown: -> render.call @, {
      abspath: (p) ->
        "/#{p.repoOwner}/#{p.repoName}/#{p.entryType}/#{p.ref}" +
          (if p.dir != '' then "/#{p.dir}" else '') +
          '/' + p.relpath
    }


Template.nogReprMarkdownFileView.helpers
  htmlFromMarkdown: -> render.call @, {
      abspath: (p) ->
        "/#{p.repoOwner}/#{p.repoName}/files" +
          (if p.dir != '' then "/#{p.dir}" else '') +
          '/' + p.relpath
    }
