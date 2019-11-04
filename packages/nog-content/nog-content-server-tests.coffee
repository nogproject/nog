import chai from 'chai'
import sinon from 'sinon'
import sinonChai from 'sinon-chai'
chai.use(sinonChai)
{ expect } = chai

{
  ERR_LOGIC
  nogthrow
} = NogError

{ NogContent, NogContentTest } = require 'meteor/nog-content'
{ contentId } = NogContent
{
  create
  collectionContainsCache
} = NogContentTest

RAW = {transform: false}
NULL_SHA1 = '0000000000000000000000000000000000000000'

testdata = idversion_testdata

# Named collections must be created only once during program startup, because
# the name acts as a global identifier in method definitions.
fakeBlobs = new Mongo.Collection 'testing.blobs'

fakeUsers = new Mongo.Collection 'testing.users'

fakeBlobId = '3f786850e387550fdab836ed7e6dc881de23001b'  # sha1_hex("a\n")

nullSha1 = '0000000000000000000000000000000000000000'
missingObjectId = '1111111111111111111111111111111111111111'
missingTreeId = '2222222222222222222222222222222222222222'
missingCommitId = '3333333333333333333333333333333333333333'
missingBlobId = '4444444444444444444444444444444444444444'

fakeObj =
  'name': 'Fake object'
  'meta':
    'content': 'test text'
    'study': 'foo',
    'specimen': 'bar'
  'blob': fakeBlobId

# Will be set below.
fakeObjId_v0 = null  # Forced idv0.
fakeObjId_v1 = null  # Forced idv1

fakeTree =
  'name': 'Fake tree'
  'meta':
    'study': 'foo',
    'specimen': 'bar'
    'a.b.c': 'bar'
  'entries': []

fakeTreeId = null  # Will be set below.

fakeCommit =
  subject: 'Fake commit'
  message: 'Lorem ipsum dolor...'
  meta:
    'gitcommit': '4444444444444444444444444444444444444444'

# Vars will be initialized below.
fakeCommitId = null
fakeCommitWithParentId_v0 = null  # A commit with idv0.
fakeCommitWithParentId = null  # A commit with idv1.

# See <http://docs.meteor.com/#/full/meteor_users> for user document format.
fakeUserDoc =
  _id: 'fakeUserId'
  username: 'fakeUser'
  emails: [{address: 'fake@example.com'}]
  profile:
    name: 'Fake User'

rgxSha1 = /// ^ [0-9a-f]{40} $ ///

# ISO datetime with UTC `Z`.
rgxISOStringUTC = ///
  ^
  [0-9]{4}-[0-9]{2}-[0-9]{2}
  T
  [0-9]{2}:[0-9]{2}:[0-9]{2}
  Z
  $
  ///

# ISO datetime with tz offset.
rgxISOStringTZ = ///
  ^
  [0-9]{4}-[0-9]{2}-[0-9]{2}
  T
  [0-9]{2}:[0-9]{2}:[0-9]{2}
  [+-]
  [0-9]{2}:[0-9]{2}
  $
  ///


expectToContainAll = (val, toks) ->
  for tok in toks
    expect(val).to.contain tok


resetCollections = (opts) ->
  for k, v of opts
    # Insert a fake document to avoid Mongo 'ns not found'.
    v.insert {}
    v._dropCollection()


