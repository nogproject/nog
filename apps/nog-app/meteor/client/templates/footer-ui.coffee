Template.footer.helpers
  optShowVersions: ->
    Meteor.settings.public?.optShowVersions ? false

  versions: ->
    vs = []
    if (app = Meteor.settings.public?.versions?.app)?
      vs.push "#{app}"
    if (db = Meteor.settings.public?.versions?.db)?
      vs.push "db-#{db}"
    if vs.length > 0
      return 'Version ' + vs.join(', ')
