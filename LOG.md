# nog -- LOG
By spr
<!--@@VERSIONINC@@-->

## 2018-10-11 How to promote towards public nog.git?

See ZIB internal `nog-sup` repo.

## 2017-07-11 Event sourcing spike

CQRS event sourcing could be useful with MongoDB and Meteor.  Commands create
events that are first atomically stored to an event journal, before they are
then applied to update the usual Meteor collections, like `Meteor.users`, in an
eventually consistent way.

The event journal could be also used to implement event streams that other
services can watch.  Polling might initially be sufficient.

A proof-of-concept is on `attic` branch
`p/event-sourcing-spike@041ffb2b79d1e6e95dceec7e73b3891b66851332`.

## 2017-01-10 Nog Git storage back end proof of concept, using GRPC and JWT

The design is described in NOE-4 and NOE-5.  The proof of concept is on `attic`
branch `p/noggitstore-spike@d4912e34668bd0dac6dfad53c99e7d4ab7c9e5e2`.

## 2016-11-04 How to gracefully shutdown behind a load balancer?

The general idea how to gracefully shutdown a Nodejs server behind a load
balancer without dropping connections is nicely explained in the blog post
'Graceful shutdown with Node.js and Kubernetes'
<https://blog.risingstack.com/graceful-shutdown-node-js-kubernetes/>, code
<https://github.com/RisingStack/kubernetes-graceful-shutdown-example>.

A spike implementation is on `attic`
`p/howto-shutdown^@7ba7a7e391cbc3a6bd4995ed14d2420ea1a0cc6c` 'shutdown SPIKE:
/healthz, startup -> ok -> shutdown on SIGTERM'.  It demonstrates how a Meteor
application can listen on SIGTERM to start a coordinated shutdown.

Another blob post
<http://joseoncode.com/2014/07/21/graceful-shutdown-in-node-dot-js/>.

Knifecycle <https://github.com/nfroidure/knifecycle> could be helpful to
managed application components during shutdown.

Termination of pods in Kubernetes
<http://kubernetes.io/docs/user-guide/pods/#termination-of-pods>.

Ingress with Kubernetes:
<https://stackpointcloud.com/community/tutorial/configure-and-manage-a-kubernetes-haproxy-ingress-controller>,
HAProxy service loadbalancer
<https://github.com/kubernetes/contrib/tree/master/service-loadbalancer>.

HAProxy health checking in the commercial doc, should apply similarly to the
open-source version,
<https://www.haproxy.com/doc/aloha/7.0/haproxy/healthchecks.html>, L7 HTTP
<https://www.haproxy.com/doc/aloha/7.0/haproxy/healthchecks.html#checking-a-http-service>,
L4 HTTP
<https://www.haproxy.com/doc/aloha/7.0/lvs/healthchecks.html#http-check>.

## 2016-10-28 Abandoned first implementation of protocols, 'p/protocol'

A first implementation of protocol workflows was not merged.  It is available
on `attic`, see `p/protocol@6a8ed9006c517827b538752ca32918d32333f05d` 'Tie
p/protocol: Initial code for adding protocols to data'.

## 2016-09-23 Safari upload reliability (spr)

Reducing the upload part size did not increase reliability:

```
diff --git a/packages/nog-blob/nog-blob-server.coffee b/packages/nog-blob/nog-blob-server.coffee
index 93f201c..3201e22 100644
--- a/packages/nog-blob/nog-blob-server.coffee
+++ b/packages/nog-blob/nog-blob-server.coffee
@@ -40,7 +40,7 @@ partParamsForSize = (size) ->
   MB = 1024 * 1024
   minPartSize = 5 * MB
   maxNParts = 10000
-  usualMaxPartSize = 100 * MB
+  usualMaxPartSize = 8 * MB
   usualNParts = 10
   maxSize = maxNParts * 5 * 1000 * MB
```

## 2016-09-16 Rusha is broken for files >= 2 GiB

Rusha is broken for files >= 2 GiB.

Confirmed with:

```
$ dd if=/dev/zero bs=1024 count=$(( 3 * 1024 * 1024 )) of=zero-3GiB

$ sha1sum zero-3GiB
6e7f6dca8def40df0b21f58e11c1a41c3e000285  zero-3GiB

## Sha1 during upload with Safari:
8c1ac19eb79337e919227c5eebe8a29cc6799d95

## -> mismatch.
```

