# This code is copied from:
# https://github.com/janl/mustache.js/blob/cd06b22dabdaeffe3e4c74ee02bd492a11bbb740/mustache.js#L71
# It is under the MIT License.

entityMap = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
  '/': '&#x2F;',
  '`': '&#x60;',
  '=': '&#x3D;'
}

NogFmt.escapeHtml = (string) ->
  return String(string).replace(/[&<>"'`=\/]/g, (s) ->
    return entityMap[s]
  )
