/* eslint-disable react/forbid-prop-types */

import PropTypes from 'prop-types';
import React from 'react';
import mdtoc from 'markdown-toc'; // eslint-disable-line no-unused-vars
import marked from 'marked';
import hljs from 'highlight.js';
import sanitizeHtml from 'sanitize-html';

// XXX We should explicitly import CSS for highlighting here.  But `github.css`
// is currently also provided in `nog-app` via Meteor package
// `simple:highlight.js`, see GitHub
// `https://github.com/stubailo/meteor-highlight.js/blob/master/github.css`.
//
// The duplication causes problems with Meteor's build system.  When we
// eventually disable UI v1, we may have to enable the import here.
//
// ```
// import 'highlight.js/styles/github.css';
// ```

// `testMd` illustrates the Markdown syntax that should be supported.  Use it
// for manual testing during development.
//
// eslint-disable-next-line no-unused-vars
const testMd = `% Pandoc Title
% Pandoc Author
% Pandoc Date
@@VERSIONINC@@

<!-- toc -->

# Heading 1

Text with \`code\`.

## Heading 2

## Heading with < special characters < $ &

## Heading with &lt; HTML entities &lt;&lt;

Links:

* [README](./README.md)
* [index](./index.md)

line 1 with eol backslash \`<br>\` \\
line 2

Table:

| col 1 | col 2 |
| ----- | ----- |
| 1     | 2     |
| a     | b     |

JavaScript code:

${'```'}{.javascript}
function foo(a, b) {
  return a + b;
}
${'```'}

Python code:

${'```'}{.python}
import os.path

def foo(a, b):
    return os.path.join(a, b)
${'```'}
`;

// `Markdown` is based on package `nog-repr-markdown`.  See
// `../nog-repr-markdown/nog-repr-markdown-ui.coffee` for details.
//
// Some Pandoc features are handled by preprocessing the Markdown:
//
//   - title `%`
//   - backslash end-of-line -> `<br>`
//
// A table of content is inserted by pre-processing the Markdown with
// `markdown-toc`.  Links and anchors are tweaked accordingly, see `heading`
// below.
//
// Features that `nog-repr-markdown-ui` provides and are missing here:
//
//  - MathJax.
//  - Images.
//  - Relative links to HTML.
//
// Further features that we might want but are currently missing:
//
//  - Footnotes
//  - BibTeX citations.
//
function Markdown({ source }) {
  let md = source;
  // md = testMd;

  // Emulate Pandoc's backslash at end of line means `<br>`.
  md = md.replace(/\\\n/g, '<br>\n');

  // Try to replace Pandoc `%` title block from more specific to less specific:
  //
  // ```
  // % title
  // %
  // % date
  // ```
  md = md.replace(
    /^%\s*([^\r\n]*)\r?\n%\s*\r?\n%\s*([^\r\n]*)/,
    '# $1\n<i>$2</i>',
  );

  // ```
  // % title
  // % author
  // % date
  // ```
  md = md.replace(
    /^%\s*([^\r\n]*)\r?\n%\s*([^\r\n]*)\r?\n%\s*([^\r\n]*)/,
    '# $1\n<i>$2</i><br>\n<i>$3</i>',
  );

  // ```
  // % title
  // % author
  // ```
  md = md.replace(
    /^%\s*([^\r\n]*)\r?\n%\s*([^\r\n]*)/,
    '# $1\n<i>$2</i>',
  );

  // ```
  // % title
  // ```
  md = md.replace(
    /^%\s*([^\r\n]*)/,
    '# $1',
  );

  // Drop stdtools-specific token.
  md = md.replace(/@@VERSIONINC@@/g, '');

  // Insert TOC if requested.
  //
  // XXX We want:
  //
  // ```
  // md = mdtoc.insert(md);
  // ```
  //
  // But it fails with `querystring.escape is not a function`.
  // <https://github.com/jonschlinkert/markdown-toc/pull/103> may be related.
  // Simply forcing a newer version `"querystring-es3":
  // "git+https://github.com/SpainTrain/querystring-es3.git#469a3f1743d50779fa3ead77463f06e757dde25f"`
  // in `nog-app/package.json`, however, did not resolve the issue.
  // Most likely, NPM `meteor-node-stubs`
  // <https://github.com/meteor/node-stubs> needs to be tweaked, since it
  // depends on NPM `querystring-es3`.
  //
  // XXX When we fix `mdtoc` here, we should also switch to `mdtoc.slugify()`
  // in `renderer.heading` below.
  //
  // As a workaround, use `MarkdownToc` from `packages/markdown-toc`.  We do
  // not like it as a longterm solution, because it uses Meteor package
  // `cosmos:browserify-example`, which is deprecated, since Meteor 1.4 should
  // handle NPM.
  md = MarkdownToc.insert(md); // eslint-disable-line no-undef

  // See <https://github.com/chjj/marked#overriding-renderer-methods> for
  // overriding renderer methods.
  const renderer = new marked.Renderer();

  // Tweak the marked anchors to match the markdown-toc targets.  Backticks in
  // `text` are already replaced by <code>...</code>, so replace the tags by a
  // single dash, as markdown-toc does for the original backtick.  Also replace
  // encoded characters like `&gt;` by a single dash.  markdown-toc deletes
  // dots but keeps multiple dashes without collapsing them into a single one.
  //
  // XXX The heuristic isn't perfect.  It should be replaced with something
  // based on `mdtoc.slugify()` when we fix the import problem.  See above.
  renderer.heading = function heading(text, level) {
    let name = text;
    name = name.replace(/<\/?code>/g, '-');
    name = name.replace(/&[^;]+;/g, '-');
    name = name.replace(/[.]/g, '');
    name = name.toLowerCase();
    name = name.replace(/[^a-z0-9]/g, '-');
    return `<h${level} id="${name}">${text}</h${level}>`;
  };

  renderer.table = function table(header, body) {
    return `
      <table class="table">
        <thead>
          ${header}
        </thead>
        <tbody>
          ${body}
        </tbody>
      </table>
    `;
  };

  // `nog-repr-markdown-ui` uses `renderer.link` to generate links between
  // trees and objects.  But FSO docs does not support directories.  So we do
  // not need anything here.  Relative links to other `.md` files should just
  // work.

  // Enable highlight.js.  See <https://github.com/markedjs/marked#highlight>.
  // See import above for related `github.css`.
  function highlight(code, lang) {
    if (!lang) {
      return code;
    }

    // Extract lang from CSS style blocks like `{.coffee}`.
    const m = lang.match(/^{[.]([^}]+)}$/);
    const hlLang = m ? m[1] : lang;
    try {
      return hljs.highlight(hlLang, code).value;
    } catch (err) {
      return code;
    }
  }

  let html = marked(md, { renderer, highlight });

  // `nog-repr-markdown-ui` uses `sanitizeHtml` to munge image src URLs to
  // point to S3.  But FSO does not yet support images.  `sanitizeHtml` is just
  // a noop placeholder here.
  //
  // See <https://www.npmjs.com/package/sanitize-html> for sanitizer doc.
  html = sanitizeHtml(html, {
    // `false` means do not filter, i.e. allow all.
    allowedTags: false,
    allowedAttributes: false,
    // See `nog-repr-markdown-ui` for `transformTags` to convert image URLs.
  });

  return (
    // eslint-disable-next-line react/no-danger
    <div dangerouslySetInnerHTML={{ __html: html }} />
  );
}

Markdown.propTypes = {
  source: PropTypes.string.isRequired,
};

export {
  Markdown,
};