```
$ dd if=/dev/zero bs=1024 count=$(( 2 * 1024 * 1024 )) of=zero-2GiB

$ sha1sum zero-2GiB
91d50642dd930e9542c39d36f0516d45f4e1af0d  zero-2GiB

## Sha1 during upload with Safari:
ccacfe767bef64b5fac30dfb60f9a00a800a9894

## -> mismatch.
```

```
$ dd if=/dev/zero bs=1024 count=$(( 2 * 1024 * 1024 - 1)) of=zero-sub2GiB

$ sha1sum zero-sub2GiB
63544aa729351c769d7d15d8fd85a0ed9ba5d3fd  zero-sub2GiB

## Sha1 during upload with Safari:
63544aa729351c769d7d15d8fd85a0ed9ba5d3fd

## -> ok.
```

## 2016-09-02 Meteor packages and Npm dependencies (spr)

Using Npm directly via `package.json` and indirectly via `package.js` may cause
problems during package loading.  Specifically, I used `package.json` to manage
Eslint and `package.js` to require the AWS SDK.  Meteor seems to have problems
with managing the `node_modules` folder.  It tried to require AWS from a folder
`node_modules1`, because `node_modules` was already taken by the direct use of
Npm.  The require failed:

```
Error: Cannot find module 'xmlbuilder'
    ...
    at Object.<anonymous> (.../.meteor/local/isopacks/nog-multi-bucket/npm/node_modules1/aws-sdk/lib/xml/builder.js:2:15)
    ...
    at Module.Mp.load (.../.meteor/packages/babel-compiler/.6.8.5.18s3icv++os+web.browser+web.cordova/npm/node_modules/reify/node/runtime.js:16:23)
    ...
```

There are a few potential solutions:

NPM-DEPS: Install aws-sdk via normal Npm without declaring it in `package.js`.
`npm install` would need to be executed before the Meteor build.

NO-NPM: Avoid the direct `node_modules` folder, either by not using it at all
or by removing it for Meteor builds.  The primary reason for `node_modules` on
the package level currently is ESLint.  But ESLint could be handled at the root
level, which seems to be a more sensible approach anyway.

NPM-PEER: Use Meteor package peer dependencies.  The package would not declare
a dependency on aws-sdk at all but only check its existence during runtime; see
'Peer npm dependencies' in the Meteor guide
<https://guide.meteor.com/writing-atmosphere-packages.html#peer-npm-dependencies>.
`package-tests` cannot be used anymore.  A full app is instead required with
the expected Npm dependencies; see 'Testing with peer npm dependencies' in the
Meteor guide
<https://guide.meteor.com/writing-atmosphere-packages.html#testing-with-peer-dependencies>.

This seems to be feasible: We run tests through apps already.  Handling Npm
dependencies at the app level might be a good idea anyway, since it forces us
to agree on a single version of a Npm package per app, which seems to be a good
idea anyway.

NPM: Start writing Npm packages instead of Meteor packages.  This choice is not
obvious.  It is unclear how to keep the convenience of Meteor's build system.

NO-NPM and NPM-PEER both seem reasonable.

I've moved ESLint out of the Meteor packages.  It is now managed in the
toplevel instead.

Npm peer dependencies seem to work.  Package test must access the package
functions through the public import.  Tests can be provided in a subdirectory
that is symlinked to `nog-app/packagetests`.  Then test with; see
<https://guide.meteor.com/testing.html#mocha>:

```
## UI
meteor test --driver-package=practicalmeteor:mocha --settings _private/settings-localhost-test.json

## Consol, server tests only.
meteor --once test --driver-package=dispatch:mocha --settings _private/settings-localhost-test.json
```

## 2016-07-09 MongoDB Atlas, Mongo DBaaS comparison (spr)

Blob post comparing hosted Mongo options, Jul 2016,
<https://meteoruniversity.org/meteor-mongodb-hosting/>.  Compose seems to be
the best options for small deployments.  It starts at 18 USD / month for
MongoDB Classic and 32 USD / month for MongoDB 3.2, which is not yet an
options; see below.  MongoDB Atlas seems to be a better option for slightly
larger data sizes (starts at 60 USD / month).  It seems reasonable to try Atlas
and maybe switch to it.  But for now, Compose Classic seems to be the more
conservative choice.

