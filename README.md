# Nog
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Introduction

Nog is a collection of experimental server programs that were developed in
a research project at ZIB.  The goal was to manage research data at the ZIB
Department of Visual Data Analysis together with cooperation partners at FU
Berlin.

The project ended in 2019.  There are no plans to continue development.

## License

Nog is licensed under the MIT license, see [LICENSE.txt](./LICENSE.txt), by the
Zuse Institute Berlin on behalf of the Nog development team.  The license
applies to all code in the Nog repository unless a different license is
explicitly stated.

## Contributing to Nog

See [CONTRIBUTING](./CONTRIBUTING.md).

## Developing Nog

See [HACKING](./HACKING.md) for developing Nog App 1, using Meteor, now with
JavaScript and React and older code also with CoffeeScript and Blaze; and
related software, like Nog packages that use Python.  The most relevant
directories are:

```
apps/
packages/
examples/
nogpackages/
```

See [HACKING-2](./HACKING-2.md) for developing Nog App 2, using Meteor with
ECMAScript and React.  The relevant directory is:

```
web/
```

See [HACKING-go](./HACKING-go.md) for developing Nog FSO backend services,
primarily in Go.  The relevant directory is:

```
backend/
```

See [HACKING-ci](./HACKING-ci.md) for continuous integration.

## Nog Packages Overview

```{.bg-warning}
2018-10 Warning: The section is incomplete and maybe outdated, too.
```

Foundations:

 - `nog-test` (see [nog-test/README](./packages/nog-test/README.md)): Testing
   infrastructure.
 - `nog-error` (see [nog-error/README](./packages/nog-error/README.md)):
   Error infrastructure.  The package contains a central list of error
   specifications, like error codes and standardized messages.  It also
   provides UI templates for displaying errors and a default error handler.
 - `nog-access` (see [nog-access/README](./packages/nog-access/README.md)):
   Central access control based on policy statements that are inspired by the
   AWS access management.  Other packages call `nog-access` to check access
   instead of implementing a custom logic.  `nog-access` contains a default
   access policy, assuming our standard role model: approved users, guests, and
   anonymous.
 - `nog-rest` (see [nog-rest/README](./packages/nog-rest/README.md)):
   Infrastructure for server-side REST APIs.  Other packages provide lists of
   actions that are plugged into `nog-rest`.  `nog-rest` uses `nog-auth` for
   authentication.
 - `nog-auth` (see [nog-auth/README](./packages/nog-auth/README.md),
   [nog-auth/apidoc](./packages/nog-auth/apidoc.md)): Signature-based
   authentication for REST API calls.

Higher-level:

 - `nog-blob` (see [nog-blob/README](./packages/nog-blob/README.md),
   [nog-blob/apidoc-blobs](./packages/nog-blob/apidoc-blobs.md), and
   [nog-blob/apidoc-upload](./packages/nog-blob/apidoc-upload.md)): Blob
   upload to AWS S3.  The package provides server and client-side Meteor
   functionality, including UI templates and REST actions that can be plugged
   into `nog-rest`.
 - `nog-s3` (see [nog-s3/README](./packages/nog-s3/README.md)): Low-level
   wrapper of the AWS SDK to provide just enough to implement `nog-blob`.
 - `nog-content` (see [nog-content/README](./packages/nog-content/README.md)
   and [nog-content/apidoc](./packages/nog-content/apidoc.md)): Git-like
   content storage.

## Application Overview

```{.bg-warning}
2018-10 Warning: The section is incomplete and maybe outdated, too.
Specifically, `backend/` is missing.
```

The main application is in `apps/nog-app`.

The Meteor application root is `apps/nog-app/meteor`.  It uses the packages
described above and additional code that is expected to be factored out to
packages at some point.

The directory `examples/` contains example and testing applications.

Applications usually need a Meteor settings file, which can be generated from
the template file `settings-template.yml` in the Meteor application root by
executing `tools/bin/gen-settings`.

## API Documentation

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
```

The API documentation is maintained in separate documents:

 - Developer API: [devdoc](./devdoc.md).
 - REST API: [apidoc](./apidoc.md).

Use `./tools/bin/gen-devdoc` to update it from the input files.

## API Evolution Strategy

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
```

The two initial use cases for API evolution were:

 - The addition of timezone support.
 - The change of `object.meta.content` to `object.text`.

We apply the following design principles:

 - Old clients must not break unexpectedly during the transition period.
 - We are willing to accept implementation complexity in the server if it helps
   to keep client code simple.  Our assumption is that the core developers are
   more likely to manage technical details correctly than our users.
 - We want to use the newer design at as many places as possible and explicitly
   fall back to the old design for compatibility.
 - Eventually, we want to remove the compatibility code.  We clearly
   communicate our intent to the users and give them time to update.

