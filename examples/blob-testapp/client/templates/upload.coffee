Template.upload.events
  'change #files': (e) ->
    e.preventDefault()
    for f in e.target.files
      console.log f
      id = NogBlob.uploadFile f, (err, res) ->
        if err
          return console.log 'failed to upload file', err
        Meteor.call 'addObject',
          name: res.filename
          blob: res.sha1
      Session.set 'currentUpload', id

Template.upload.helpers
  uploads: -> NogBlob.files.find()

currentUpload = -> Session.get('currentUpload')

Template.currentUpload.helpers
  haveUpload: -> currentUpload()?
  file: -> NogBlob.files.findOne currentUpload()

Template.currentUpload.helpers _.pick(
  NogBlob.fileHelpers, 'name', 'progressWidth', 'uploadCompleteClass'
)