Meteor seems to have some issues with Compose 3.2 deployments.  Meteor can only
connect to a single mongos, since failover is not supported with the node
driver; see
<https://compose.com/articles/connecting-to-the-new-mongodb-at-compose/>.
Compose explicitly advises against running Meteor with 3.2 deployments
<https://www.compose.com/articles/connecting-to-the-oplog-on-the-new-mongodb/>.
Meteor GitHub issues that seem related:
<https://github.com/meteor/meteor/issues/6258>,
<https://github.com/meteor/meteor/issues/5773>.  It is not entirely clear
whether this is resolved with Meteor 1.4, which claims to fully support MongoDB
3.2.  For now, it seems safest to use Compose MongoDB Classic deployments.

Altas seems to use standard replica set URLs; see
<http://blog.cloud.mongodb.com/post/146993789415/atlas-on-day-one-importing-data>.
Although Altas only provides MongoDB 3.2, it may work with Meteor due to the
direct replica set access.

## 2016-07-08 How to use a Meteor login token for a different URL (spr)

If two app instances share a database but run on different URLs, for example
with `meteor-run2sharedb`, a Meteor login token, in principle, is valid for
both.  But Meteor cannot automatically detect this, because the browser local
storage is scoped to the URL.

As a workaround, display the token in the JavaScript console of the first URL:

```js
localStorage.getItem('Meteor.loginToken');
```

Copy paste the value and set it in the JavaScript console of the second URL:

```js
localStorage.setItem('Meteor.loginToken', '...');
```

The page will automatically login immediately after you set the token.

## 2016-07-08 MongoDB oplog links (spr)

Talk 'MongoDB and the Oplog' at Strip, slides
<http://www.slideshare.net/Stripe_talks/mongo-db-oplog>, video
<https://www.mongodb.com/presentations/building-real-time-systems-mongodb-using-oplog-stripe>.

Stripe's Mongodriver <https://github.com/stripe/mongoriver> to write oplog
tailers; see also 'Using the MongoDB Oplog to trigger asynchronous work'
<https://goo.gl/9xDJP7>.

'MoSQL: a MongoDB to SQL streaming translator'
<https://github.com/stripe/mosql>. It uses oplog tailing to receive all updates
from the oplog and put it into PostgreSQL.  The source might be useful to
understand the details of oplog tailing.

'ZeroWing' <https://github.com/colinmarc/zerowing>.  Copying from MongoDB into
HBase.

Mongo Connector <https://github.com/mongodb-labs/mongo-connector> is a Python
implementation to tail the oplog and pipe data into different backends.  It can
be extended through doc handlers.  Mongo Connector handles rollbacks; see Mongo
Connector wiki 'Writing Your Own DocManager', method `search()`,
<https://goo.gl/DV1UZY>, and grep for 'def rollback' in the source code.

Compose blog 'The MongoDB Oplog & Node.js'
<https://www.compose.com/articles/the-mongodb-oplog-and-node-js/> describes how
to tail the oplog in node.  'Oplog: Tools & Libraries for All'
<https://www.compose.com/articles/oplog-tools-and-libraries-for-all/> lists
links to oplog libraries for various languages.

The Go tooling seems particularly interesting.  Compose used it to create
'seed' <https://github.com/MongoHQ/seed>, a tool for syncing Mongo replica
sets.  It does more than we need, such as syncing the indexes.

Some npm packages may be useful:  'mongo-oplog'
<https://github.com/cayasso/mongo-oplog>.  'mongo-tail'
<https://github.com/dab00/mongodb-tail>.  'mongo-watch'
<https://github.com/TorchlightSoftware/mongo-watch>, although deprecated, may
be interesting to understand some details; for example the cursor options
<https://goo.gl/iySucM>.  The higher-level package 'particle'
<https://github.com/torchlightsoftware/particle> might be interesting, too.

I was not immediately convinced that we should use any of the existing
packages.  Code that uses the driver API directly might be easier to
understand.