Based on these principles, here is the transition plan that we used for adding
timezone support.  We expect that future transitions would follow a similar
plan:

 - Add the new API v1 at a separate URL `/api/v1`.
 - Expose the old API v0 at a separate URL `/api/v0`.
 - Keep `/api` as an alias to `/api/v0`.
 - Implement v1 in a way that v0 remains unchanged.  Cover v0 with unit tests.
 - Modify clients (like nogpy) to use v1 explicitly.
 - Announce the transition, help users to migrate.
 - Wait, push the transition, wait.
 - Switch `/api` to `/api/v1`.  Old clients can still use `/api/v0`.
 - Wait, push the transition, wait.
 - Drop v0.

The changes affected the canonical entry format and, therefore, the way how
entry ids are computed.  To handle the transition, we used the following
approach:

 - For compatibility, new entries should be created in the oldest reasonable
   format whenever possible, so that old clients can use the new entries.
   Specifically, when interacting through the web UI, new commits should be
   created with ISO UTC Z, so that old programs can fetch the new commits.

 - The new entry format is only used when required.  Specifically, the
   one-to-one import of a git history will create commits that use timezone
   offsets.  Such repos cannot be accessed through the v0 API.

 - The mapping to the old entry format is implemented in the nog-content
   `Store` functions.

 - The mapping to old entry format will be dropped when dropping support for
   older APIs that require it.  The older format should not be dropped too
   quickly; clients need time to migrate.

 - The v1 API contains mechanisms to help handling future entry format
   evolution.

 - Entries will be delivered via the v1 API with a new field `_idversion`.  It
   is an integer that indicates the canonical representation to be used to
   compute the entry sha1.  v1 clients must check `_idversion` and report
   unknown versions as errors.  Clients should handle all older `_idversions`
   correctly forever, since they may receive old formats when accessing older
   repos.  Future format changes can be handled by increasing `_idversion`
   without introducing a new API version.  The rule applies nonetheless that
   new entries should be created with an old format for a transition period in
   order to give clients time to migrate to the new `_idversion`.

 - Low-level clients, which compute sha1s of the canonical representation,
   need the canonical representation anyway.  So it seems useful to deliver the
   canonical representation that is used to compute the sha1 by default.
   Higher-level clients, though, which do not compute sha1s, may want to
   receive all entries in a standardized format.  They can request a specific
   format with a query parameter like `?format=raw.v1`.

 - The v0 API will report an error if the entry uses a newer format, which the
   client cannot handle.  The v0 API refuses to insert entries in a new format.

 - v1 API clients can post a specific `_idversion` that they want to create.
   If the client does not specify a version, the server uses the oldest
   possible supported format for highest compatibility with older clients.

 - The `_idversion` will be added to the transformed docs when accessing
   MongoDB entries, but only if the version can be unambiguously determined
   from the set of requested fields.

 - The common API code paths are implemented in functions `*_v0_v1`.  The
   specific parts are implemented in functions `*_v0` and `*_v1`.  The main
   tests are modifies to test the latest version (and some compatibility).  We
   hope that it will be relatively easy to delete the `_v0` code paths when we
   want to drop support for the old API.  It seems reasonable to accept some
   duplication, especially in testing code, if it is obvious how to drop it
   later.

 - The old API doc will be frozen by copying it to a separate document.  The
   current API doc is then updated to describe the latest version (with
   a reference to the previous version).  The old documentation can be simply
   removed by deleting a document when the API is dropped.

We decided to keep entries with the old format and not apply any conversion.
Alternatives would be:

 - We could migrate the content of a repository to the latest format in
   a separate commit.  The information would be preserved, but the object ids
   would change.  A simple client could from this point on rely on the latest
   format.  Such a simple client would fail, however, if it traversed the
   commit history into the past.

 - We could rewrite the entire repo history (like git filter-branch).  The
   commit ids, however, would change for the entire history.

## Database Migrations

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
```

We use package `percolate:migrations`,
<https://github.com/percolatestudio/meteor-migrations/>, to manage data base
migrations.  Packages provide migrations on their API objects, for example
`NogContent.migrations.addOwnerId()`.  The apps manage migrations, usually in
`server/migrations.coffee` (see examples in `examples/content-testapp` and
`apps/nog-app/`).

It can be tricky to manage indices.  See example for a schema evolution in
commit 46421e39a57de13726b72eae1b3bce10e6af2720 'nog-content: Store ownerId on
repos'.

## Upgrading Meteor and Packages

See Git history, [LOG](./LOG.md), and [LOG-2015](./LOG-2015.md) for notes about
upgrading Meteor and packages.

## Router and Layout Architecture

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
2018-10 Warning: The section is incomplete and outdated.  Specifically, the
decision to transition to React is missing.
```

