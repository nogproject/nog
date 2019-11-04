# Developing Nog App 1
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## Introduction

This document describes the setup for a local development environment for the
main application `apps/nog-app`.  Other applications from the repo require
usually only a subset of the configuration.

A reasonable alternative for some development tasks is to publish to private
nog repos on nog.zib.de and test there.  See details below.

## Minimal Setup

Ensure that all submodules are initialized.  Check with:

```bash
git submodule status --recursive
```

Ensure that all npm dependencies are installed.

You may have to configure `node-gyp` once, see
<https://github.com/nodejs/node-gyp>:

```bash
meteor npm config set python python2.7
```

When npm dependencies change:

```bash
meteor npm install
( cd apps/nog-app/meteor && meteor npm install )
( cd examples/access-testapp && meteor npm install )
( cd examples/blob-testapp && meteor npm install )
( cd examples/content-testapp && meteor npm install )
```

The Meteor application needs a settings file, which can be created from
a template in the Meteor application directory.  To quickly get a minimal,
working app, you can fill in invalid ids and credentials and use the default
`0` for all choices:

```bash
cd apps/nog-app/meteor
../../../tools/bin/gen-settings
```

Move the settings to a private subdirectory that is ignored by git:

```bash
mkdir _private/
mv settings.json _private/settings.json
```

Then start the application:

```bash
meteor npm start
open http://localhost:3000
```

You may need to tweak the settings to get a working configuration.  See below
for details.

## Complete Settings for Local Development

To get reasonably complete settings for local application development, you need
an S3 bucket and a GitHub application.  Either run `gen-settings` to create
a settings file with valid secrets or edit a previously generated JSON.

### S3

The general configuration is described in the package docs for
[nog-multi-bucket](./packages/nog-multi-bucket/README.md).

The recommended development setup is a local Docker container with Minio, which
can be automatically started and configured using Docker Compose:

```bash
docker-compose build
docker-compose up
```

Either remove `multiBucket` from `settings.json` if the default settings can be
used; or start from the following settings and adjust `endpoint` to point to
your local Docker, which may require to use the IP address of a virtual machine
if Docker does not run directly on the system.

```json
{
    "multiBucket": {
        "readPrefs": ["noglocal"],
        "writePrefs": ["noglocal"],
        "fallback": "noglocal",
        "buckets": [
            {
                "name": "noglocal",
                "endpoint": "http://localhost:10080",
                "accessKeyId": "Cdemo",
                "secretAccessKey": "Cdemosecret"
            }
        ]
    }
}
```

Alternatively, you can create and configure an AWS S3 bucket.

### GitHub Application

Register an application at GitHub <https://github.com/settings/developers>.
Example:

 - Application name: `researchdata (localhost)`
 - Homepage URL: `http://localhost:3000`
 - Authorization callback URL: same as homepage URL

### Email

You can usually ignore the email settings for local development.

## Initial Configuration

When you run `nog-app` for the first time, perform the following
initialization.  It assumes that you enabled local testing users
`optTestingUsers` in the settings.

 - Use the 'test options' toolbar to login the testing user.
 - Go to the admin panel and add the testing user to admins and users.
 - Sign out and create a second user 'nog' with a password-based account.
 - Sign out and sign in with the first testing user; add 'nog' to users.

You should have a user with admin rights and a user 'nog'.

## Nog Documentation

### Working on the Documentation on localhost

You can work on the documentation without a valid S3 setup.

With API keys for user 'nog':

```bash
./tools/nogdoc-upload/nogdoc-upload
```

View the documentation at <http://localhost:3000>.  Change and repeat.

### Working on the Documentation on nog.zib.de

With API keys for your nog.zib.de account:

```bash
NOGDOC_REPO=sprohaska/doc ./tools/nogdoc-upload/nogdoc-upload
```

View the documentation at <http://nog.zib.de>.  Change and repeat.

## Nog Packages

### Developing Packages on localhost

You need a valid S3 setup to work on Nog packages and the Nog Package Manager
`nogpm`.