See 'Tailing the MongoDB Oplog on Sharded Clusters'
<https://www.mongodb.com/blog/post/tailing-mongodb-oplog-sharded-clusters> and
'Pitfalls and Workarounds for Tailing the Oplog on a MongoDB Sharded Cluster'
<https://www.mongodb.com/blog/post/pitfalls-and-workarounds-for-tailing-the-oplog-on-a-mongodb-sharded-cluster>
as a warning that the details of oplog tailing can become tricky when using
a sharded setup.

DynamoDB Cross-region Replication
<https://github.com/awslabs/dynamodb-cross-region-library/blob/master/README.md>.
Uses a similar idea as the MongoDB oplog to replicate DynamoDB across regions.

The blog post 'Using MongoDB as Publish Subscribe middleware'
<https://www.devco.net/archives/2012/08/31/using-mongodb-as-a-queue.php> gives
an idea what can be done with capped collection tailing.

## 2016-07-08 Meteor db tricks: multiple OAuth, linking apps, ... (spr)

The SO answer to 'Using Meteor Accounts package to link multiple services'
<http://stackoverflow.com/a/18382093> describes how to link multiple OAuth
accounts to a single Meteor account.  It could be relevant at some point.

The SO answer to 'Can I use the same DB for multiple Meteor apps?'
<http://stackoverflow.com/a/28656271> describes options how to share the user
collection between Meteor apps.

Another SO answer 'How to create Meteor collection in different database?'
<http://stackoverflow.com/a/30640072> with details on multiple MongoDBs.

## 2016-07-01 MongoDB transaction patterns (spr)

A good blog post that summarized alternative approaches to transactions in
MongoDB: How to implement robust and scalable transactions across documents
with MongoDB, <http://goo.gl/iHCLFl>.

## 2016-06-30 Update to Meteor 1.3.4.1

We update, since it seems to be a good idea to use the latest version.  We did
not observe any issues with 1.3.4.

## 2016-06-23 Update to Meteor 1.3.4

The main reason for us to update is that 1.3.4 reduces build times with large
`node_modules` directories.

1.3.4 seems to reduce the build time problem a bit.  But it is not a real
solution: a test build of a package with eslint and flowtype as devDependencies
in `node_modules` took 40s, compared to 7s without `node_modules`.  It still
seems advisable to move `node_modules` temporarily if possible, so that Meteor
does not see it.

## 2016-06-18 Update to Meteor 1.3.3.1

1.3.3.1 fixes the problem related to backticks in CoffeeScript and require.
Since we want to transition to modules anyway, we keep the dependency on
modules for the nog packages.

## 2016-06-17 Update to Meteor 1.3.3

The update to Meteor 1.3.3 required some changes related to module imports and
testing:

* Packages that use CoffeeScript now use `api.use('modules')` to avoid
  a spurious undefined symbol if a CoffeeScript file contains backticks.

* Some packages now use minimal module exports to avoid undefined symbols.  For
  unknown reasons, only some packages were affected by the problem.  Since
  should refactor better sooner than later towards full use of modules import,
  the minimal amount of changes seems sufficient for now.

* Small workarounds for tree view Nightwatch tests were added.

## 2016-06-17 Update to Meteor 1.3.2.3

The update to Meteor 1.3.2.3 required minor changes related to package tests:

 - Explicit dependency on `chai`.
 - Workaround for AWS SDK.

## 2016-02-28 Capacity planning on a napkin

<http://blog.smartbear.com/loaduiweb/capacity-planning-on-a-cocktail-napkin/>

## 2016-02-28 Keycloak for AuthN (spr)

Keycloak <http://keycloak.jboss.org> seems to be an interesting option for
identity management; presentation <https://youtu.be/kt35NROpjE0>.  It support
MongoDB backend with replication (see Keycloak reference guide).

The meteor package `silveirado:keycloak-auth`
<https://github.com/Procempa/meteor-keycloak-auth> could be useful; forked to
<https://github.com/sprohaska/meteor-keycloak-auth>.

## 2016-02-28 Meteor links (spr)

DDP monitor in Chrome seems very useful.
<https://github.com/thebakeryio/meteor-ddp-monitor>

## 2016-02-28 Meteor 1.3 links (spr)

Why JS2015 import / export? <http://benjamn.github.io/empirenode-2015/#/>

