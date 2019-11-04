# Doc at <https://github.com/percolatestudio/meteor-migrations/>.

optGlobalReadOnly = Meteor.settings.optGlobalReadOnly

# See commit 'nog-content: Store ownerId on repos'.
Migrations.add
  version: 1
  up: NogContent.migrations.addOwnerId

# See commit 'nog-blob: Fix concurrent uploads of same blob'.
Migrations.add
  version: 2
  up: NogBlob.migrations.removeUploadInfoFromBlobs


optForceMigration = false
if optForceMigration
  Migrations._collection.update {_id: 'control'}, {$set: {locked: false}}


Meteor.startup ->
  if optGlobalReadOnly
    console.log('[app] [GRO] Skipping migrations in read-only mode.')
  else
    Migrations.migrateTo 'latest'
  Meteor.settings.public.versions ?= {}
  Meteor.settings.public.versions.db = Migrations.getVersion()
