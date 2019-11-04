/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

import chai from 'chai';
import sinon from 'sinon';
import sinonChai from 'sinon-chai';
chai.use(sinonChai);
const { expect } = chai;

import { EJSON } from 'meteor/ejson';
import { Meteor } from 'meteor/meteor';
import { Mongo } from 'meteor/mongo';
import { Random } from 'meteor/random';
import { _ } from 'meteor/underscore';
import { PublicationCollector } from 'meteor/johanbrook:publication-collector';

import { NogContent } from 'meteor/nog-content';
const {
  createContentCollections,
  createContentStore,
} = NogContent;

import { createCatalogServerModule } from 'meteor/nog-catalog';


function makePubName(namespace, basename) {
  return `${namespace.pub}.${basename}`;
}

function makeCollName(namespace, basename) {
  return `${namespace.coll}.${basename}`;
}

function makeUnknownVolumeName() {
  return `unknown.vol_${Random.id()}_44444`;
}

function makeWellformedVolumeName() {
  return `wellformed.vol_${Random.id()}_1`;
}


function createFakeRateLimiters() {
  const readLimiter = {
    op(value) {
      console.log(`[nog-catalog] readlimiter.op(${value})`);
    },
  };
  const writeLimiter = {
    op(value) {
      console.log(`[nog-catalog] writelimiter.op(${value})`);
    },
  };
  return { readLimiter, writeLimiter };
}


function createTestContext() {
  const checkAccess = sinon.spy();
  const testAccess = sinon.spy(() => true);
  const rnd = `test_catalog_${Random.id().toLowerCase()}`;
  const namespace = {
    coll: `coll_${rnd}`,
    meth: `meth_${rnd}`,
    pub: `pub_${rnd}`,
  };

  // See <http://docs.meteor.com/#/full/meteor_users> for user document format.
  const userDocs = {
    catOwner: { _id: `catOwnerId-${rnd}`, username: `catOwner-${rnd}` },
    repoOwner: { _id: `repoOwnerId-${rnd}`, username: `repoOwner-${rnd}` },
  };
  const users = new Mongo.Collection(`${rnd}.users`);
  for (const doc of _.values(userDocs)) {
    users.insert(doc);
  }

  const contentColls = createContentCollections({
    namespace,
  });
  const contentStore = createContentStore({
    users,
    blobs: null,
    repoSets: null,
    checkAccess,
    ...contentColls,
  });

  const rateLimiters = createFakeRateLimiters();

  const nogCatalog = createCatalogServerModule({
    namespace,
    contentStore,
    checkAccess,
    testAccess,
    rateLimiters,
  });

  return {
    rnd, namespace, contentStore, userDocs, checkAccess, testAccess,
    nogCatalog,

    destroy() {
      const colls = ['repos', 'commits', 'trees', 'objects', 'users'];
      for (const c of colls) {
        this.contentStore[c].rawCollection().drop();
      }
    },
  };
}


function createCatalogFaker({
  euid = null,
  ownerDoc: owner,
  contentStore: store,
  catalogConfig: config,
}) {
  const ownerName = owner.username;
  const repoName = `repo-${Random.id()}`;
  const repoFullName = [ownerName, repoName].join('/');
  const repoId = store.createRepo(euid, { repoFullName });
  const treeId = store.createTree(euid, {
    ownerName, repoName,
    content: {
      name: `root ${repoFullName}`,
      entries: [],
      meta: {
        catalog: { config: EJSON.stringify(config, { canonical: true }) },
      },
    },
  });
  const commitId = store.createCommit(euid, {
    ownerName, repoName,
    content: {
      subject: 'init',
      message: 'init root tree',
      meta: {},
      parents: [],
      tree: treeId,
    },
  });
  store.updateRef(euid, {
    ownerName, repoName,
    refName: 'branches/master',
    new: commitId, old: null,
  });

  return {
    store, ownerName, repoName, repoId,

    updateConfig({ catalogConfig: newConfig }) {
      const refName = 'branches/master';
      const refs = store.getRefs(euid, { ownerName, repoName });
      const master = refs[refName];
      const newTreeId = store.createTree(euid, {
        ownerName, repoName,
        content: {
          name: `root ${repoFullName}`,
          entries: [],
          meta: {
            catalog: {
              config: EJSON.stringify(newConfig, { canonical: true }),
            },
          },
        },
      });
      const newCommitId = store.createCommit(euid, {
        ownerName, repoName,
        content: {
          subject: 'add object',
          message: '',
          meta: {},
          parents: [master],
          tree: newTreeId,
        },
      });
      store.updateRef(euid, {
        ownerName, repoName, refName, new: newCommitId, old: master,
      });
    },
  };
}