Best practices JS2015-style npm package
<https://github.com/benjamn/jsnext-skeleton>.

Meteor 1.3 and React example, blog with good links to other sources.
<https://medium.com/@kenrogers/build-a-journaling-app-with-meteor-1-3-beta-react-react-bootstrap-and-mantra-7965d9e9fc23#.5kmif6omr>

## 2016-02-24 OpenStack Swift links (spr)

Swift all in one dev setup:
<http://docs.openstack.org/developer/swift/development_saio.html>.

Swift onlyone Docker container:
<https://github.com/ccollicutt/docker-swift-onlyone>.

Swift middleware to emulate S3 API: <https://github.com/openstack/swift3>.

Oracle Cloud Storage with Swift API:
<https://docs.oracle.com/cloud/latest/storagecs_common/CSSTO/GUID-47A892FA-7C3D-4FF6-BBE0-D4B7362E9F7D.htm#CSSTO-GUID-47A892FA-7C3D-4FF6-BBE0-D4B7362E9F7D>.
Google for 'oracle swift HSM' for links about Swift with SAMFS tape storage.

Open question: Is 'finding out the status of objects in an archive container'
/ HTTP header `X-Archive-Restore-Status` implemented when we install locally?

## 2016-02-14 Various links (spr)

Faker might be useful to create testing data
<https://github.com/practicalmeteor/meteor-faker>.  It has an image mode, which
might be interesting for blobber.

Meteor dev Docker images from Practical Meteor
<https://github.com/practicalmeteor/docker-meteor-dev> might be worth a look.

Practical Meteor's isup <https://github.com/practicalmeteor/meteor-isup>.  We
should either use it directly or do something similar to implement a `/_health`
or `/healthz` route.

`practicalmeteor:easy-meteor-settings`
<https://github.com/practicalmeteor/meteor-easy-meteor-settings> seems useful.
It could replace some `foo?.bar?.` chains in nog.

Mantra <https://kadirahq.github.io/mantra/> describes Kadira's opinionated
recommendation for Meteor 1.3 apps.  No immediately usable, but interesting.
No universal apps
<https://voice.kadira.io/say-no-to-isomorphic-apps-b7b7c419c634#.mqlrskqb7>.

JavaScript module in Meteor 1.3
<https://github.com/meteor/meteor/blob/release-1.3/packages/modules/README.md>.

Meteor meets GraphQL
<https://voice.kadira.io/meteor-meets-graphql-3cba2e65fd00#.r929my4wr>.

## 2016-02-14 Testing links (spr)

Spacejam uses a relatively old version of PhantomJS, which has issues with
modern HTML5 concepts.  GitHub issue
<https://github.com/practicalmeteor/spacejam/issues/23>.

Spacejam presentation, YouTube <https://www.youtube.com/watch?v=CoUZETNKuqU>,
Slides
<https://www.dropbox.com/s/7yqx21ldaqa2tag/Unit%20testing%20and%20CI%20with%20mocha%20and%20spacejam.pptx?dl=0>.
Plans to support running tests in real browser instead of PhatomJS
<https://youtu.be/CoUZETNKuqU?t=31m23s>.  Interesting thoughts: CI services
<https://youtu.be/CoUZETNKuqU?t=23m16s>, Drone.io docker container could be
interesting; deploy every pull request as a separate app instance
<https://youtu.be/CoUZETNKuqU?t=32m24s>.

My thoughts: go open source, then use CI services like Travis-CI for free.
Maybe try Drone.io Docker container.  Survice with Spacejam PhantomJS; wait for
support for other browsers.

Chimp seems to be Xolvio's replacement for Velocity Cucumber
<https://chimp.readme.io/docs/migrating-from-xolviocucumber-to-chimp>.

Meteor 1.3 probably gets a application test mode.  Preview text for guide
<https://github.com/meteor/guide/blob/testing-modules-content/content/testing.md>.
Forum announcement
<https://forums.meteor.com/t/a-first-implementation-of-meteor-app-testing-for-1-3/17097>.

Gagarin on CircleCI
<http://www.johnpinkerton.me/2016/01/03/testing-meteor-with-gagarin-on-circleci-osx-environment/>.
Chromedriver and Selenium are available via Brew.

