if Meteor.isServer
  return

@NogModal = {}

# XXX: ReactiveDict from Meteor 1.2 has a method `clear()`, which should allow
# us to avoid tracking the `keys` here.  But `clear()` reported undefined
# symbols with `reactive-dict@1.1.2`, so we keep tracking the `keys`
# explicitly.  See <https://github.com/meteor/meteor/issues/5530>.
#
# Maybe retry again with a later version of `reactive-dict`.
state = new ReactiveDict('NogModal.state')
keys = []


NogModal.start = (path, params) ->
  keys = []
  for k, v of params
    keys.push k
    state.set k, v
  FlowRouter.go path


NogModal.end = ->
  backref = state.get('backref')
  for k in keys
    state.set k, undefined
  FlowRouter.go backref


NogModal.get = (k) -> state.get k
NogModal.equals = (k, v) -> state.equals k, v