function createRepoFaker({
  euid = null,
  ownerDoc: owner,
  repoName,
  contentStore: store,
}) {
  const ownerName = owner.username;
  const repoFullName = [ownerName, repoName].join('/');
  const repoId = store.createRepo(euid, { repoFullName });
  const treeId = store.createTree(euid, {
    ownerName, repoName,
    content: {
      name: `root ${repoFullName}`,
      entries: [],
      meta: {},
    },
  });
  const commitId = store.createCommit(euid, {
    ownerName, repoName,
    content: {
      subject: 'init',
      message: 'init root tree',
      meta: {},
      parents: [],
      tree: treeId,
    },
  });
  store.updateRef(euid, {
    ownerName, repoName,
    refName: 'branches/master',
    new: commitId, old: null,
  });

  return {
    repoId,
    ownerName, repoName, repoFullName,
    store,
    commitId,
    nObjects: 0,

    addObject({ name, meta }) {
      const refName = 'branches/master';
      const refs = store.getRefs(euid, { ownerName, repoName });
      const master = refs[refName];
      const commit = store.getCommit(euid, {
        ownerName, repoName, sha1: master,
      });
      const tree = store.getTree(euid, {
        ownerName, repoName, sha1: commit.tree,
      });
      const objectId = store.createObject(euid, {
        ownerName, repoName,
        content: { name, blob: null, meta },
      });

      const newTreeId = store.createTree(euid, {
        ownerName, repoName, content: {
          name: tree.name,
          meta: {},
          entries: [
            ...tree.entries,
            { type: 'object', sha1: objectId },
          ],
        },
      });
      const newCommitId = store.createCommit(euid, {
        ownerName, repoName,
        content: {
          subject: 'add object',
          message: '',
          meta: {},
          parents: [master],
          tree: newTreeId,
        },
      });
      store.updateRef(euid, {
        ownerName, repoName, refName, new: newCommitId, old: master,
      });

      this.nObjects += 1;
      this.commitId = newCommitId;
      return { commitId: newCommitId, objectId };
    },
  };
}