For new code, use the architecture as described in Kadira's [Routing Guide for
Meteor Apps][routing_guide].  Specifically, do no pass router params through
BlazeLayout.  The guide warns in [note][routing_data] that this may cause low
rendering performance.  Instead, get the params directly from FlowRouter as
described in the section on [accessing the URL state][routing_params].

Also check the Meteor guide and the publish generation trick in
1a9a2c3b82af5bc431a713cf715fcafdab10f354 'nog-flow: re-render only if
subscription workspaceContent has updated'.

[routing_guide]: <https://kadira.io/academy/meteor-routing-guide>
[routing_data]: <https://kadira.io/academy/meteor-routing-guide/content/rendering-blaze-templates>
[routing_params]: <https://kadira.io/academy/meteor-routing-guide/content/accessing-the-url-state>

## Testing Approach

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
```

Use `tools/bin/test-all` to run most tests automatically.  Close other browsers
windows to `localhost:3000` to avoid spurious test errors.

Use `meteor npm run test` in application directories to run per-application
tests, which may comprise only a subset of the tests.  Use `meteor npm run` to
list available test scripts.

We currently use the following approaches to testing:

 - Testing apps that use one or several packages in a full Meteor application.
   Testing apps demonstrate that the API is useful.  It may also be
   a reasonable approach to UI component testing.

 - Package tests that exercise the public API: Such tests confirm that the
   public API integrates with Meteor as expected.  But the tests may be a bit
   tedious to write, because a straightforward API often uses static state such
   as Meteor methods, templates, or settings.

 - Package tests that exercise the internal implementation: Such tests seem
   useful to test the low-level implementation in isolation without depending
   on static Meteor state.

 - End-to-end that simulate how a user operates the browser.  The approach
   could be useful to test a deployed version.

Currently used test runners are:

 - Some of the Mocha test drivers listed in the Meteor guide 1.3
   <https://guide.meteor.com/v1.3/testing.html#mocha>.   We use `meteor test`
   and `meteor test-packages`.  See right below and `meteor npm run` in
   application directories for details.  Mocha is a good option for writing
   unit tests that do not cause side-effects.  But there are limitations: Tests
   that require a sequence of actions on the server and the client are
   difficult to write.  Test side-effects might be a problem, in particular
   changing the Meteor user is not possible.  Some test drivers use PhantomJS,
   which does not support some modern HTML 5 API; specifically, we observed
   problems with `File()`.

 - Mocha with `meteor test-packages`.  Tests are declared in the Meteor
   `package.js` files.

 - Mocha with `meteor test`.  To use `meteor test` for packages, tests are
   located in a package subdir `tests/meteor` and must follow the `meteor test`
   naming conventions explained in the Meteor guide.  Subdirs `client` and
   `server` can be used to limit scope.  Tests are symlinked into an
   application as `package-tests/<package> ->
   ../packages/<package>/tests/meteor`.

 - Nightwatch: Tests are located in the subdir `tests/nightwatch` of Meteor
   apps, usually example apps that test packages.  See `example/blob-testapp`.
   Tests are executed in a real browser via Webdriver.

 - Manual testing: Manual test cases are described in `tests/manual/*.md`
   files.  The recommended structure is a short introduction that describes the
   purpose and scope, followed by steps that describe the required actions and
   the expected observations.  For an example see
   `packages/nog-blob/tests/manual/large-parallel-upload.test.md`

Tests that are currently not actively used have been moved to sub-directories
of `tests/` with suffix `-abandoned`.  We keep them for a while, probably until
we have decided which approaches to use for application-level testing in the
future.

See `tools/bin/test-all` and `meteor npm run` in application directories for
details how to run tests.

You may have to limit package tests for reliable results.  `flow-router`, for
example is known to cause dependency resolution problems with Meteor 1.2.  To
limit the packages, you can, for example, grep for nog packages that contain
tests:

    meteor test-packages ... \
        $(grep -l onTest packages/nog-*/package.js | cut -d '/' -f 2)

This filter is implemented as `meteor npm run ls-test-packages` in `nog-app`.

Test locally in production mode to confirm that the code does not accidentally
depend on debug dependencies:

    meteor run --settings _private/settings.json --production

We may consider using the following test runners:

 - Chimp.  The Meteor guide (currently 1.4) suggests Chimp for acceptance
   testing.  We briefly tried Chimp in the past and decided against it.  We
   should perhaps reconsider.

