# See server file for details.

# Fix monkey patching of unknown package; see
# <https://github.com/vsivsi/meteor-file-sample-app/issues/2#issuecomment-120780592>
#
# FIXME: identify the package that causes the problem and consider avoiding it.
Mongo.Collection.prototype.constructor = Mongo.Collection

@NogExec =
  jobs: new JobCollection 'nogexec.jobs'