describe('nog-catalog', function () {
  describe('configureCatalog()', function () {
    let tc = null;

    before(function () {
      tc = createTestContext();
    });

    after(function () {
      tc.destroy();
    });

    function getRootTree(euid, { ownerName, repoName }) {
      const store = tc.contentStore;
      const refName = 'branches/master';
      const refs = store.getRefs(euid, { ownerName, repoName });
      const master = refs[refName];
      const commit = store.getCommit(euid, {
        ownerName, repoName, sha1: master,
      });
      return store.getTree(euid, {
        ownerName, repoName, sha1: commit.tree,
      });
    }

    it('writes the config in canonical EJSON to the root tree.', function () {
      const { nogCatalog } = tc;
      const catFaker = createRepoFaker({
        ownerDoc: tc.userDocs.catOwner,
        repoName: 'catalog',
        contentStore: tc.contentStore,
      });
      const catalogConfig = {
        preferredMetaKeys: ['project'],
        contentRepoConfigs: [
          {
            repoSelector: { name: 'foo' },
            pipeline: [
              { $select: { 'meta.project': { $exists: true } } },
            ],
          },
        ],
      };
      const catalogConfigJson = EJSON.stringify(
        catalogConfig, { canonical: true },
      );

      const { ownerName, repoName } = catFaker;
      const euid = null;
      nogCatalog.configureCatalog(euid, {
        ownerName, repoName, catalogConfig,
      });

      const root = getRootTree(euid, { ownerName, repoName });
      expect(root.meta.catalog.config).to.eql(catalogConfigJson);

      tc.catFaker = catFaker;
    });

    function itRejects(name, expected, catalogConfig) {
      it(`rejects invalid config, ${name}.`, function () {
        const { nogCatalog, catFaker } = tc;
        const { ownerName, repoName } = catFaker;
        const euid = null;
        const fn = () => nogCatalog.configureCatalog(euid, {
          ownerName, repoName, catalogConfig,
        });
        expect(fn).to.throw(expected);
      });
    }

    itRejects(
      'missing `preferredMetaKeys`',
      /Match error.*Missing.*preferredMetaKeys/,
      {
        contentRepoConfigs: [],
      },
    );

    itRejects(
      'missing `contentRepoConfigs`',
      /Match error.*Missing.*contentRepoConfigs/,
      {
        preferredMetaKeys: [],
      },
    );

    itRejects(
      'invalid pipeline step',
      /Match error.*field.*contentRepoConfigs.*pipeline/,
      {
        preferredMetaKeys: [],
        contentRepoConfigs: [
          {
            repoSelector: {},
            pipeline: [{ $invalid: { foo: 1 } }],
          },
        ],
      },
    );
  });

  describe('updateCatalog()', function () {
    let tc = null;

    before(function () {
      tc = createTestContext();
    });

    after(function () {
      tc.destroy();
    });

    it('creates a catalog - minimal init check.', function () {
      const catFaker = createCatalogFaker({
        ownerDoc: tc.userDocs.catOwner,
        contentStore: tc.contentStore,
        catalogConfig: {
          preferredMetaKeys: ['project', 'specimen'],
          contentRepoConfigs: [
            {
              repoSelector: { name: 'foo' },
              pipeline: [
                { $select: { 'meta.project': { $exists: true } } },
              ],
            },
          ],
        },
      });

      const repoFaker = createRepoFaker({
        ownerDoc: tc.userDocs.repoOwner,
        repoName: 'foo',
        contentStore: tc.contentStore,
      });
      const { commitId, objectId } = repoFaker.addObject({
        name: 'a',
        meta: { project: 'prjA', specimen: 'spcA' },
      });

      const euid = null;
      tc.nogCatalog.updateCatalog(euid, {
        ownerName: catFaker.ownerName,
        repoName: catFaker.repoName,
      });

      const catalogsCollName = `${tc.namespace.coll}.catalogs`;
      expect(tc.nogCatalog.catalogs._name).to.eql(catalogsCollName);
      const catalogs = new Mongo.Collection(
        catalogsCollName,
        { defineMutationMethods: false },
      );
      const catalog = catalogs.findOne(catFaker.repoId);
      expect(catalog).to.exist;

      const serial = 1;
      const volume = new Mongo.Collection(
        `${tc.namespace.coll}.catalogs.vol_${catFaker.repoId}_${serial}`,
        { defineMutationMethods: false },
      );

      const item = volume.findOne();
      expect(item).to.exist;
      expect(item._id).to.eql(objectId);
      expect(item.name).to.eql('a');
      expect(item.m.m0).to.eql(['prjA']);
      expect(item.m.m1).to.eql(['spcA']);
      expect(item.refpaths).to.deep.eql([
        {
          repoId: repoFaker.repoId,
          owner: repoFaker.ownerName,
          repo: repoFaker.repoName,
          commitId,
          path: 'a',
        },
      ]);

      tc.catFaker = catFaker;
      tc.repoFaker = repoFaker;
    });

    function getActiveVolume() {
      const { catFaker, nogCatalog } = tc;
      const catalog = nogCatalog.catalogs.findOne(catFaker.repoId);
      const volume = nogCatalog.volumes[catalog.active.volumes[0].name];
      const volumeRaw = volume.rawCollection();
      volumeRaw.indexesSync = Meteor.wrapAsync(volumeRaw.indexes, volumeRaw);
      return { volume, volumeRaw };
    }

    it('creates MongoDB indexes.', function () {
      const { volumeRaw } = getActiveVolume();
      const indexes = volumeRaw.indexesSync().map(idx => idx.name);
      expect(indexes).contains('m0');
      expect(indexes).contains('m1');
    });

    it('computes meta key stats.', function () {
      const { catFaker, nogCatalog } = tc;
      const catalog = nogCatalog.catalogs.findOne(catFaker.repoId);
      expect(catalog.active.metaKeyCounts).to.deep.eql({
        m0: 1, m1: 1,
      });
    });

    it('updates a catalog when the content repo changes.', function () {
      const { catFaker, repoFaker, nogCatalog } = tc;

      const { objectId } = repoFaker.addObject({
        name: 'b',
        meta: { project: 'prjB', specimen: 'spcB', detail: 'dtlB' },
      });

      const euid = null;
      nogCatalog.updateCatalog(euid, {
        ownerName: catFaker.ownerName,
        repoName: catFaker.repoName,
      });

      const { volume } = getActiveVolume();
      const item = volume.findOne(objectId);
      expect(item).to.exist;
      expect(item.name).to.eql('b');
      expect(item.m.m0).to.eql(['prjB']);
      expect(item.m.m1).to.eql(['spcB']);
      expect(item.m.m2).to.eql(['dtlB']);
      expect(item.refpaths).to.deep.eql([
        {
          repoId: repoFaker.repoId,
          owner: repoFaker.ownerName,
          repo: repoFaker.repoName,
          commitId: repoFaker.commitId,
          path: 'b',
        },
      ]);
    });

    it('adds MongoDB indexes for new meta fields.', function () {
      const { volumeRaw } = getActiveVolume();
      const indexes = volumeRaw.indexesSync().map(idx => idx.name);
      expect(indexes).contains('m2');
    });

    it('updates meta key stats for new meta fields.', function () {
      const { catFaker, nogCatalog } = tc;
      const catalog = nogCatalog.catalogs.findOne(catFaker.repoId);
      expect(catalog.active.metaKeyCounts).to.deep.eql({
        m0: 2, m1: 2, m2: 1,
      });
    });

    it('updates a catalog when the catalog config changes.', function () {
      const { catFaker, repoFaker, nogCatalog } = tc;
      catFaker.updateConfig({
        catalogConfig: {
          preferredMetaKeys: ['detail'],
          contentRepoConfigs: [
            {
              repoSelector: { name: 'foo' },
              pipeline: [
                { $select: { 'meta.detail': { $exists: true } } },
              ],
            },
          ],
        },
      });

      const euid = null;
      nogCatalog.updateCatalog(euid, {
        ownerName: catFaker.ownerName,
        repoName: catFaker.repoName,
      });

      const { volume } = getActiveVolume();
      expect(volume.find({}).count()).to.eql(1);
      const item = volume.findOne();
      expect(item).to.exist;
      expect(item.name).to.eql('b');
      expect(item.m.m0).to.eql(['dtlB']);
      expect(item.m.m1).to.eql(['prjB']);
      expect(item.m.m2).to.eql(['spcB']);
      expect(item.refpaths).to.deep.eql([
        {
          repoId: repoFaker.repoId,
          owner: repoFaker.ownerName,
          repo: repoFaker.repoName,
          commitId: repoFaker.commitId,
          path: 'b',
        },
      ]);
    });

    it('updates meta key stats when the config changes.', function () {
      const { catFaker, nogCatalog } = tc;
      const catalog = nogCatalog.catalogs.findOne(catFaker.repoId);
      expect(catalog.active.metaKeyCounts).to.deep.eql({
        m0: 1, m1: 1, m2: 1,
      });
    });

    it('updates a catalog when a repo has been deleted.', function () {
      const {
        contentStore, catFaker, repoFaker, nogCatalog,
      } = tc;

      const euid = null;
      contentStore.deleteRepo(euid, {
        ownerName: repoFaker.ownerName,
        repoName: repoFaker.repoName,
      });
      nogCatalog.updateCatalog(euid, {
        ownerName: catFaker.ownerName,
        repoName: catFaker.repoName,
      });

      const { volume } = getActiveVolume();
      expect(volume.find({}).count()).to.eql(0);

      const catalog = nogCatalog.catalogs.findOne(catFaker.repoId);
      expect(catalog.active.metaKeyCounts).to.deep.eql({
        m0: 0, m1: 0, m2: 0,
      });
    });

    it('removes stale catalogs after catalog repo re-create.', function () {
      const { contentStore, catFaker, nogCatalog } = tc;
      const { ownerName, repoName } = catFaker;
      const euid = null;

      const master = contentStore.getRef(
        euid, { ownerName, repoName, refName: 'branches/master' },
      );
      contentStore.deleteRepo(euid, { ownerName, repoName });
      contentStore.createRepo(
        euid, { repoFullName: `${ownerName}/${repoName}` },
      );
      contentStore.updateRef(euid, {
        ownerName, repoName,
        refName: 'branches/master', new: master, old: null,
      });
      nogCatalog.updateCatalog(euid, { ownerName, repoName });

      expect(
        nogCatalog.catalogs.find({ owner: ownerName, name: repoName }).count(),
      ).to.eql(1);
    });
  });


  describe('publications', function () {
    let tc = null;

    before(function () {
      tc = createTestContext();
    });

    after(function () {
      tc.destroy();
    });

    it('Setup catalog for publication tests.', function () {
      const { nogCatalog } = tc;
      const catFaker = createRepoFaker({
        ownerDoc: tc.userDocs.catOwner,
        repoName: 'catalog',
        contentStore: tc.contentStore,
      });

      const catalogConfig = {
        preferredMetaKeys: ['project'],
        contentRepoConfigs: [
          {
            repoSelector: {},
            pipeline: [
              { $select: { 'meta.project': { $exists: true } } },
            ],
          },
        ],
      };
      const { ownerName, repoName } = catFaker;
      const euid = null;
      nogCatalog.configureCatalog(euid, {
        ownerName, repoName, catalogConfig,
      });

      const repoFaker = createRepoFaker({
        ownerDoc: tc.userDocs.repoOwner,
        repoName: 'content',
        contentStore: tc.contentStore,
      });
      for (let i = 0; i < 4; i++) { // eslint-disable-line no-plusplus
        repoFaker.addObject({
          name: `content-${i}`,
          meta: { project: 'prj', specimen: `spc-${i}` },
        });
      }

      nogCatalog.updateCatalog(euid, { ownerName, repoName });

      tc.catFaker = catFaker;
      tc.repoFaker = repoFaker;
    });

    describe('publishCatalog', function () {
      it('publishes the active catalog state.', function (done) {
        const { namespace } = tc;
        const { ownerName, repoName, repoId } = tc.catFaker;
        const sub = new PublicationCollector();
        const opts = { ownerName, repoName };
        sub.collect(makePubName(namespace, 'catalog'), opts, (colls) => {
          const catalogs = colls[makeCollName(namespace, 'catalogs')];
          expect(catalogs).to.have.length(1);
          const c = catalogs[0];
          const { active } = c;
          expect(c._id).to.eql(repoId);
          expect(c.owner).to.eql(ownerName);
          expect(c.name).to.eql(repoName);
          expect(active).to.exist;
          expect(active.volumes).to.have.length(1);
          expect(active.metaKeys).to.deep.eql(['project', 'specimen']);
          expect(active.metaKeyCounts).to.deep.eql({ m0: 4, m1: 4 });
          done();
        });
      });
    });

    describe('publishCatalogHitCount', function () {
      let namespace;
      let nogCatalog;
      let ownerName;
      let repoName;
      let nObjects;
      let catalog;
      let volumeName;
      const pub = 'catalogHitCount';

      before(function () {
        ({ namespace, nogCatalog } = tc);
        ({ ownerName, repoName } = tc.catFaker);
        ({ nObjects } = tc.repoFaker);
        catalog = nogCatalog.catalogs.findOne();
        volumeName = catalog.active.volumes[0].name;
      });

      // The next two tests are skipped, because they cause spurious failures.
      // The result looks fine in the reporter, but the summary failure count
      // increases by one.  The reason for the spurious failures is unclear.
      // The tests may be temporarily useful during development but should not
      // be unskipped permanently to avoid confusion when running the full test
      // suite.

      it.skip('publishes a counter for the active catalog state; ' +
      'causes spurious failures.', function (done) {
        const sub = new PublicationCollector();
        const opts = {
          ownerName, repoName, volumeName,
          filter: '',
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          const counters = colls['counters-collection'];
          expect(counters[0]).to.deep.eql({
            _id: volumeName, count: nObjects,
          });
          done();
        });
      });

      it.skip('uses filter to select; causes spurious failures.' +
      '', function (done) {
        const sub = new PublicationCollector();
        const filter = 'm1:spc-1';
        const opts = {
          ownerName, repoName, volumeName, filter,
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          const counters = colls['counters-collection'];
          expect(counters[0]).to.deep.eql({
            _id: volumeName, count: 1,
          });
          done();
        });
      });

      it('rejects unknown volumeName.', function (done) {
        const sub = new PublicationCollector();
        const opts = {
          ownerName, repoName, volumeName: makeUnknownVolumeName(), filter: '',
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          expect(colls).to.deep.eql({});
          done();
        });
      });

      it('rejects volumeName with wrong serial.', function (done) {
        const sub = new PublicationCollector();
        const vn = volumeName.replace(/\d$/, '99999999');
        const opts = {
          ownerName, repoName, volumeName: vn, filter: '',
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          expect(colls).to.deep.eql({});
          done();
        });
      });
    });

    describe('publishCatalogVolume', function () {
      let namespace;
      let nogCatalog;
      let ownerName;
      let repoName;
      let catalog;
      let volumeName;
      const pub = 'catalogVolume';

      before(function () {
        ({ namespace, nogCatalog } = tc);
        ({ ownerName, repoName } = tc.catFaker);
        catalog = nogCatalog.catalogs.findOne();
        volumeName = catalog.active.volumes[0].name;
      });

      it('publishes the catalog volume.', function (done) {
        const sub = new PublicationCollector();
        const opts = {
          ownerName, repoName, volumeName,
          filter: '',
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          const vol = colls[volumeName];
          expect(vol).to.have.length(4);
          expect(vol.map(v => v.name)).to.contain('content-0');
          expect(vol.map(v => v.m.m0[0])).to.contain('prj');
          expect(vol.map(v => v.m.m1[0])).to.contain('spc-3');
          done();
        });
      });

      it('uses filter to select.', function (done) {
        const sub = new PublicationCollector();
        const filter = 'm1:spc-1';
        const opts = {
          ownerName, repoName, volumeName, filter,
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          const vol = colls[volumeName];
          expect(vol).to.have.length(1);
          expect(vol[0].m.m1).to.eql(['spc-1']);
          done();
        });
      });

      it('rejects invalid volumeName.', function (done) {
        const sub = new PublicationCollector();
        const opts = {
          ownerName, repoName, volumeName: makeUnknownVolumeName(), filter: '',
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          expect(colls).to.deep.eql({});
          done();
        });
      });

      it('rejects volumeName with wrong serial.', function (done) {
        const sub = new PublicationCollector();
        const vn = volumeName.replace(/\d$/, '99999999');
        const opts = {
          ownerName, repoName, volumeName: vn, filter: '',
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          expect(colls).to.deep.eql({});
          done();
        });
      });
    });

    describe('publishCatalogVolumeStats', function () {
      let namespace;
      let nogCatalog;
      let ownerName;
      let repoName;
      let catalog;
      let volumeName;
      const pub = 'catalogVolumeStats';

      before(function () {
        ({ namespace, nogCatalog } = tc);
        ({ ownerName, repoName } = tc.catFaker);
        catalog = nogCatalog.catalogs.findOne();
        volumeName = catalog.active.volumes[0].name;
      });

      it('publishes the catalog volumes stats.', function (done) {
        const sub = new PublicationCollector();
        const opts = {
          ownerName, repoName, volumeName, field: 'm0', limit: 1,
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          const stats = colls[`${volumeName}.stats`];
          expect(stats).to.have.length(1);
          expect(stats[0].field).to.eql('m0');
          expect(stats[0].val).to.eql('prj');
          expect(stats[0].count).to.eql(4);
          done();
        });
      });

      function itPublishesWithLimit(limit) {
        it(`publishes the catalog volumes stats with limit=${limit}.` +
        '', function (done) {
          const sub = new PublicationCollector();
          const opts = {
            ownerName, repoName, volumeName, field: 'm1', limit,
          };
          sub.collect(makePubName(namespace, pub), opts, (colls) => {
            const stats = colls[`${volumeName}.stats`];
            expect(stats).to.have.length(limit);
            expect(stats[0].field).to.eql('m1');
            expect(stats[0].count).to.eql(1);
            done();
          });
        });
      }
      // eslint-disable-next-line no-plusplus
      for (let limit = 1; limit < 5; limit++) {
        itPublishesWithLimit(limit);
      }

      it('rejects invalid volumeName.', function (done) {
        const sub = new PublicationCollector();
        const opts = {
          ownerName, repoName, volumeName: makeUnknownVolumeName(),
          field: 'm1', limit: 5,
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          expect(colls).to.deep.eql({});
          done();
        });
      });

      it('rejects volumeName with wrong serial.', function (done) {
        const sub = new PublicationCollector();
        const vn = volumeName.replace(/\d$/, '99999999');
        const opts = {
          ownerName, repoName, volumeName: vn, field: 'm1', limit: 5,
        };
        sub.collect(makePubName(namespace, pub), opts, (colls) => {
          expect(colls).to.deep.eql({});
          done();
        });
      });
    });
  });

  describe('access checks', function () {
    let tc = null;

    before(function () {
      tc = createTestContext();
    });

    after(function () {
      tc.destroy();
    });

    it('updateCatalog() checks catalog access.', function () {
      const { nogCatalog, checkAccess } = tc;
      const catFaker = createCatalogFaker({
        ownerDoc: tc.userDocs.catOwner,
        contentStore: tc.contentStore,
        catalogConfig: {
          preferredMetaKeys: [],
          contentRepoConfigs: [
            {
              repoSelector: { name: 'repo' },
              pipeline: [],
            },
          ],
        },
      });

      checkAccess.reset();
      const euid = Random.id();
      nogCatalog.updateCatalog(euid, {
        ownerName: catFaker.ownerName,
        repoName: catFaker.repoName,
      });

      expect(checkAccess).to.have.been.calledWith(
        euid, 'nog-catalog/update', sinon.match({
          ownerName: catFaker.ownerName,
          repoName: catFaker.repoName,
        }),
      );

      tc.catFaker = catFaker;
    });

    it('updateCatalog() checks content repo access.', function () {
      const { nogCatalog, testAccess, catFaker } = tc;
      const repoFaker = createRepoFaker({
        ownerDoc: tc.userDocs.repoOwner,
        repoName: 'repo',
        contentStore: tc.contentStore,
      });

      testAccess.reset();
      const euid = Random.id();
      nogCatalog.updateCatalog(euid, {
        ownerName: catFaker.ownerName,
        repoName: catFaker.repoName,
      });

      expect(testAccess).to.have.been.calledWith(
        euid, 'nog-content/get', sinon.match({
          ownerName: repoFaker.ownerName,
          repoName: repoFaker.repoName,
        }),
      );
    });

    it('configureCatalog() checks catalog access.', function () {
      const { nogCatalog, checkAccess } = tc;
      const repoFaker = createRepoFaker({
        ownerDoc: tc.userDocs.repoOwner,
        repoName: `repo-${Random.id()}`,
        contentStore: tc.contentStore,
      });
      const { ownerName, repoName } = repoFaker;

      checkAccess.reset();
      const euid = Random.id();
      nogCatalog.configureCatalog(euid, {
        ownerName, repoName,
        catalogConfig: { preferredMetaKeys: [], contentRepoConfigs: [] },
      });

      expect(checkAccess).to.have.been.calledWith(
        euid, 'nog-catalog/configure', sinon.match({
          ownerName, repoName,
        }),
      );
    });

    function itPubChecksAccess(publishName, moreOpts = {}) {
      const stripped = publishName.replace(/^publish/, '');
      const pubname = `${stripped[0].toLowerCase()}${stripped.slice(1)}`;

      it(`${publishName} checks content access.`, function (done) {
        const { namespace, catFaker, testAccess } = tc;
        const { ownerName, repoName } = catFaker;
        const euid = Random.id();
        const sub = new PublicationCollector({ userId: euid });
        testAccess.reset();
        const opts = { ownerName, repoName, ...moreOpts };
        sub.collect(makePubName(namespace, pubname), opts, () => {
          expect(testAccess).to.have.been.calledWith(
            euid, 'nog-content/get', sinon.match({ ownerName, repoName }),
          );
          done();
        });
      });
    }

    const volFilter = {
      volumeName: makeWellformedVolumeName(), filter: '',
    };
    const volFieldLimit = {
      volumeName: makeWellformedVolumeName(), field: 'm0', limit: 10,
    };
    itPubChecksAccess('publishCatalog');
    itPubChecksAccess('publishCatalogHitCount', volFilter);
    itPubChecksAccess('publishCatalogVolume', volFilter);
    itPubChecksAccess('publishCatalogVolumeStats', volFieldLimit);
  });


  describe('updateCatalog() - many repos', function () {
    let tc = null;

    before(function () {
      tc = createTestContext();
    });

    after(function () {
      tc.destroy();
    });

    it('volume, indexes and key stats for 100 repos.', function () {
      const nRepos = 100;
      this.timeout(nRepos * 300);
      const { nogCatalog } = tc;

      const catFaker = createCatalogFaker({
        ownerDoc: tc.userDocs.catOwner,
        contentStore: tc.contentStore,
        catalogConfig: {
          preferredMetaKeys: ['project'],
          contentRepoConfigs: [
            {
              repoSelector: {},
              pipeline: [
                { $select: { 'meta.project': { $exists: true } } },
              ],
            },
          ],
        },
      });

      for (let i = 0; i < nRepos; i++) { // eslint-disable-line no-plusplus
        const repoFaker = createRepoFaker({
          ownerDoc: tc.userDocs.repoOwner,
          repoName: `repo-${i}`,
          contentStore: tc.contentStore,
        });
        repoFaker.addObject({
          name: 'a',
          meta: { project: `prj-${i}`, [`detail-${i}`]: `detail-${i}` },
        });

        const euid = null;
        tc.nogCatalog.updateCatalog(euid, {
          ownerName: catFaker.ownerName,
          repoName: catFaker.repoName,
        });

        const volume = _.values(nogCatalog.volumes)[0];
        expect(volume.find({}).count()).to.eql(i + 1);
      }

      const volume = _.values(nogCatalog.volumes)[0];
      const volumeRaw = volume.rawCollection();
      volumeRaw.indexesSync = Meteor.wrapAsync(volumeRaw.indexes, volumeRaw);
      const indexes = volumeRaw.indexesSync().map(idx => idx.name);
      const catalogMaxNumMetaIndexes = 32;
      // eslint-disable-next-line no-plusplus
      for (let i = 0; i < catalogMaxNumMetaIndexes; i++) {
        expect(indexes).contains(`m${i}`);
      }
      expect(indexes).to.not.contain(`m${catalogMaxNumMetaIndexes}`);

      const catalog = nogCatalog.catalogs.findOne(catFaker.repoId);
      const { metaKeyCounts } = catalog.active;
      expect(metaKeyCounts.m0).to.eql(nRepos);
      for (let i = 1; i < nRepos; i++) { // eslint-disable-line no-plusplus
        expect(metaKeyCounts[`m${i}`]).to.eql(1);
      }
    });
  });
});
