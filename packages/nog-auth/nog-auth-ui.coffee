{ NogAuth, NogAuthTest } = share

{
  createError
  ERR_APIKEY_CREATE
  ERR_APIKEY_DELETE
} = NogError

# It works as expected in Chrome and Firefox.  Safari, however, displays the
# text instead of downloading it.
#
# Perhaps use FileSaver.js instead, see
# <https://github.com/eligrey/FileSaver.js/>.
#
# Allow the tests to disable saving.
NogAuthTest.saveKey = (key) ->
  txt = """
        # To access the API, save these settings and source them in your shell.
        #
        # To use nog.py, create a cache directory on your work filesystem,
        # and specify its path in NOG_CACHE_PATH.

        export NOG_API_URL=#{Meteor.absoluteUrl('api')}
        export NOG_USERNAME=#{key.username}
        export NOG_KEYID=#{key.keyid}
        export NOG_SECRETKEY=#{key.secretkey}

        # export NOG_CACHE_PATH='/un-comment/and/replace/this/with/local/dir'\n
        """
  datauri = 'data:text/plain;base64,' + btoa(txt)
  link = document.createElement 'a'
  link.href = datauri
  link.download = 'apikey.sh.txt'
  e = document.createEvent 'MouseEvents'
  e.initEvent 'click', true, true
  link.dispatchEvent e

Template.nogApiKeys.helpers
  # Set an `_id` on each item, so that the items remain attached to their
  # nogApiKey template instance.
  keys: ->
    user = @
    for k in user.services?.nogauth?.keys ? []
      _.extend {_id: k.keyid}, k

Template.nogApiKeys.events
  'click .js-create-apikey': (ev) ->
    ev.preventDefault()
    user = @
    NogAuth.call.createKey {keyOwnerId: user._id}, (err, res) ->
      if err?
        return NogAuth.onerror createError ERR_APIKEY_CREATE, {cause: err}
      res.username = user.username
      NogAuthTest.saveKey(res)


Template.nogApiKey.onCreated ->
  @isDeleting = new ReactiveVar false

Template.nogApiKey.helpers
  isDeleting: -> Template.instance().isDeleting.get()
  createDate: -> @createDate.toDateString()

Template.nogApiKey.events
  'click .js-delete-apikey-start': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set true

  'click .js-delete-apikey-cancel': (ev, tpl) ->
    ev.preventDefault()
    tpl.isDeleting.set false

  'click .js-delete-apikey-confirm': (ev) ->
    ev.preventDefault()
    opts =
      keyid: @keyid
      keyOwnerId: Template.parentData()._id
    NogAuth.call.deleteKey opts, (err, res) ->
      if err
        return NogAuth.onerror createError ERR_APIKEY_DELETE, {cause: err}