Login with user 'nog'.  Download and configure API keys and a cache directory:

```bash
source ~/Desktop/apikey.sh.txt
mkdir /tmp/nogcache
export NOG_CACHE_PATH=/tmp/nogcache
```

Create repos `nog/packages` and `nog/example_programs_2015` of type 'Program
Registry' as user 'nog' through the GUI.  Share them publicly.

Then publish the base packages using `nogpm`:

```bash
cd nogpackages/nogpy
../nogpm/nogpm publish
```

```bash
cd nogpackages/nogjobpy
../nogpm/nogpm publish
```

Publishing `photo-gallery` is trickier.  It contains frozen references to sha1s
that are only available at `nog.zib.de`.  To publish it locally, you have to
change the frozen references.  Commit the changes to a separate commit, which
you never propose for master.

```bash
cd nogpackages/photo-gallery
rm -rf nogpackages
../nogpm/nogpm install --local
../nogpm/nogpm freeze
git commit -m 'LOCAL photo-gallery: Update frozen version to localhost' -- nogpackage.json
../nogpm/nogpm publish
```

This strategy should allow you to develop any nogpackage locally, simulating
a full nog installation.

## Nog Compute Jobs

### Running Compute Jobs Locally

To run Nog compute jobs locally, run `nogjobd-forever` in a separate shell as
described in the nogexecd [README.md](./tools/nogexecd/README.md).

Briefly, login as the admin user; create and download an API key for
`nogexecbot1`.  Then run:

```bash
source ~/Desktop/apikey.sh.txt
mkdir /tmp/nogcache
export NOG_CACHE_PATH=/tmp/nogcache

cd tools/nogexecd
virtualenv -p python3 virtualenv
./virtualenv/bin/pip install -r requirements.txt
npm install

./nogjobd-forever
```

The program `photo-gallery` should work as described on nog.zib.de but execute
in the separate shell where you started `nogjobd-forever`.

## Coding Style

If in doubt, follow recommendations from the Meteor guide.

CoffeeScript used to be the primary programming language and Jade the primary
templating language.  They are not anymore.

New contributions should prefer ES2015+, as available through the Meteor
ecmascript package, and HTML templates.  This combination is recommended in the
Meteor guide.  By using it, we hope to reduce the mental overhead of
translating to different languages.  We also hope that we will see fewer
problems when updating to new Meteor versions and can use the latest language
features, like module imports, without waiting for CoffeeScript to catch up.

Use module imports.

ES2015+ and CoffeeScript may be used simultaneously in a package.  This allows
transitioning on a per-file basis.

Use ESLint as suggested in the Meteor guide.  Start from the Airbnb Javascript
Style Guide.  But do not be too pedantic, since their style guide may conflict
with current practice in the Nog code base.  Reasonable suggestions how to
proceed in terms of style are appreciated, including contributions towards
a automated and comprehensive linting setup.

Flow type annotations are acceptable.  They are supported by the Meteor
ecmascript configuration.  Whether and how we use Flow is up for discussion.

A warning about build times: Build times may be substantially impacted by
a large `node_modules` subdirectory, even if it only contains dev dependencies
like ESLint or Flow; see the 1.3.4 release notes and references therein,
<https://github.com/meteor/meteor/blob/devel/History.md#v134>.  A somewhat
annoying workaround is to temporarily move `node_modules` to hide it from
Meteor and move it back when working with non-Meteor tools.

The restructuring of the code base towards module imports and npm is up for
discussion.  The answer is not as obvious as suggested in the Meteor guide.
Nog packages cannot be simply moved to an `imports/` folder, because they are
shared between several applications.  They also cannot simply be transformed
into npm packages, since the Nog packages depend on Meteor core APIs; see
Meteor guide discussion on 'Atmosphere vs. npm',
<https://guide.meteor.com/atmosphere-vs-npm.html>.  Furthermore, the details of
using local npm packages seem to be a bit tricky; see suggestion in blog post
'Build modular application with npm local modules', <https://goo.gl/Qp2gHE>.