describe 'nog-content', ->

  deps = null
  store = null
  ownerName = 'userbar'
  ownerId = 'userbarId'

  @timeout(10000)
  before ->
    deps = _.pick(
        NogContent, 'repos', 'commits', 'trees', 'objects', 'deletedRepos'
      )
    deps.blobs = fakeBlobs
    deps.users = fakeUsers
    resetCollections deps
    deps.users.insert {_id: ownerId, username: ownerName}
    deps.users.insert {_id: fakeUserDoc._id, username: fakeUserDoc.username}
    deps.checkAccess = sinon.spy()
    store = new NogContentTest.Store deps

  describe 'Store', ->
    userName = 'userfoo'
    repoName = 'repobar'
    repoFullName = "#{ownerName}/#{repoName}"
    invalidNameOpts = [
        {ownerName: 'a', repoName: 'b:'}
        {ownerName: 'a:', repoName: 'b'}
      ]
    invalidRefNames = ['a b', 'a:b', 'a@b', 'a/b/', '/a/b']

    describe 'createRepo()', ->
      nulluser = null

      it 'throws if owner is unknown.', ->
        fn = -> store.createRepo userName, {repoFullName: 'unknownUser/foo'}
        expect(fn).to.throw '[ERR_UNKNOWN_USERNAME]'

      it 'inserts a repo.', ->
        repoId = store.createRepo userName, {repoFullName}
        repo = deps.repos.findOne repoId
        expect(repo).to.exist
        expect(repo.owner).to.equal ownerName
        expect(repo.ownerId).to.equal ownerId
        expect(repo.name).to.equal repoName
        for k, v of repo.refs ? {}
          expect(v, 'ref is sha1').to.match rgxSha1

      it 'checks access.', ->
        deps.checkAccess.reset()
        randomName = Random.id()
        store.createRepo userName, {repoFullName: "#{ownerName}/#{randomName}"}
        expect(deps.checkAccess).to.have.been.calledWith(
            userName, 'nog-content/create-repo', sinon.match {
                ownerName: ownerName
                repoName: randomName
              }
          )

      it 'rejects invalid repo full names.', ->
        invalidNames = [
            'a', 'a./b', 'a:/b', 'a/b:', 'a/.b', 'a/b.', 'a/b/c'
          ]
        for name in invalidNames
          fn = -> store.createRepo nulluser, {repoFullName: name}
          expect(fn, "name: #{name}").to.throw 'Invalid'

      it 'throws if the repo already exists.', ->
        fn = -> store.createRepo userName, {repoFullName}
        expect(fn).to.throw '[ERR_CONTENT_REPO_EXISTS]'


    describe 'deleteRepo()', ->
      it 'throws if the repo is unknown', ->
        fn = -> store.deleteRepo userName, {
            ownerName, repoName: 'unknown'
          }
        expect(fn).to.throw '[ERR_REPO_MISSING]'

      it 'checks access', ->
        randomName = Random.id()
        store.createRepo userName, {repoFullName: ownerName + '/' + randomName}
        deps.checkAccess.reset()
        store.deleteRepo userName, {ownerName, repoName: randomName}
        expect(deps.checkAccess).to.have.been.calledWith(
            userName, 'nog-content/delete-repo', sinon.match {
                ownerName: ownerName
                repoName: randomName
              }
          )

      it 'moves the raw repo doc to the deleted repos collection',  ->
        randomName = Random.id()
        repoId = store.createRepo userName, {
            repoFullName: ownerName + '/' + randomName
          }
        expect(deps.repos.findOne(repoId)).to.exist
        store.deleteRepo userName, {ownerName, repoName: randomName}
        expect(deps.repos.findOne(repoId)).to.not.exist
        deleted = deps.deletedRepos.findOne(repoId)
        expect(deleted).to.exist
        expect(deleted.fullName).to.not.exist

      it 'restarts after crash', ->
        for loc in ['_deleteRepo2', '_deleteRepo3']
          randomName = Random.id()
          repoId = store.createRepo userName, {
            repoFullName: ownerName + '/' + randomName
          }
          expect(deps.repos.findOne(repoId)).to.exist
          store._maybeCrash = (l) ->
            if l == loc
              throw new Error('fake crash')
          fn = ->
            store.deleteRepo userName, { ownerName, repoName: randomName }
          expect(fn).to.throw 'fake crash'
          store._maybeCrash = ->
          store.tick({ timeout_s: 0 })
          expect(deps.repos.findOne(repoId)).to.not.exist
          deleted = deps.deletedRepos.findOne(repoId)
          expect(deleted).to.exist

    describe 'interrupted deleteRepo()', ->

      # Before each test, create a repo with a crashed deleteRepo cmd.
      # Cleanup after each test.

      repoId = null
      randomName = null

      beforeEach ->
        randomName = Random.id()
        repoId = store.createRepo userName, {
          repoFullName: ownerName + '/' + randomName
        }
        expect(deps.repos.findOne(repoId)).to.exist
        store._maybeCrash = (l) -> throw new Error('fake crash')
        fn = ->
          store.deleteRepo userName, { ownerName, repoName: randomName }
        expect(fn).to.throw 'fake crash'

      afterEach ->
        store._maybeCrash = ->
        store.tick({ timeout_s: 0 })
        expect(deps.repos.findOne(repoId)).to.not.exist
        expect(deps.deletedRepos.findOne(repoId)).to.exist

      it 'blocks another deleteRepo()', ->
        fn = ->
          store.deleteRepo userName, { ownerName, repoName: randomName }
        expect(fn).to.throw '[ERR_CONFLICT]'

      it 'blocks renameRepo()', ->
        fn = ->
          store.renameRepo userName, {
            old: { ownerName, repoName: randomName },
            new: { repoFullName: "#{ownerName}/#{Random.id()}" },
          }
        expect(fn).to.throw '[ERR_CONFLICT]'

      it 'blocks forkRepo()', ->
        fn = ->
          store.forkRepo userName, {
            old: { ownerName, repoName: randomName },
            new: { ownerName },
          }
        expect(fn).to.throw '[ERR_CONFLICT]'

      it 'blocks updateRef()', ->
        repo = deps.repos.findOne(repoId)
        fn = ->
          store.updateRef userName, {
            ownerName, repoName: randomName,
            refName: 'branches/master',
            old: repo.refs['branches/master'],
            new: NULL_SHA1,
          }
        expect(fn).to.throw '[ERR_CONFLICT]'


    describe 'renameRepo()', ->
      it 'throws if the repo is missing', ->
        fn = -> store.renameRepo userName, {
            old: {ownerName, repoName: 'unknown'}
            new: {repoFullName: 'foo/bar'}
          }
        expect(fn).to.throw '[ERR_REPO_MISSING]'

      it 'checks access', ->
        r1 = Random.id()
        r2 = Random.id()
        store.createRepo userName, {repoFullName: ownerName + '/' + r1}
        deps.checkAccess.reset()
        store.renameRepo userName, {
            old: {ownerName, repoName: r1}
            new: {repoFullName: "#{ownerName}/#{r2}"}
          }
        expect(deps.checkAccess).to.have.been.calledWith(
            userName, 'nog-content/rename-repo', sinon.match {
                old: {ownerName: ownerName, repoName: r1}
                new: {ownerName: ownerName, repoName: r2}
              }
          )

      it 'throws if the new name already exists', ->
        r1 = Random.id()
        store.createRepo userName, {repoFullName: ownerName + '/' + r1}
        r2 = Random.id()
        store.createRepo userName, {repoFullName: ownerName + '/' + r2}
        fn = -> store.renameRepo userName, {
            old: {ownerName, repoName: r1}
            new: {repoFullName: "#{ownerName}/#{r2}"}
          }
        expect(fn).to.throw '[ERR_CONTENT_REPO_EXISTS]'
        expect(fn).to.throw r2

      it 'rejects invalid repo full names', ->
        rand = Random.id()
        randFullName = "#{ownerName}/#{rand}"
        store.createRepo userName, {repoFullName: randFullName}
        nulluser = null
        invalidNames = [
            'a', 'a./b', 'a:/b', 'a/b:', 'a/.b', 'a/b.', 'a/b/c'
          ]
        for name in invalidNames
          fn = -> store.renameRepo nulluser, {
              old: {ownerName, repoName: rand}
              new: {repoFullName: name}
            }
          expect(fn, "name: #{name}").to.throw 'Invalid'

      it 'renames to new name and maintains a list of old full names', ->
        r1 = Random.id()
        repoId = store.createRepo userName, {repoFullName: ownerName + '/' + r1}
        r2 = Random.id()
        store.renameRepo userName, {
            old: {ownerName, repoName: r1}
            new: {repoFullName: "#{ownerName}/#{r2}"}
          }
        repo = deps.repos.findOne repoId
        expect(repo.oldFullNames).to.contain "#{ownerName}/#{r1}"
        expect(repo.name).to.equal r2

    describe 'forkRepo()', ->
      fakeUsername = fakeUserDoc.username

      it 'throws if repo is missing', ->
        fn = -> store.forkRepo userName, {
            old: {ownerName, repoName: 'unknown'}
            new: {ownerName}
          }
        expect(fn).to.throw '[ERR_REPO_MISSING]'

      it 'checks access (fork, get old, and create new)', ->
        r1 = Random.id()
        store.createRepo fakeUserDoc, {repoFullName: fakeUsername + '/' + r1}
        deps.checkAccess.reset()
        store.forkRepo userName, {
            old: {ownerName: fakeUsername, repoName: r1}
            new: {ownerName}
          }
        expect(deps.checkAccess).to.have.been.calledWith(
            userName, 'nog-content/fork-repo', sinon.match {
                ownerName: fakeUsername, repoName: r1
              }
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            userName, 'nog-content/get', sinon.match {
                ownerName: fakeUsername, repoName: r1
              }
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            userName, 'nog-content/create-repo', sinon.match {
                ownerName, repoName: r1
              }
          )

      it 'creates a new repo with a fork link', ->
        r1 = Random.id()
        id1 = store.createRepo fakeUserDoc, {
            repoFullName: fakeUsername + '/' + r1
          }
        deps.repos.update id1, {$set: {'refs.fake': missingTreeId}}
        forkRes = store.forkRepo ownerName, {
            old: {ownerName: fakeUsername, repoName: r1}
            new: {ownerName}
          }
        fork = deps.repos.findOne(forkRes.id)
        expect(fork).to.exist
        expect(fork.name).to.equal forkRes.name
        expect(fork.owner).to.equal forkRes.owner
        expect(fork.name).to.equal r1
        expect(fork.owner).to.equal ownerName
        expect(fork.ownerId).to.equal ownerId
        expect(fork.refs.fake).to.equal missingTreeId
        expect(fork.forkedFrom).to.deep.equal {
            id: id1, owner: fakeUsername, name: r1
          }

      it 'automatically resolves naming conflicts', ->
        r1 = Random.id()
        store.createRepo fakeUserDoc, {
            repoFullName: fakeUsername + '/' + r1
          }
        store.createRepo fakeUserDoc, {
            repoFullName: ownerName + '/' + r1
          }
        forkRes = store.forkRepo ownerName, {
            old: {ownerName: fakeUsername, repoName: r1}
            new: {ownerName}
          }
        fork = deps.repos.findOne(forkRes.id)
        expect(fork).to.exist
        expect(fork.name).to.equal forkRes.name
        expect(fork.name).to.not.equal r1
        # Start postfix search with '2', not '1'.
        expect(fork.name[fork.name.length - 1]).to.equal '2'
        expect(deps.checkAccess).to.have.been.calledWith(
            ownerName, 'nog-content/create-repo', sinon.match {
                ownerName, repoName: fork.name
              }
          )

    describe 'createObject()', ->
      obj = fakeObj

      it 'inserts object.', ->
        deps.blobs.upsert fakeBlobId, {$set: {status: 'available'}}
        content = _.clone fakeObj
        content._idversion = 0
        objId = store.createObject userName, {
            ownerName: ownerName, repoName: repoName, content
          }
        o = deps.objects.findOne objId
        expect(o).to.exist
        for k, v of obj
          expect(o[k]).to.deep.equal obj[k]
        # Save for later use:
        #
        #  - idv0 with forced idv 0.
        #  - idv1
        #
        fakeObjId_v0 = objId
        content = _.clone fakeObj
        content._idversion = 1
        fakeObjId_v1 = store.createObject userName, {
            ownerName, repoName, content
          }
        expect(fakeObjId_v0).to.not.equal(fakeObjId_v1)
        res = store.objects.findOne fakeObjId_v1
        expect(res._idversion).to.equal 1

      it 'inserts idv0 object if forced.', ->
        content = _.clone fakeObj
        content.meta = _.clone content.meta
        content.meta.content = 'idv0 text content'
        content.blob = nullSha1
        content._idversion = 0
        id = store.createObject userName, {ownerName, repoName, content}
        o = deps.objects.findOne id
        expect(o).to.exist
        expect(o._idversion).to.equal 0

      it 'throws if both text and meta.content.', ->
        content = _.clone fakeObj
        content.meta = _.clone content.meta
        content.meta.content = 'idv0 text content'
        content.text = content.meta.content
        fn = -> store.createObject userName, {ownerName, repoName, content}
        expect(fn).to.throw '[ERR_PARAM_INVALID]'

      it 'inserts idv1 object by default.', ->
        content = _.clone fakeObj
        content.meta = _.clone content.meta
        delete content.meta.content
        content.text = 'idv1 text content'
        content.blob = null
        id = store.createObject userName, {
            ownerName, repoName, content
          }
        o = deps.objects.findOne id
        expect(o).to.exist
        expect(o._idversion).to.equal 1

      # See testdata for details.
      it 'inserts and fetches variants correctly.', ->
        for test in _.where(testdata.tests, {type: 'object'})
          for content in test.create ? []
            blob = content.blob
            if blob? and blob != nullSha1
              deps.blobs.upsert blob, {$set: {status: 'available'}}
            deps.objects.remove {_id: test.canonical._id}
            id = store.createObject userName, {ownerName, repoName, content}
            expect(id).to.equal test.canonical._id
            raw = deps.objects.findOne test.canonical._id, RAW
            expect(raw).to.deep.equal test.mongo

            tf = store.getObject userName, {ownerName, repoName, sha1: id}
            if test.canonical.text? or test.canonical.meta.content?
              # XXX: For transition period both text and meta.content.
              expect(tf).to.exist
              expect(tf.text).to.exist
              expect(tf.meta.content).to.exist
              expect(tf.text).to.equal tf.meta.content
            if not blob? or blob == nullSha1
              expect(tf.blob).to.be.null


      it 'throws if blob is missing.', ->
        deps.blobs.remove {}
        collectionContainsCache.clear()
        fn = -> store.createObject userName, {
            ownerName: ownerName, repoName: repoName, content: obj
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'

      it 'does not check existence of null blob.', ->
        o = _.clone obj
        o.blob = nullSha1
        fn = -> store.createObject userName, {
            ownerName: ownerName, repoName: repoName, content: o
          }
        fn()  # Does not throw.
        o.blob = null
        fn()  # Does not throw.

      it 'can be intantiated to accept missing blob.', ->
        depsNoBlobs = _.omit deps, 'blobs'
        storeNoBlobs = new NogContentTest.Store depsNoBlobs
        storeNoBlobs.createObject userName, {
            ownerName: ownerName, repoName: repoName, content: obj
          }  # Does not throw.

      it '
        stores selected meta as toplevel fields and the rest as a {key, val}
        array (idv0).
      ', ->
        deps.blobs.upsert fakeBlobId, {$set: {status: 'available'}}
        o = _.clone obj
        o._idversion = 0
        o.meta = {}
        o.meta['x.y.z'] = 'baz'
        selected = ['description', 'content']
        for s in selected
          o.meta[s] = s
        objId = store.createObject userName, {
            ownerName: ownerName, repoName: repoName, content: o
          }  # Does not throw.
        res = deps.objects.findOne objId, {transform: null}
        for s in selected
          expect(res.meta[s]).to.equal s
        expect(res.meta.more).to.have.length 1
        expect(res.meta.more[0].key).to.equal 'x.y.z'
        expect(res.meta.more[0].val).to.equal 'baz'

      it '
        stores selected meta as toplevel fields and the rest as a {key, val}
        array (idv1).
      ', ->
        deps.blobs.upsert fakeBlobId, {$set: {status: 'available'}}
        o = _.clone obj
        o.meta = {}
        o.meta['x.y.z'] = 'baz'
        selected = ['description']
        for s in selected
          o.meta[s] = s
        objId = store.createObject userName, {
            ownerName: ownerName, repoName: repoName, content: o
          }  # Does not throw.
        res = deps.objects.findOne objId, {transform: null}
        for s in selected
          expect(res.meta[s]).to.equal s
        expect(res.meta.more).to.have.length 1
        expect(res.meta.more[0].key).to.equal 'x.y.z'
        expect(res.meta.more[0].val).to.equal 'baz'

    describe 'getObject()', ->
      it 'gets an object.', ->
        res = store.getObject userName, {
            ownerName, repoName, sha1: fakeObjId_v1
          }
        expect(res).to.exist
        expect(res._id).to.equal fakeObjId_v1


    describe 'createTree()', ->

      it 'inserts trees.', ->
        tree = _.clone fakeTree
        tree.entries = [{type: 'object', sha1: fakeObjId_v1}]
        fakeTreeId = id = store.createTree userName, {
            ownerName: ownerName, repoName: repoName, content: tree
          }
        o = deps.trees.findOne id
        expect(o).to.exist
        for k, v of tree
          expect(o[k]).to.deep.equal tree[k]

        tree.entries.push {type: 'tree', sha1: id}
        id2 = store.createTree userName, {
            ownerName: ownerName, repoName: repoName, content: tree
          }
        o = deps.trees.findOne id2
        expect(o).to.exist
        for k, v of tree
          expect(o[k]).to.deep.equal tree[k]

      it 'inserts expanded trees.', ->
        tree = _.clone fakeTree
        tr = _.clone fakeTree
        tr.entries = [fakeObj]
        tree.entries = [fakeObj, tr]
        id = store.createTree userName, {
            ownerName: ownerName, repoName: repoName, content: tree
          }
        o = deps.trees.findOne id
        expect(o).to.exist
        expect(o.entries[0].sha1).to.equal fakeObjId_v1
        expect(o.entries[1].sha1).to.equal fakeTreeId

      it 'rejects invalid entry types.', ->
        tree = _.clone fakeTree
        tree.entries = [{type: 'foo', sha1: fakeObjId_v1}]
        fn = -> store.createTree userName, {
            ownerName: ownerName, repoName: repoName, content: tree
          }
        expect(fn).to.throw 'entry type'

      it 'throws if an entry of type object is missing.', ->
        tree = _.clone fakeTree
        tree.entries = [{type: 'object', sha1: missingObjectId}]
        fn = -> store.createTree userName, {
            ownerName: ownerName, repoName: repoName, content: tree
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'

      it 'throws if an entry of type tree is missing.', ->
        tree = _.clone fakeTree
        tree.entries = [{type: 'tree', sha1: missingTreeId}]
        fn = -> store.createTree userName, {
            ownerName: ownerName, repoName: repoName, content: tree
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'

    describe 'getTree()', ->
      it 'gets a tree.', ->
        res = store.getTree userName, {
            ownerName, repoName, sha1: fakeTreeId
          }
        expect(res).to.exist
        expect(res._id).to.equal fakeTreeId


    describe 'createCommit()', ->

      it 'inserts commits.', ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = []
        fakeCommitId = id = store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        res = deps.commits.findOne id
        expect(res).to.exist
        for k in ['subject', 'message', 'tree', 'parents', 'meta']
          expect(res[k]).to.deep.equal commit[k]
        expect(res.authors).to.have.length 1
        expect(res.authors[0]).to.contain fakeUserDoc.profile.name
        expect(res.authors[0]).to.contain fakeUserDoc.emails[0].address
        expect(res.committer).to.contain fakeUserDoc.profile.name
        expect(res.committer).to.contain fakeUserDoc.emails[0].address
        expect(res.authorDate).to.exist
        expect(res.commitDate).to.exist

        commit.parents = [id]
        # Explicitly specify TZ to force idv1.
        commit.authorDate = '2000-01-01T00:00:00+01:00'
        commit.commitDate = '2000-01-01T00:00:00+01:00'
        fakeCommitWithParentId = id2 = store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        res = deps.commits.findOne id2
        expect(res).to.exist
        expect(res.parents).to.deep.equal commit.parents

        # Confirm that idv0 commits can be inserted.
        commit.authorDate = '2000-01-01T00:00:00Z'
        commit.commitDate = '2000-01-01T00:00:00Z'
        content = _.clone commit
        content._idversion = 0
        fakeCommitWithParentId_v0 = id3 = store.createCommit fakeUserDoc, {
            ownerName, repoName, content
          }
        res = deps.commits.findOne id3, RAW
        expect(res.authorDate).to.match rgxISOStringUTC
        expect(res.commitDate).to.match rgxISOStringUTC

        # Test momentjs behavior >= 2.13.0.  It now uses `Z` for UTC dates.
        # See <https://github.com/moment/moment/pull/3098>.
        #
        # Confirm that TZ `+00:00` is preserved and not converted to `Z`.
        commit.authorDate = '2001-01-01T00:00:00+00:00'
        commit.commitDate = '2002-02-02T00:00:00+00:00'
        id4 = store.createCommit(fakeUserDoc, {
          ownerName, repoName, content: commit
        })
        res = deps.commits.findOne(id4, RAW)
        expect(res.authorDate).to.equal(commit.authorDate)
        expect(res.commitDate).to.equal(commit.commitDate)

        # Confirm that default date uses `+00:00`.
        delete commit.authorDate
        delete commit.commitDate
        id5 = store.createCommit(fakeUserDoc, {
          ownerName, repoName, content: commit
        })
        res = deps.commits.findOne(id5, RAW)
        expect(res.authorDate).to.have.string('+00:00')
        expect(res.commitDate).to.have.string('+00:00')

      it 'inserts empty default meta.', ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = []
        delete commit.meta
        id = store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        res = deps.commits.findOne id
        expect(res).to.exist
        expect(res.meta).to.exist

      it 'takes authors and authorDate from opts.', ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = []
        commit.authors = ['A U Thor <author@example.com>', 'foo@example.com']
        commit.authorDate = new Date()
        commit.authorDate.setMilliseconds(0)
        id = store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        res = deps.commits.findOne id
        expect(res.authors).to.deep.equal commit.authors
        expect(res.authorDate.toISOString()).to.equal(
            commit.authorDate.toISOString()
          )

      it 'takes committer and commitDate from opts.', ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = []
        commit.committer = 'A U Thor <author@example.com>'
        commit.commitDate = new Date()
        commit.commitDate.setMilliseconds(0)
        id = store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        res = deps.commits.findOne id
        expect(res.committer).to.equal commit.committer
        expect(res.commitDate.toISOString()).to.equal(
            commit.commitDate.toISOString()
          )

      it 'checks opts.', ->
        fn = -> store.createCommit fakeUserDoc, {
            ownerName, repoName, content: fakeCommit
          }
        expect(fn).to.throw 'Match'

      it 'stores dates as ISO strings, and returns them as moments.', ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = []
        commit.authorDate = '2000-01-01T00:00:00+01:00'
        commit.commitDate = '2000-01-01T00:00:00-06:00'
        id = store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        rawres = deps.commits.findOne id, {transform: null}
        expect(rawres.authorDate).to.match rgxISOStringTZ
        expect(rawres.commitDate).to.match rgxISOStringTZ
        res = deps.commits.findOne id
        expect(moment.isMoment(res.authorDate), 'is moment').to.be.true
        expect(moment.isMoment(res.commitDate), 'is moment').to.be.true

      it 'throws if the tree is missing.', ->
        commit = _.clone fakeCommit
        commit.tree = missingTreeId
        commit.parents = []
        fn = -> store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'

      it 'throws if the parent commit is missing.', ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = [missingCommitId]
        fn = -> store.createCommit fakeUserDoc, {
            ownerName, repoName, content: commit
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'

    describe 'getCommit()', ->
      it 'gets a commit.', ->
        res = store.getCommit userName, {
            ownerName, repoName, sha1: fakeCommitId
          }
        expect(res).to.exist
        expect(res._id).to.equal fakeCommitId

    describe 'copyEntry()', ->
      repoName2 = Random.id()

      it 'copies various entry types', ->
        repoFullName2 = ownerName + '/' + repoName2
        store.createRepo userName, {repoFullName: repoFullName2}
        specs = [
            {type: 'object', sha1: fakeObjId_v1}
            {type: 'tree', sha1: fakeTreeId}
            {type: 'commit', sha1: fakeCommitId}
            {type: 'blob', sha1: fakeBlobId}
          ]
        for s in specs
          res = store.copyEntry userName, {
              ownerName
              repoName: repoName2
              content:
                copy: _.extend({repoFullName}, s)
            }
          expect(res).deep.equal s

      it 'throws if content is missing', ->
        repoFullName2 = ownerName + '/' + repoName2
        specs = [
            {type: 'object', sha1: missingObjectId}
            {type: 'tree', sha1: missingTreeId}
            {type: 'commit', sha1: missingCommitId}
            {type: 'blob', sha1: missingBlobId}
          ]
        for s in specs
          fn = -> store.copyEntry userName, {
              ownerName
              repoName: repoName2
              content:
                copy: _.extend({repoFullName}, s)
            }
          expect(fn).to.throw '[ERR_CONTENT_MISSING]'

    describe 'updateRef()', ->

      findRepo = ->
        deps.repos.findOne {owner: ownerName, name: repoName}

      it 'expects and old and a new ref.', ->
        refName = 'branches/master'
        fn = -> store.updateRef userName, {
            ownerName, repoName, refName, new: fakeCommitId
          }
        expect(fn).to.throw 'Match'
        fn = -> store.updateRef userName, {
            ownerName, repoName, refName, old: nullSha1
          }
        expect(fn).to.throw 'Match'

      it 'updates a null ref.', ->
        refName = 'branches/master'
        expect(findRepo().refs[refName]).to.be.equal nullSha1
        store.updateRef userName, {
            ownerName, repoName, refName, old: nullSha1, new: fakeCommitId
          }
        expect(findRepo().refs[refName]).to.be.equal fakeCommitId

      it 'creates a new ref.', ->
        refName = 'branches/' + Random.id()
        store.updateRef userName, {
            ownerName, repoName, refName, old: nullSha1, new: fakeCommitId
          }
        expect(findRepo().refs[refName]).to.be.equal fakeCommitId

      it 'accepts null as nullSha1', ->
        refName = 'branches/' + Random.id()
        store.updateRef userName, {
            ownerName, repoName, refName, old: null, new: fakeCommitId
          }
        expect(findRepo().refs[refName]).to.be.equal fakeCommitId
        store.updateRef userName, {
            ownerName, repoName, refName, old: fakeCommitId, new: null
          }
        expect(findRepo().refs[refName]).to.be.equal nullSha1

      it 'refuses to update if old ref does not match.', ->
        refName = 'branches/master'
        expect(findRepo().refs[refName]).to.be.equal fakeCommitId
        fn = -> store.updateRef userName, {
            ownerName, repoName, refName, old: missingCommitId, new: nullSha1
          }
        expect(fn).to.throw '[ERR_REF_MISMATCH]'

      it 'updates a ref.', ->
        refName = 'branches/master'
        expect(findRepo().refs[refName]).to.be.equal fakeCommitId
        store.updateRef userName, {
            ownerName, repoName, refName, old: fakeCommitId, new: nullSha1
          }
        expect(findRepo().refs[refName]).to.be.equal nullSha1

      it 'throws if commit is missing.', ->
        refName = 'branches/' + Random.id()
        fn = -> store.updateRef userName, {
            ownerName, repoName, refName, old: nullSha1, new: missingCommitId
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'

      it 'rejects invalid ref names.', ->
        for refName in invalidRefNames
          fn = -> store.updateRef userName, {
              ownerName, repoName, refName, old: nullSha1, new: fakeCommitId
            }
          expect(fn).to.throw 'ref name'


    describe 'getRef()', ->
      it 'gets a ref.', ->
        refName = 'branches/' + Random.id()
        store.updateRef userName, {
            ownerName, repoName, refName, old: null, new: fakeCommitId
          }
        res = store.getRef userName, {ownerName, repoName, refName}
        expect(res).to.equal fakeCommitId

      it 'throws if the ref name is unknown.', ->
        fn = -> store.getRef userName, {
            ownerName, repoName, refName: 'unknown/ref'
          }
        expect(fn).to.throw '[ERR_REF_NOT_FOUND]'

      it 'rejects invalid ref names.', ->
        for refName in invalidRefNames
          fn = -> store.getRef userName, {
              ownerName, repoName, refName
            }
          expect(fn).to.throw 'ref name'

    describe 'getRefs()', ->
      it 'gets refs.', ->
        res = store.getRefs userName, {ownerName, repoName}
        expect(k for k of res).to.have.length.gt 1
        for name, sha1 of res
          expect(sha1, 'is a testing sha1').to.satisfy ->
            (sha1 is fakeCommitId) or (sha1 is nullSha1)


    describe 'repo content access checks', ->

      specs = [
          {
            name: 'getObject'
            action: 'get'
            opts: -> {sha1: fakeObjId_v1}
          }
          {
            name: 'createObject'
            action: 'modify'
            opts: -> {content: fakeObj}
          }
          {
            name: 'getTree'
            action: 'get'
            opts: -> {sha1: fakeTreeId}
          }
          {
            name: 'createTree'
            action: 'modify'
            opts: -> {content: fakeTree}
          }
          {
            name: 'getCommit'
            action: 'get'
            opts: -> {sha1: fakeCommitId}
          }
          {
            name: 'createCommit'
            action: 'modify'
            opts: ->
              commit = _.clone fakeCommit
              commit.tree = fakeTreeId
              commit.parents = []
              {content: commit}
          }
          {
            name: 'getRef'
            action: 'get'
            opts: -> {refName: 'branches/master'}
          }
          {
            name: 'getRefs'
            action: 'get'
            opts: -> {}
          }
          {
            name: 'updateRef'
            action: 'modify'
            opts: -> {refName: Random.id(), old: null, new: fakeCommitId}
          }
        ]

      for spc in specs
        do (spc) -> describe "#{spc.name}()", ->
          it 'rejects invalid owner and repo names.', ->
            for n in invalidNameOpts
              fn = -> store[spc.name] userName, _.extend(
                  {}, n, spc.opts()
                )
              expect(fn).to.throw 'simple name'

          it 'checks access.', ->
            deps.checkAccess.reset()
            store[spc.name] userName, _.extend {
                ownerName, repoName
              }, spc.opts()
            expect(deps.checkAccess).to.have.been.calledWith(
                userName, "nog-content/#{spc.action}", sinon.match {
                    ownerName, repoName
                  }
              )

          it 'throws if the repo is missing.', ->
            fn = -> store[spc.name] userName, _.extend {
                ownerName, repoName: 'invalid'
              }, spc.opts()
            expect(fn).to.throw '[ERR_REPO_MISSING]'

      describe 'copyEntry()', ->
        repoName2 = Random.id()

        it 'checks access on dest and source repo', ->
          repoFullName2 = ownerName + '/' + repoName2
          store.createRepo userName, {repoFullName: repoFullName2}
          deps.checkAccess.reset()
          store.copyEntry userName, {
              ownerName
              repoName: repoName2
              content:
                copy: {type: 'object', sha1: fakeObjId_v1, repoFullName}
            }
          expect(deps.checkAccess).to.have.been.calledWith(
              userName, "nog-content/modify", sinon.match {
                  ownerName, repoName: repoName2
                }
            )
          expect(deps.checkAccess).to.have.been.calledWith(
              userName, "nog-content/get", sinon.match {
                  ownerName, repoName: repoName
                }
            )

        it 'throws if the dest repo is missing.', ->
          fn = -> store.copyEntry userName, {
              ownerName
              repoName: 'invalid'
              content:
                copy: {type: 'object', sha1: fakeObjId_v1, repoFullName}
            }
          expect(fn).to.throw '[ERR_REPO_MISSING]'

        it 'throws if the source repo is missing.', ->
          fn = -> store.copyEntry userName, {
              ownerName
              repoName: repoName2
              content:
                copy: {
                  type: 'object', sha1: fakeObjId_v1,
                  repoFullName: "#{ownerName}/invalid"
                }
            }
          expect(fn).to.throw '[ERR_REPO_MISSING]'


  describe 'ReposApi', ->
    api = null
    actions = null
    fakeRepoName = null

    baseUrl = '/baseurl'
    fakeUser = 'fakeUser'
    basereq =
      baseUrl: baseUrl
      auth: {user: fakeUser}

    # `postedObject` will be set in POST and used to test GET.
    postedObject = null

    # `postedTree` will be set in POST and used to test GET.
    postedTree = null


    expectRefRes = (res, opts) ->
      {refName} = opts
      commitId = opts.commitId ? fakeCommitId
      expect(res._id.refName).to.equal refName
      expectToContainAll res._id.href,
        [baseUrl, fakeUser, fakeRepoName, 'db/refs', refName]
      expect(res.entry.type).to.equal 'commit'
      expect(res.entry.sha1).to.equal commitId
      expectToContainAll res.entry.href,
        [baseUrl, fakeUser, fakeRepoName, 'db/commits', commitId]

    expectCommitRes_v1 = (res, opts) ->
      idversion = opts.idversion ? 1
      fmtversion = opts.fmtversion ? idversion
      expect(res._id.sha1).to.exist
      expectToContainAll res._id.href,
        [baseUrl, fakeUser, fakeRepoName, 'db/commits', res._id.sha1]
      expect(res.subject).to.deep.equal opts.subject
      expect(res.message).to.deep.equal opts.message
      expect(res.tree.sha1).to.equal fakeTreeId
      expectToContainAll res.tree.href,
        [baseUrl, fakeUser, fakeRepoName, 'db/trees', fakeTreeId]
      expect(res.parents[0].sha1).to.equal fakeCommitId
      expectToContainAll res.parents[0].href,
        [baseUrl, fakeUser, fakeRepoName, 'db/commits', fakeCommitId]
      expect(res._idversion).to.equal idversion
      switch fmtversion
        when 0
          expect(res.authorDate).to.match rgxISOStringUTC
          expect(res.commitDate).to.match rgxISOStringUTC
        when 1
          expect(res.authorDate).to.match rgxISOStringTZ
          expect(res.commitDate).to.match rgxISOStringTZ
        else
          nogthrow ERR_LOGIC

    expectMinimalCommitRes_v1 = (res, opts) ->
      idversion = opts.idversion ? 1
      fmtversion = opts.fmtversion ? idversion
      expect(res._id).to.match rgxSha1
      expect(res.tree).to.match rgxSha1
      expect(res.parents[0]).to.match rgxSha1
      expect(res._idversion).to.equal idversion
      switch fmtversion
        when 0
          expect(res.authorDate).to.match rgxISOStringUTC
          expect(res.commitDate).to.match rgxISOStringUTC
        when 1
          expect(res.authorDate).to.match rgxISOStringTZ
          expect(res.commitDate).to.match rgxISOStringTZ


    expectTreeRes_v1 = (res) ->
      expect(res._id.sha1).to.exist
      expectToContainAll res._id.href,
        [baseUrl, fakeUser, fakeRepoName, 'db/trees', res._id.sha1]
      expect(res.name).to.deep.equal fakeTree.name
      expect(res.meta).to.deep.equal fakeTree.meta
      expect(res.entries).to.have.length 2
      expect(res.entries[0].sha1).to.equal fakeObjId_v1
      expectToContainAll res.entries[0].href,
        [baseUrl, fakeUser, fakeRepoName, 'db/objects', fakeObjId_v1]
      expect(res.entries[1].sha1).to.equal fakeTreeId
      expectToContainAll res.entries[1].href,
        [baseUrl, fakeUser, fakeRepoName, 'db/trees', fakeTreeId]
      expect(res._idversion).to.exist

    expectMinimalTreeRes_v1 = (res) ->
      expect(res._id).to.match rgxSha1
      expect(res.entries).to.have.length 2
      expect(res.entries[0].sha1).to.equal fakeObjId_v1
      expect(res.entries[0].href).to.not.exist
      expect(res.entries[1].sha1).to.equal fakeTreeId
      expect(res.entries[1].href).to.not.exist
      expect(res._idversion).to.exist


    expectObjectRes_v1 = (res, opts) ->
      idversion = opts.idversion ? 1
      fmtversion = opts.fmtversion ? idversion
      expect(res._id.sha1).to.exist
      expectToContainAll res._id.href,
        [baseUrl, fakeUser, fakeRepoName, 'db/objects', res._id.sha1]
      expect(res.name).to.equal fakeObj.name
      expect(res._idversion).to.equal idversion
      switch fmtversion
        when 0
          expect(res.meta).to.deep.equal fakeObj.meta
          expect(res.text).to.not.exist
          expect(res.blob).to.not.be.null
        when 1
          expect(res.text).to.equal fakeObj.meta.content
          expect(res.meta).to.deep.equal _.omit(fakeObj.meta, 'content')

    expectMinimalObjectRes_v1 = (res, opts) ->
      idversion = opts.idversion ? 1
      fmtversion = opts.fmtversion ? idversion
      expect(res._id).to.match rgxSha1
      expect(res._idversion).to.equal idversion
      switch fmtversion
        when 0
          expect(res.text).to.not.exist
          expect(res.blob).to.match rgxSha1
        when 1
          expect(res.text).to.not.be.undefined


    describeApi = (opts) ->
      {
        expectCommitRes
        expectMinimalCommitRes
        expectObjectRes
        expectMinimalObjectRes
        expectTreeRes
        expectMinimalTreeRes
      } = opts

      it "'POST /' checks access and creates a repo.", ->
        deps.checkAccess.reset()
        repoFullName = "#{fakeUser}/#{fakeRepoName}"
        req =
          body: {repoFullName}
        res = actions['POST /'](_.extend(req, basereq))
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/create-repo'
          )
        expect(res.statusCode).to.equal 201
        expect(res.fullName).to.equal repoFullName
        expect(res._id.href).to.contain repoFullName
        expect(deps.repos.find({name: fakeRepoName}).count()).to.equal 1

      it "
        'POST /:ownerName/:repoName/db/objects' checks access and creates db
        entry.
      ", ->
        deps.checkAccess.reset()
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
          body: fakeObj
        res = actions['POST /:ownerName/:repoName/db/objects'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expect(res.statusCode).to.equal 201
        postedObject = res
        expectObjectRes res, {idversion: 1}

      it "
        'POST /:ownerName/:repoName/db/objects?format=...' controls the result
        format.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            query: {}
          body: fakeObj
        req.params.query.format = 'hrefs'
        res = actions['POST /:ownerName/:repoName/db/objects'](
            _.extend(req, basereq)
          )
        expectObjectRes res, {idversion: 1}
        req.params.query.format = 'minimal'
        res = actions['POST /:ownerName/:repoName/db/objects'](
            _.extend(req, basereq)
          )
        expectMinimalObjectRes res, {idversion: 1}

      it "
        'GET /:ownerName/:repoName/db/objects/:sha1' checks access and returns
        the object.
      ", ->
        deps.checkAccess.reset()
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedObject._id.sha1
        res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/get'
          )
        expectObjectRes res, {idversion: 1}

      it "
        'GET /:ownerName/:repoName/db/objects/:sha1?format=...' controls the
        result format.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedObject._id.sha1
            query: {}
        req.params.query.format = 'hrefs'
        res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
            _.extend(req, basereq)
          )
        expectObjectRes res, {idversion: 1}
        req.params.query.format = 'minimal'
        res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
            _.extend(req, basereq)
          )
        expectMinimalObjectRes res, {idversion: 1}

      it "
        JSON for objects has a blob href.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedObject._id.sha1
        res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
            _.extend(req, basereq)
          )
        expect(res.blob.sha1).to.exist
        expect(res.blob.href).to.exist
        expectToContainAll res.blob.href,
          [baseUrl, fakeUser, fakeRepoName, 'db/blobs', res.blob.sha1]

      it "
        blob href creation can be configured.
      ", ->
        api.useBlobHrefs = false
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedObject._id.sha1
        res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
            _.extend(req, basereq)
          )
        expect(res.blob.sha1).to.exist
        expect(res.blob.href).to.not.exist
        api.useBlobHrefs = true

      it "
        JSON for objects contains errata.
      ", ->
        errata = [{ code: 'ERA000000' }]
        store.objects.update(
          { _id: postedObject._id.sha1 },
          { $set: { errata } }
        )
        req = {
          params: {
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedObject._id.sha1
          }
        }
        res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
          _.extend(req, basereq)
        )
        expect(res.errata).to.deep.equal(errata)
        store.objects.update(
          { _id: postedObject._id.sha1 },
          { $unset: { errata: '' } }
        )


      it "
        'POST /:ownerName/:repoName/db/trees' checks access and creates db entry.
      ", ->
        deps.checkAccess.reset()
        tree = _.clone fakeTree
        tree.entries = [
            {type: 'object', sha1: fakeObjId_v1}
            {type: 'tree', sha1: fakeTreeId}
          ]
        req =
          params: {ownerName: fakeUser, repoName: fakeRepoName}
          body: {tree: tree}
        res = actions['POST /:ownerName/:repoName/db/trees'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expect(res.statusCode).to.equal 201
        postedTree = res
        expectTreeRes res, req

      it "
        'POST /:ownerName/:repoName/db/trees' accepts an expanded tree.
      ", ->
        tree = _.clone fakeTree
        tr = _.clone fakeTree
        tr.entries = [fakeObj]
        tree.entries = [fakeObj, tr]
        req =
          params: {ownerName: fakeUser, repoName: fakeRepoName}
          body: {tree: tree}
        res = actions['POST /:ownerName/:repoName/db/trees'](
            _.extend(req, basereq)
          )
        expect(res.statusCode).to.equal 201
        expectTreeRes res, req

      it "
        'POST /:ownerName/:repoName/db/trees?format=...' controls the result
        format.
      ", ->
        tree = _.clone fakeTree
        tree.entries = [
            {type: 'object', sha1: fakeObjId_v1}
            {type: 'tree', sha1: fakeTreeId}
          ]
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            query: {}
          body: {tree: tree}
        req.params.query.format = 'hrefs'
        res = actions['POST /:ownerName/:repoName/db/trees'](
            _.extend(req, basereq)
          )
        expectTreeRes res, req
        req.params.query.format = 'minimal'
        res = actions['POST /:ownerName/:repoName/db/trees'](
            _.extend(req, basereq)
          )
        expectMinimalTreeRes res, req

      it "
        'GET /:ownerName/:repoName/db/trees/:sha1' checks access and returns
        the tree.
      ", ->
        deps.checkAccess.reset()
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedTree._id.sha1
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/get'
          )
        expectTreeRes res, req

      it "
        'GET /:ownerName/:repoName/db/trees/:sha1?format=...' controls the result
        format.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedTree._id.sha1
            query: {}
        req.params.query.format = 'hrefs'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expectTreeRes res, req
        req.params.query.format = 'minimal'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expectMinimalTreeRes res, req

      it "
        'GET /:ownerName/:repoName/db/trees/:sha1?expand=<levels>' reports match
        error for invalid <levels>.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedTree._id.sha1
            query: {}
        fn = -> actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        rgx = /// match.*error.*malformed.*param.*expand ///i
        req.params.query.expand = '00'
        expect(fn).to.throw rgx
        req.params.query.expand = '-1'
        expect(fn).to.throw rgx

      it "
        'POST /:ownerName/:repoName/db/commits' checks access and creates db
        entry.
      ", ->
        deps.checkAccess.reset()
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = [fakeCommitId]
        req =
          params: {ownerName: fakeUser, repoName: fakeRepoName}
          body: commit
        res = actions['POST /:ownerName/:repoName/db/commits'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expect(res.statusCode).to.equal 201
        expectCommitRes res, _.extend({idversion: 1}, commit)

      it "
        'POST /:ownerName/:repoName/db/commits?format=...' controls the result
        format.
      ", ->
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = [fakeCommitId]
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            query: {}
          body: commit
        req.params.query.format = 'hrefs'
        res = actions['POST /:ownerName/:repoName/db/commits'](
            _.extend(req, basereq)
          )
        expectCommitRes res, _.extend({idversion: 1}, commit)
        req.params.query.format = 'minimal'
        res = actions['POST /:ownerName/:repoName/db/commits'](
            _.extend(req, basereq)
          )
        expectMinimalCommitRes res, _.extend({idversion: 1}, commit)

      it "
        'POST /:ownerName/:repoName/db/bulk' creates objects, trees, and commits.
      ", ->
        deps.checkAccess.reset()
        commit = _.clone fakeCommit
        commit.tree = fakeTreeId
        commit.parents = []
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
          body:
            entries: [fakeObj, fakeTree, commit]
        res = actions['POST /:ownerName/:repoName/db/bulk'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expect(res.statusCode).to.equal 201
        expect(res.entries).to.have.length 3
        for i in [0...3]
          expect(res.entries[i].sha1).to.match rgxSha1
        expect(res.entries[0].type).to.equal 'object'
        expect(res.entries[1].type).to.equal 'tree'
        expect(res.entries[2].type).to.equal 'commit'

      it "
        'POST /:ownerName/:repoName/db/bulk' copies objects, trees, commits, and
        blobs.
      ", ->
        repoFullName = fakeUser + '/' + fakeRepoName
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
          body:
            entries: [
              {copy: {type: 'object', sha1: fakeObjId_v1, repoFullName}}
              {copy: {type: 'tree', sha1: fakeTreeId, repoFullName}}
              {copy: {type: 'commit', sha1: fakeCommitId, repoFullName}}
              {copy: {type: 'blob', sha1: fakeBlobId, repoFullName}}
            ]
        res = actions['POST /:ownerName/:repoName/db/bulk'](
            _.extend(req, basereq)
          )
        expect(res.statusCode).to.equal 201
        expect(res.entries).to.have.length 4
        for i in [0...4]
          expect(res.entries[i].sha1).to.match rgxSha1
        expect(res.entries[0].type).to.equal 'object'
        expect(res.entries[1].type).to.equal 'tree'
        expect(res.entries[2].type).to.equal 'commit'
        expect(res.entries[3].type).to.equal 'blob'

      it "
        'POST /:ownerName/:repoName/db/bulk' throws malformed.
      ", ->
        deps.checkAccess.reset()
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
          body:
            entries: [{invalid: 'object'}]
        fn = -> actions['POST /:ownerName/:repoName/db/bulk'](
            _.extend(req, basereq)
          )
        expect(fn).to.throw '[ERR_PARAM_MALFORMED]'

      it "
        'PATCH /:ownerName/:repoName/db/refs/:refName+' checks access and creates
        db entry.
      ", ->
        deps.checkAccess.reset()
        refName = 'branches/foo/bar'
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            refName: refName
          body:
            old: nullSha1
            new: fakeCommitId
        res = actions['PATCH /:ownerName/:repoName/db/refs/:refName+'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expectRefRes res, {refName}

      it "
        'GET /:ownerName/:repoName/db/refs/:refName+' checks access and returns
        an entry.
      ", ->
        deps.checkAccess.reset()
        refName = 'branches/foo/bar'
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            refName: refName
        res = actions['GET /:ownerName/:repoName/db/refs/:refName+'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/get'
          )
        expectRefRes res, {refName}

      it "
        'GET /:ownerName/:repoName/db/refs' checks access and returns items.
      ", ->
        deps.checkAccess.reset()
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
        res = actions['GET /:ownerName/:repoName/db/refs'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/get'
          )
        expect(res.count).to.equal res.items.length
        for item in res.items
          expectRefRes item, {
            refName: item._id.refName
            commitId: item.entry.sha1
          }


      # Will be created by POST and then removed by DELETE.
      refNamePost = null

      it "
        'POST /:ownerName/:repoName/db/refs' checks access and creates an entry.
      ", ->
        deps.checkAccess.reset()
        refNamePost = refName = 'branches/' + Random.id()
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
          body:
            refName: refName
            sha1: fakeCommitId
        res = actions['POST /:ownerName/:repoName/db/refs'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expect(res.statusCode).to.equal 201
        expectRefRes res, {refName}

      it "
        'DELETE /:ownerName/:repoName/db/refs/:refName+' checks access and
        delete the ref.
      ", ->
        deps.checkAccess.reset()
        refName = refNamePost
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            refName: refName
          body:
            old: fakeCommitId
        res = actions['DELETE /:ownerName/:repoName/db/refs/:refName+'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/modify'
          )
        expect(res.statusCode).to.equal 204

      it "
        'POST /:ownerName/:repoName/db/stat' returns whether objects, trees,
          blobs, and commits exist.
      ", ->
        deps.checkAccess.reset()
        knownEntries = [
            {type: 'object', sha1: postedObject._id.sha1}
            {type: 'tree', sha1: postedTree._id.sha1}
            {type: 'blob', sha1: fakeBlobId}
            {type: 'commit', sha1: fakeCommitId}
          ]
        unknownSha1 = '9999999999999999999999999999999999999999'
        fakeUploadingBlobId = '15ff15ff15ff15ff15ff15ff15ff15ff15ff15ff'
        deps.blobs.upsert fakeUploadingBlobId, {$set: {status: 'uploading'}}
        unknownEntries = [
            {type: 'object', sha1: unknownSha1}
            {type: 'tree', sha1: unknownSha1}
            {type: 'blob', sha1: unknownSha1}
            {type: 'commit', sha1: unknownSha1}
            {type: 'blob', sha1: fakeUploadingBlobId}
          ]
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
          body:
            entries: knownEntries.concat(unknownEntries)
        res = actions['POST /:ownerName/:repoName/db/stat'](
            _.extend(req, basereq)
          )
        expect(deps.checkAccess).to.have.been.calledWith(
            fakeUser, 'nog-content/get'
          )
        expect(res.entries).to.have.length req.body.entries.length
        for e in res.entries
          if e.sha1 == unknownSha1
            expect(e.status).to.equal 'unknown'
          else if e.sha1 == fakeUploadingBlobId
            expect(e.status).to.equal 'unknown'
          else
            expect(e.status).to.equal 'exists'


    describeCommitApi_v1 = ->
      it "
        'GET /:ownerName/:repoName/db/commits/:sha1' checks access and returns
        the commit.
      ", ->
        for fk in [
          {id: fakeCommitWithParentId_v0, idversion: 0}
          {id: fakeCommitWithParentId, idversion: 1}
        ]
          deps.checkAccess.reset()
          req =
            params:
              ownerName: fakeUser
              repoName: fakeRepoName
              sha1: fk.id
          res = actions['GET /:ownerName/:repoName/db/commits/:sha1'](
              _.extend(req, basereq)
            )
          expect(deps.checkAccess).to.have.been.calledWith(
              fakeUser, 'nog-content/get'
            )
          c = _.clone fakeCommit
          c.idversion = fk.idversion
          expectCommitRes_v1 res, c

      it "
        'GET /:ownerName/:repoName/db/commits/:sha1?format=...' controls the
        result format.
      ", ->
        for fk in [
          {id: fakeCommitWithParentId_v0, fmt: '', fmtv: 0, idv: 0}
          {id: fakeCommitWithParentId, fmt: '', fmtv: 1, idv: 1}
          {id: fakeCommitWithParentId_v0, fmt: '.v1', fmtv: 1, idv: 0}
          {id: fakeCommitWithParentId, fmt: '.v0', err: '[ERR_API_VERSION]'}
        ]
          expected = _.clone fakeCommit
          expected.idversion = fk.idv
          expected.fmtversion = fk.fmtv
          req =
            params:
              ownerName: fakeUser
              repoName: fakeRepoName
              sha1: fk.id
              query: {}
          req.params.query.format = 'hrefs' + fk.fmt
          fn = -> actions['GET /:ownerName/:repoName/db/commits/:sha1'](
              _.extend(req, basereq)
            )
          if (err = fk.err)?
            expect(fn).to.throw err
          else
            expectCommitRes_v1 fn(), expected
          req.params.query.format = 'minimal' + fk.fmt
          fn = -> actions['GET /:ownerName/:repoName/db/commits/:sha1'](
              _.extend(req, basereq)
            )
          if (err = fk.err)?
            expect(fn).to.throw err
          else
            expectMinimalCommitRes_v1 fn(), expected

      it "
        'POST /:ownerName/:repoName/db/commits' can control idversion.
      ", ->
        for spec in [
          {idv: 0}
          {idv: 1}
          {idv: 0, authorDate: '2000-01-01T00:00:00+00:00'}
          {idv: 0, commitDate: '2000-01-01T00:00:00+00:00'}
          {idv: 1, authorDate: '2000-01-01T00:00:00+00:00'}
          {idv: 1, commitDate: '2000-01-01T00:00:00+00:00'}
          {idv: 1, authorDate: '2000-01-01T00:00:00+01:00'}
          {idv: 1, commitDate: '2000-01-01T00:00:00-01:00'}
        ]
          commit = _.clone fakeCommit
          commit.tree = fakeTreeId
          commit.parents = [fakeCommitId]
          expected = _.clone commit
          expected.idversion = spec.idv
          commit._idversion = spec.idv
          if (d = spec.authorDate)?
            commit.authorDate = d
          if (d = spec.commitDate)?
            commit.commitDate = d
          req =
            params: {ownerName: fakeUser, repoName: fakeRepoName}
            body: commit
          res = actions['POST /:ownerName/:repoName/db/commits'](
              _.extend(req, basereq)
            )
          expect(res.statusCode).to.equal 201
          expectCommitRes_v1 res, expected

      it "
        'POST /:ownerName/:repoName/db/commits' throws if idversion is 0 and
         date has tz.
      ", ->
        for spec in [
          {idv: 0, authorDate: '2000-01-01T00:00:00+01:00'}
          {idv: 0, commitDate: '2000-01-01T00:00:00-01:00'}
        ]
          commit = _.clone fakeCommit
          commit.tree = fakeTreeId
          commit.parents = [fakeCommitId]
          commit._idversion = spec.idv
          if (d = spec.authorDate)?
            commit.authorDate = d
          if (d = spec.commitDate)?
            commit.commitDate = d
          req =
            params: {ownerName: fakeUser, repoName: fakeRepoName}
            body: commit
          fn = -> actions['POST /:ownerName/:repoName/db/commits'](
              _.extend(req, basereq)
            )
          expect(fn).to.throw '[ERR_PARAM_INVALID]'


    describeObjectsApi_v1 = ->
      it "
        'GET /:ownerName/:repoName/db/objects/:sha1?format=...' controls the
        result format.
      ", ->
        for fk in [
          {id: fakeObjId_v0, fmt: '', fmtv: 0, idv: 0}
          {id: fakeObjId_v0, fmt: '.v0', fmtv: 0, idv: 0}
          {id: fakeObjId_v0, fmt: '.v1', fmtv: 1, idv: 0}
          {id: fakeObjId_v1, fmt: '', fmtv: 1, idv: 1}
          {id: fakeObjId_v1, fmt: '.v0', fmtv: 0, idv: 1}
          {id: fakeObjId_v1, fmt: '.v1', fmtv: 1, idv: 1}
        ]
          expected = {
            idversion: fk.idv
            fmtversion: fk.fmtv
          }
          req =
            params:
              ownerName: fakeUser
              repoName: fakeRepoName
              sha1: fk.id
              query: {}
          req.params.query.format = 'hrefs' + fk.fmt
          res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
              _.extend(req, basereq)
            )
          expectObjectRes_v1 res, expected
          req.params.query.format = 'minimal' + fk.fmt
          res = actions['GET /:ownerName/:repoName/db/objects/:sha1'](
              _.extend(req, basereq)
            )
          expectMinimalObjectRes_v1 res, expected

      it "
        'POST /:ownerName/:repoName/db/objects' can control idversion.
      ", ->
        for spec in [
          {idv: 0}
          {idv: 1}
        ]
          obj = _.clone fakeObj
          obj._idversion = spec.idv
          req =
            params:
              ownerName: fakeUser
              repoName: fakeRepoName
            body: obj
          res = actions['POST /:ownerName/:repoName/db/objects'](
              _.extend(req, basereq)
            )
          expect(res.statusCode).to.equal 201
          expectObjectRes_v1 res, {idversion: spec.idv}


    describeTreesApi_v1 = ->
      postedTree_v1 = null

      it "
        'POST /:ownerName/:repoName/db/trees' handles idversion 1 objects.
      ", ->
        tree = _.clone fakeTree
        tree.entries = [
            {type: 'object', sha1: fakeObjId_v1}
            {type: 'tree', sha1: fakeTreeId}
          ]
        req =
          params: {ownerName: fakeUser, repoName: fakeRepoName}
          body: {tree: tree}
        res = actions['POST /:ownerName/:repoName/db/trees'](
            _.extend(req, basereq)
          )
        expect(res.statusCode).to.equal 201
        postedTree_v1 = res

      it "
        'GET /:ownerName/:repoName/db/trees/:sha1?format=...' supports '.v0'
        suffix.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedTree._id.sha1
            query: {}
        req.params.query.format = 'hrefs.v0'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expectTreeRes_v1 res, req
        req.params.query.format = 'minimal'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expectMinimalTreeRes_v1 res, req

      it "
        'GET /:ownerName/:repoName/db/trees/:sha1?expand=<levels>' expands
        entries recursively.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedTree_v1._id.sha1
            query:
              expand: '0'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expect(res.entries[0].sha1).to.exist  # collapsed
        req.params.query.expand = '1'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expect(res.entries[0].blob).to.exist
        expect(res.entries[0].blob.href).to.exist
        expect(res.entries[0]._id.sha1).to.exist
        expect(res.entries[0]._id.href).to.exist
        expect(res.entries[0]._idversion).to.exist
        expect(res.entries[0].text).to.exist
        expect(res.entries[0].meta.content).to.be.undefined
        expect(res.entries[1].entries).to.exist
        expect(res.entries[1]._id.sha1).to.exist
        expect(res.entries[1]._id.href).to.exist
        expect(res.entries[1].entries[0].sha1).to.exist  # collapsed
        req.params.query.expand = '2'
        res = actions['GET /:ownerName/:repoName/db/trees/:sha1'](
            _.extend(req, basereq)
          )
        expect(res.entries[1].entries[0].blob).to.exist
        expect(res.entries[1].entries[0].blob.href).to.exist
        expect(res.entries[1].entries[0]._idversion).to.exist
        expect(res.entries[1].entries[0].text).to.exist
        expect(res.entries[1].entries[0].meta.content).to.be.undefined

      it "
        'GET /:ownerName/:repoName/db/trees/:sha1?format=<fmt>.v#' version
        suffix can only be used with expand=0.
      ", ->
        req =
          params:
            ownerName: fakeUser
            repoName: fakeRepoName
            sha1: postedTree._id.sha1
        for spec in [
          {throws: false, query: {expand: '0', format: 'minimal.v0'}}
          {throws: false, query: {expand: '0', format: 'hrefs.v0'}}
          {throws: false, query: {expand: '1', format: 'minimal'}}
          {throws: false, query: {expand: '1', format: 'hrefs'}}
          {throws: true, query: {expand: '1', format: 'minimal.v0'}}
          {throws: true, query: {expand: '1', format: 'hrefs.v0'}}
        ]
          req.params.query = spec.query
          fn = -> actions['GET /:ownerName/:repoName/db/trees/:sha1'](
              _.extend(req, basereq)
            )
          if spec.throws
            expect(fn).to.throw '[ERR_PARAM_INVALID]'
          else
            fn()  # Does not throw.


    describe 'v1', ->
      before ->
        fakeRepoName = Random.id()
        api = new NogContentTest.ReposApi {store}
        actions = {}
        for a in api.actions_v1()
          actions[a.method + ' ' + a.path] = a.action

      describeApi {
        expectCommitRes: expectCommitRes_v1
        expectMinimalCommitRes: expectMinimalCommitRes_v1
        expectObjectRes: expectObjectRes_v1
        expectMinimalObjectRes: expectMinimalObjectRes_v1
        expectTreeRes: expectTreeRes_v1
        expectMinimalTreeRes: expectMinimalTreeRes_v1
      }
      describeCommitApi_v1()
      describeObjectsApi_v1()
      describeTreesApi_v1()


# Test methods invocation only on the server, since it is easier and the code
# path on the client is identical except for the early return.

methodSpecs = [
  {
    method: 'createRepo'
    action: 'nog-content/create-repo'
    opts:
      repoFullName: 'userfoo/repobar'
    acheck: sinon.match
      ownerName: sinon.match.string
      repoName: sinon.match.string
  }
]

describe 'nog-content', -> describe 'methods', ->
  fakeUser = Random.id()

  # User `configure()` to install a spy instead of direct `sinon.stub
  # NogContent`, since `configure()` properly re-initializes the internal
  # objects.
  origCfg = null
  userStub = null
  before ->
    origCfg = NogContent.configure
      checkAccess: sinon.spy -> throw 'checkAccessSpy'
    userStub = sinon.stub(Meteor, 'user').callsFake -> fakeUser
  after ->
    NogContent.configure origCfg
    userStub.restore()

  # Return from the access check by throwing from `checkAccessSpy()`.  The
  # implementation is already covered by direct tests on the `Store`.
  for spec in methodSpecs
    do (spec) ->
      it "#{spec.method}() passes opts and checks access.", ->
        NogContent.checkAccess.reset()
        fn = -> NogContent.call[spec.method] spec.opts
        expect(fn).to.throw 'checkAccessSpy'
        expect(NogContent.checkAccess).to.have.been.calledWith(
            fakeUser, spec.action, spec.acheck
          )

# Test with a fake collection and the real RepoSets (not the CachedRepoSets) in
# order to verify that the repo set code correctly operates on the Mongo
# collection.

fakeRepoSets = new Mongo.Collection 'testing.repo_sets'

# The tests below could be refactored into two groups: 1) test the low-level
# RepoSets; 2) test that Store calls RepoSets.  But since we're primarily
# interested in the joint effect, the refactoring might not be worth it.
describe 'nog-content', -> describe 'repo membership check', ->
  deps = null
  store = null
  repoSets = null
  ownerName = 'userbar'
  ownerId = 'userbarId'
  repoName = null
  repoName2 = null

  @timeout(10000)
  before ->
    deps = _.pick NogContent, 'repos', 'commits', 'trees', 'objects'
    deps.blobs = fakeBlobs
    deps.users = fakeUsers
    deps.repoSets = fakeRepoSets
    resetCollections deps
    deps.repoSets = repoSets = new NogContentTest.RepoSets deps
    deps.blobs.upsert fakeBlobId, {$set: {status: 'available'}}
    deps.users.insert {_id: ownerId, username: ownerName}
    deps.users.insert {_id: fakeUserDoc._id, username: fakeUserDoc.username}
    deps.checkAccess = sinon.spy()
    store = new NogContentTest.Store deps

    repoName = Random.id()
    repoName2 = Random.id()
    store.createRepo ownerName, {repoFullName: "#{ownerName}/#{repoName}"}
    store.createRepo ownerName, {repoFullName: "#{ownerName}/#{repoName2}"}

  # It's not obvious how to check blob access, since blob handling is
  # implemented in a different package.  Ideas:
  #
  #  - Duplicate the BlobsApi in nog-content and provide a different
  #    implementation that is aware of repo membership.  The REST API would
  #    enforce strict checks, although other code paths via method calls
  #    would provide access if only the blob sha1 is known.
  #  - Change all access checks to expect a repo owner and name and implement
  #    access check statements that use the information to check repo
  #    membership.
  #  - Move blob code to nog-content.  Change everything to be repo-aware,
  #    that is blob upload and download must happen through a repo URL.
  #    Nothing can be done without a repo.
  #  - Ignore question and accept that blob access is only loosely checked.
  #
  # => Ignore it for now, put on tech debt list, and reconsider later.

  it 'finds recent objects that are unrefed', ->
    repoSets.updateMembership {
        ownerName, repoName
      }, {
        type: 'blob', sha1: fakeBlobId
      }
    sha1 = store.createObject ownerName, {
        ownerName, repoName, content: fakeObj
      }
    expect(sha1).to.eql fakeObjId_v1
    store.getObject ownerName, {ownerName, repoName, sha1}
    expect(repoSets.isMember {ownerName, repoName, sha1}).to.be.true
    sel = {ownerName, repoName, sha1: fakeBlobId}
    expect(repoSets.isMember sel).to.be.true

  it 'finds recent trees that are unrefed', ->
    tree = _.clone fakeTree
    tree.entries = [{type: 'object', sha1: fakeObjId_v1}]
    sha1 = store.createTree ownerName, {
        ownerName, repoName, content: tree
      }
    expect(sha1).to.eql fakeTreeId
    store.getTree ownerName, {ownerName, repoName, sha1}
    expect(repoSets.isMember {ownerName, repoName, sha1}).to.be.true

  it 'finds recent commits that are unrefed', ->
    commit = _.clone fakeCommit
    commit.tree = fakeTreeId
    commit.parents = []
    fakeCommitId = sha1 = store.createCommit ownerName, {
        ownerName, repoName, content: commit
      }
    store.getCommit ownerName, {ownerName, repoName, sha1}
    expect(repoSets.isMember {ownerName, repoName, sha1}).to.be.true
    commit.parents = [fakeCommitId]
    fakeCommitWithParentId = sha1 = store.createCommit ownerName, {
        ownerName, repoName, content: commit
      }
    store.getCommit ownerName, {ownerName, repoName, sha1}
    expect(repoSets.isMember {ownerName, repoName, sha1}).to.be.true
    # Ref the commit for tests below.
    refName = 'branches/master'
    store.updateRef ownerName, {
        ownerName, repoName, refName, old: nullSha1, new: sha1
      }

  it 'throws when accessing an object through a foreign repo', ->
    sel = {ownerName, repoName: repoName2, sha1: fakeObjId_v1}
    blobSel = {ownerName, repoName: repoName2, sha1: fakeBlobId}
    expect(repoSets.isMember sel).to.be.false
    expect(repoSets.isMember blobSel).to.be.false
    fn = -> store.getObject ownerName, sel
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'object'

  it 'throws when accessing a tree through a foreign repo', ->
    sel = {ownerName, repoName: repoName2, sha1: fakeTreeId}
    expect(repoSets.isMember sel).to.be.false
    fn = -> store.getTree ownerName, sel
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'tree'

  it 'throws when accessing a commit through a foreign repo', ->
    sel = {ownerName, repoName: repoName2, sha1: fakeCommitId}
    expect(repoSets.isMember sel).to.be.false
    fn = -> store.getCommit ownerName, sel
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'commit'

  it 'throws when trying to ref a commit in a foreign repo', ->
    sel = {ownerName, repoName: repoName2, sha1: fakeCommitId}
    expect(repoSets.isMember sel).to.be.false
    refName = 'branches/master'
    fn = -> store.getCommit ownerName, sel
    # Ref the commit for tests below.
    fn = -> store.updateRef ownerName, {
        ownerName, repoName: repoName2,
        refName, old: nullSha1, new: fakeCommitId
      }
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'commit'

  it 'throws when createObject is missing blob in foreign repo', ->
    fn = -> store.createObject ownerName, {
        ownerName, repoName: repoName2, content: fakeObj
      }
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'blob'

  it 'throws when createTree is missing object in foreign repo', ->
    tree = _.clone fakeTree
    tree.entries = [{type: 'object', sha1: fakeObjId_v1}]
    fn = -> store.createTree ownerName, {
        ownerName, repoName: repoName2, content: tree
      }
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'object'

  it 'throws when createTree is missing tree in foreign repo', ->
    tree = _.clone fakeTree
    tree.entries = [{type: 'tree', sha1: fakeTreeId}]
    fn = -> store.createTree ownerName, {
        ownerName, repoName: repoName2, content: tree
      }
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'tree'

  it 'throws when createCommit is missing tree in foreign repo', ->
    commit = _.clone fakeCommit
    commit.tree = fakeTreeId
    commit.parents = []
    fn = -> store.createCommit ownerName, {
        ownerName, repoName: repoName2, content: commit
      }
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'tree'

  it 'throws when createCommit is missing parent in foreign repo', ->
    commit = _.clone fakeCommit
    commit.tree = fakeTreeId
    commit.parents = [fakeCommitId]
    fn = -> store.createCommit ownerName, {
        ownerName, repoName: repoName2, content: commit
      }
    expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    expect(fn).to.throw 'commit'

  it 'finds refed objects with a clear cache', ->
    fakeRepoSets.remove {}
    sel = {ownerName, repoName, sha1: fakeObjId_v1}
    blobSel = {ownerName, repoName, sha1: fakeBlobId}
    expect(repoSets.isMember sel).to.be.true
    expect(repoSets.isMember blobSel).to.be.true
    store.getObject ownerName, sel

  it 'finds refed trees with a clear cache', ->
    fakeRepoSets.remove {}
    sel = {ownerName, repoName, sha1: fakeTreeId}
    expect(repoSets.isMember sel).to.be.true
    store.getTree ownerName, sel

  it 'finds refed commits with a clear cache', ->
    fakeRepoSets.remove {}
    sel = {ownerName, repoName, sha1: fakeCommitWithParentId}
    expect(repoSets.isMember sel).to.be.true
    store.getCommit ownerName, sel

  it 'returns stat "exists" for known entries with a clear cache', ->
    fakeRepoSets.remove {}
    opts = {
      ownerName, repoName,
      entries: [
        {type: 'blob', sha1: fakeBlobId}
        {type: 'object', sha1: fakeObjId_v1}
        {type: 'tree', sha1: fakeTreeId}
        {type: 'commit', sha1: fakeCommitId}
        {type: 'commit', sha1: fakeCommitWithParentId}
      ]
    }
    stat = store.stat ownerName, opts
    for s in stat
      expect(s.status).to.equal 'exists'

  it 'returns stat "unknown" through a foreign repo', ->
    opts = {
      ownerName, repoName: repoName2,
      entries: [
        {type: 'blob', sha1: fakeBlobId}
        {type: 'object', sha1: fakeObjId_v1}
        {type: 'tree', sha1: fakeTreeId}
        {type: 'commit', sha1: fakeCommitId}
        {type: 'commit', sha1: fakeCommitWithParentId}
      ]
    }
    stat = store.stat ownerName, opts
    for s in stat
      expect(s.status).to.equal 'unknown'

  it 'refused to copy entries from a foreign repo', ->
    repoFullName = ownerName + '/' + repoName
    repoFullName2 = ownerName + '/' + repoName2
    repoNameDest = Random.id()
    repoFullNameDest = ownerName + '/' + repoNameDest
    store.createRepo ownerName, {repoFullName: repoFullNameDest}
    specs = [
        {type: 'blob', sha1: fakeBlobId}
        {type: 'object', sha1: fakeObjId_v1}
        {type: 'tree', sha1: fakeTreeId}
        {type: 'commit', sha1: fakeCommitId}
      ]
    for s in specs
        fn = -> store.copyEntry ownerName, {
            ownerName,
            repoName: repoNameDest
            content:
              copy: _.extend({repoFullName: repoFullName2}, s)
          }
        expect(fn).to.throw '[ERR_CONTENT_MISSING]'
    # Confirm that copy works from repo that has the entries.
    for s in specs
        store.copyEntry ownerName, {
            ownerName,
            repoName: repoNameDest
            content:
              copy: _.extend({repoFullName}, s)
          }
        sel = {ownerName, repoName: repoNameDest, sha1: s.sha1}
        expect(repoSets.isMember sel).to.be.true


describe 'nog-content', -> describe 'repo membership check', ->
  it 'can be configured via optStrictRepoMembership', ->
    expect(NogContent.store.repoSets).to.exist
    Meteor.settings.optStrictRepoMembership = false
    NogContent.configure()
    expect(NogContent.store.repoSets).to.not.exist
    Meteor.settings.optStrictRepoMembership = true
    NogContent.configure()
    expect(NogContent.store.repoSets).to.exist


describe 'nog-content', -> describe 'id versions', ->
  for test in testdata.tests
    do (test) -> describe "#{test.name}", ->
      coll = switch test.type
        when 'commit' then NogContent.commits
        when 'object' then NogContent.objects
        when 'tree' then NogContent.trees

      it 'canonical id matches.', ->
        canonical = _.omit(test.canonical, '_id')
        id = contentId(canonical)
        expect(id).to.equal test.canonical._id

      it 'stores expected raw.', ->
        coll.remove {_id: test.canonical._id}
        create coll, test.canonical
        raw = coll.findOne test.canonical._id, RAW
        expect(raw).to.deep.equal test.mongo

      it 'stores expected raw with errata, ignoring errata in sha1.', ->
        errata = { errata: [{ code: 'ERA'}] }
        canonical = _.extend({}, test.canonical, errata)
        mongo = _.extend({}, test.mongo, errata)
        coll.remove({ _id: canonical._id })
        create(coll, canonical)
        raw = coll.findOne(canonical._id, RAW)
        expect(raw).to.deep.equal(mongo)

      it 'detects id checksum errors', ->
        raw = coll.findOne test.canonical._id, RAW
        raw._id = '0000000000' + raw._id[10..]
        coll.insert raw
        fn = -> coll.findOne raw._id
        expect(fn).to.throw '[ERR_CONTENT_CHECKSUM]'
        expect(fn).to.throw test.type

      it 'ignores `_idversion` during create.', ->
        coll.remove {_id: test.canonical._id}
        create coll, _.extend({_idversion: 0}, test.canonical)
        raw = coll.findOne test.canonical._id, RAW
        expect(raw).to.deep.equal test.mongo

      it 'find() creates `_idversion`.', ->
        latest = coll.findOne test.canonical._id
        expect(latest._idversion).to.equal test.idversion

      if test.type == 'commit'
        it 'partial find() does not create `_idversion`.', ->
          latest = coll.findOne {
            _id: test.canonical._id
          }, {
            fields: {authorDate: 1}
          }
          expect(latest._idversion).to.not.exist

          latest = coll.findOne {
            _id: test.canonical._id
          }, {
            fields: {commitDate: 1}
          }
          expect(latest._idversion).to.not.exist

      if test.type == 'object'
        it 'partial find() does not create `_idversion`.', ->
          latest = coll.findOne {
            _id: test.canonical._id
          }, {
            fields: {name: 1, blob: 1}
          }
          expect(latest._idversion).to.not.exist
