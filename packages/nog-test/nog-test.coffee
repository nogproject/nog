NogTest = {}

# Flags in NOG_TEST_FLAGS:
#
#  - `useRealAws`: Enable tests that access the real AWS API.
#
# Transfer environment to client via settings.

if Meteor.isServer
  Meteor.settings.public.NOG_TEST_FLAGS = process.env.NOG_TEST_FLAGS

# Monkey-patch mocha if available.  Keep in sync with node module
# `nog-test-flags`.
if _.isFunction(describe ? null) and _.isFunction(it ? null)
  describe.ifRealAws = it.ifRealAws = (name, args...) ->
    flags = (Meteor.settings.public.NOG_TEST_FLAGS ? '').split(',')
    if 'useRealAws' in flags
      @(name + ' [exec since useRealAws in NOG_TEST_FLAGS]', args...)
    else
      @skip(name + ' [skip since useRealAws not in NOG_TEST_FLAGS]', args...)

# `testingMethods()` ignores errors, so that methods can be defined within
# tests that may be called repeatedly.
NogTest.testingMethods = (defs) ->
  names = (k for k of defs).join "', '"
  try
    Meteor.methods defs
    console.log "Note: defined testing methods '#{names}'."
  catch err
    console.log "
        Note: ignoring error while defining testing methods '#{names}'
        (probably already defined):
      ", err

NogTest.pause = (duration_ms, fn) -> setTimeout fn, duration_ms