Gagarin tests specs seem to be a good source for ideas what Gagarin can do
<https://github.com/anticoders/gagarin/tree/develop/tests/specs>.  Gagarin
context to pass state through the promise chain
<https://github.com/anticoders/gagarin/blob/develop/tests/specs/context.js>.
Gagarin account helpers
<https://github.com/anticoders/gagarin/blob/develop/tests/specs/helpersAccounts.js>.

Nightwatch could probably be used with Gagarin tests
<https://github.com/anticoders/gagarin/issues/141>.  The following code inside
a Gagarin test file opened the URL in a browser.  So it seems to work in
principle.

```coffee
nightwatch = require 'nightwatch'

describe 'nightwatch', ->
  nwclient = nightwatch.initClient({silent: true})
  nwbrowser = nwclient.api()
  it 'nightwatch', (done) ->
    nwbrowser
      .url('http://google.com')
      .waitForElementVisible('body', 5000)
    nwclient.start(done)
```


## 2016-02-12 React links (spr)

React + Coffeescript: I like the Lisp style `(ul, {}, [...])` as described here
<http://developerblog.redhat.com/2014/06/19/have-some-coffeescript-with-your-react/>.
I think it could be a reasonable (maybe even better) replacement of Jade
template files.

## 2016-02-12 Testing links (spr)

Some links and thoughts about testing.

tl;dr: I think it makes sense to try StarryNight with the test frameworks
tinytest-ci (+ smithy:describe), gagarin and nightwatch. If it looks good after
trying a bit, we should consider it a candidate for our new testing
environment.

I think we should not spend to much time now on a perfect solution, because
things might change soon again with Meteor 1.3, when JavaScript modules become
available. The beta of 1.3 is already available.

 - <http://l.goodbits.io/l/asy364oz>
 - <https://github.com/meteor/meteor/blob/release-1.3/packages/modules/README.md>

The meteor package clinical:nightwatch and nightwatch itself are two separate
things. clinical:nightwatch never worked for me, but nightwatch always did (see
LOG-2015).

StarryNight seems to be a replacement for clinical:nightwatch. But StarryNight
also brings a lot more tooling. The run-test part might be exactly what we
want: it seems to support tinytest and nightwatch.

But I'm still undecided: tinytest + nightwatch (maybe through StarryNight)
seems the more conservative choice. tinytest + gagarin seems to be the riskier,
however maybe more powerful, approach.

arunoda's recommendation seems to be definitely gagarin:
<https://forums.meteor.com/t/continuous-integration-with-meteor-circleci-and-gagarin/14192/6>,
<https://hackpad.com/Gagarin-Guide-RzdMvlwyYHV>

The StarryNight folks seem to also use Gagarin for some parts
<https://starrynight.meteor.com/testing>.

They also experiment with spacejam for chai assertions with TinyTest. First
look: I'm not convinced of spacejam. I'd try TinyTest + smithy:describe first.

Blog post about Gagarin
<https://medium.com/@SamCorcos/continuous-integration-with-meteor-circleci-and-gagarin-a77db143efdd#.wj5w0hopu>.

smithy:describe <https://github.com/paolo/smithy-describe/> seems to
a reasonable package to ease the transition. It is a thin wrapper around
tinytest to provide describe.it syntax. The package source fits on 4 screens.
It also packages sinon and chai.

Xolv.io stopped maintaining Velocity <http://xolv.io/velocity-announcement>.
It's unclear whether MDG picked it up (quick googling was inconclusive). I had
issues with package tests during the migration to 1.2; some are still
unresolved. => maybe better stop using Velocity. Use tinytest instead for
package tests, since it is officially supported by MDG.

The cookbook's
<https://github.com/clinical-meteor/cookbook/blob/master/cookbook/test-driven-development.md>
recommendation is TinyTest for packages and Nightwatch for UI.

ui harness: The video only explains a concept in my opinion. The
meteor-ui-harness package is not very active and relatively undocumented. My
feeling is: "don't use it". I think we could instead follow the meteor guide
and implement our own solution to use specific routes for testing that display
the dumb components in isolation. The components can then be tested either
manually or using Gagarin or Nightwatch.

## 2016-01-24 Start year

See [LOG-2015](./LOG-2015.md) for older entries.

<!-- vim: set sw=4: -->