We have abandoned the idea to use the following test runners:

 - Gagarin: The Gagarin project has been inactive since 2016 and the tests
   caused unstable test runs for unknown reasons.  We stopped using Gagarin and
   completely removed it from our apps.

 - Using TinyTest instead of `practicalmeteor:mocha`.  Meteor starts with 1.3
   to move towards npm packages with Mocha as the recommended test driver.
 - StarryNight as a wrapper around Nightwatch.  See
   <https://starrynight.meteor.com/testing>.  The value of another wrapper is
   not obvious.  We prefer to use Nightwatch directly and maintain our own
   wrapper.
 - Velocity with `mike:mocha`:  It is unclear who maintains Velocity after
   Xolv.io stopped maintaining it <http://xolv.io/velocity-announcement>.  We
   also had stability issues when upgrading to Meteor 1.2.
 - `mike:mocha`:  We had stability issues when upgrading to Meteor 1.2.
   Responsiveness of the maintainer on GitHub was not as expected.  Activity on
   the GitHub project is low.
 - tinytest with `smithy:describe`: It looked promising at first.  But
   `smithy:describe` lacks useful mocha functionality like `it.skip` and
   `@timeout`, which makes it tedious to port mocha tests.  Furthermore, the
   exception handling seems brittle.

### Gotchas

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
```

Explicitly specify a version in the package file to avoid using outdated
packages.  In particular, do this for jade to avoid undefined
`Template.name...`:

```{.javascript}
api.use('mquandalle:jade@0.4.1');
```

Named collections must be instantiated with `new` only once during program
startup, because the name acts as a global identifier, for example, in method
definitions.  This limits the options for dependency injection with fake
collections during testing.  The following, for example, would fail with "A
method named '/testing/...' is already defined" during the second test run.

```{.coffee}
describe 'test', ->
  fake = null
  before ->
    fake = new Mongo.Collection 'testing'
```

When publishing a package with `nogpm publish` to a testing deployment of nog
(running on localhost) fails due to a mismatch between the sha1's in the
`frozen` section of `nogpackage.json` and the sha1 of the package that the
package to be published depends on, do the following:

```{.bash}
cd <your/package/dir>
rm -rf nogpackages
nogpm install --local # local installation of dependencies
nogpm freeze # update of sha1's in nogpackage.json
nogpm publish --registry <your/registry>
```

### Tricks

```{.bg-warning}
2019-03 Warning: The section describes only Nog App 1 but not Nog App 2.
```

Use the Meteor core package `test-helpers`.  It contains useful functions like
`renderToDiv()`.  For example:

```{.coffee}
tmpl = Template.errorDisplay
errorDisplay = $(renderToDiv tmpl).find('.nog-error-display')
expect(errorDisplay.length).to.equal 0
```

We used to use `NogTest.testingMethods()` to install Meteor testing helper
methods that allowed client-side tests to perform special testing operations
such as resetting the db; grep for more examples.

Use Meteor settings to control tests that should be disabled by default.  For
example, enable tests that access the real AWS API:

```{.json}
{
    "public": {
        "tests": {
            "aws": {
                "useRealAws": true
            }
        }
    }
}
```

Inject testing passwords through Meteor settings to avoid accidentally leaking
passwords by committing them to Git.  Example:

```{.json}
{
    "tests": {
        "passwords": {
            "user": "..."
        }
    }
}
```

Use `Meteor.loginWithPassword()` and `Meteor.logout()` with test users
(passwords from settings) to simulate a user login.  Note that this
`practicalmeteor:mocha` test reporting of concurrently running server tests.
See package `nog-access` for a possible, although not recommended, workaround.

Define a `setTimeout()` replacement with a more human-reader-friendly order:
delay first, then the continuation function:

```{.coffee}
pause = (duration_ms, fn) -> setTimeout fn, duration_ms
```

Use `pause 0, -> ...` to yield to let Meteor handle events, such as updating
the UI reactively after modifying a session variable.

Add UI elements to the testapps that allow easy testing, for example login as
different users, modify settings.  The elements can then be triggered manually
of from nightwatch.

Use `new File` on the client with a Meteor method to simulate a file upload
from the client.  Triggering an input file element is forbidden from
JavaScript.

Use a special test-only export in `package.js` to test internal functions.
Example:

```{.javascript}
api.export('NogAccessTest', 'server', {testOnly: true});
```

Use `meteor test-packages --show-test-app-path` to debug the package versions
that are used during package tests (inspect `.meteor/` at the reported path).

Sinon.JS spies and stubs (from Meteor package `practicalmeteor:sinon`) seem to
be very useful.  They can be used to hook into existing APIs during testing.
They seem to be a viable alternative to dependency injection.  In `nog-blob`,
for example, the tests use stubs to replace the low-level `S3` functions to
avoid calling the real AWS API.

## Operations Guide

See ZIB internal `nog-sup` repo.

## Developer Walkthroughs

See [devwalkthrough_2015](./devwalkthrough_2015.md) for outdated instructions
how to create an example app from scratch.  It might be better than nothing.

## Data management plan

See [DATAPLAN](./DATAPLAN.md).
