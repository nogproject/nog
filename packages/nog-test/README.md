# Package `nog-test`

The package `nog-test` provides testing infrastructure.

## Configuring the test selection

During startup, `nog-test` configures Mocha by default to exclude all tagged
test.  The tag format is `_[A-Z]+_` anywhere in a test name.

To include tests, set `Meteor.settings.public.tests.mocha` to an object
`{"grep": String, "invert": Boolean}` to specify a test filter.  Use `{"grep":
".*"}` to include all tests.

## `NogTest.testingMethods(methods)` (anywhere)

`testingMethods(methods)` calls `Meteor.methods(methods)`.  It ignores errors,
so that methods can be defined within tests that may be called repeatedly.
Example:

```{.coffee}
describe 'some test', ->
  savedAccess = null
  testingMethods
    'testing/nog-auth/disableAccessCheck': ->
      if Meteor.isServer
        savedAccess = NogAuth.access
        NogAuth.configure {access: {check: ->}}
    'testing/nog-auth/restoreAccessCheck': ->
      if Meteor.isServer
        NogAuth.configure {access: savedAccess}

  before((next) -> Meteor.call 'testing/nog-auth/disableAccessCheck', next)
  after((next) -> Meteor.call 'testing/nog-auth/restoreAccessCheck', next)

  it.client 'a test that runs without access control', (next) ->
```

## `NogTest.pause(duration_ms, fn)` (anywhere)

`pause()` is `setTimeout` with a human-friendly argument order.  Example:

```{.coffee}
describe 'some context', ->
  it 'a test', ->
    someWork()
    pause 1000, ->
      expect(...)
```
