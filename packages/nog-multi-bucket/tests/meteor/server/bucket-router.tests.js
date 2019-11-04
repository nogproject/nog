/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable no-unused-expressions */

// Tests must be run in a full application, because `nog-multi-bucket` depends
// on the peer npm package `aws` from the main application.  Tests, therefore,
// cannot be run as package tests, because `aws` would be missing.

import { Meteor } from 'meteor/meteor';
import { Random } from 'meteor/random';
import { _ } from 'meteor/underscore';
import chai from 'chai';
import sinon from 'sinon';
import sinonChai from 'sinon-chai';
const expect = chai.expect;
chai.use(sinonChai);

import { createBucketRouterFromSettings } from 'meteor/nog-multi-bucket';

const fakeSettings = {
  readPrefs: ['noglocal3', 'noglocal', 'noglocal2'],
  writePrefs: ['noglocal2', 'noglocal', 'noglocal3'],
  fallback: 'noglocal',
  buckets: [
    {
      name: 'noglocal',
      region: 'eu-central-1',
      accessKeyId: 'AK0xxxxxxxxxxxxxxxxx',
      secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
    },
    {
      name: 'noglocal2',
      region: 'eu-central-1',
      accessKeyId: 'AK2xxxxxxxxxxxxxxxxx',
      secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
    },
    {
      name: 'noglocal3',
      region: 'eu-west-1',
      accessKeyId: 'AK3xxxxxxxxxxxxxxxxx',
      secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
    },
  ],
};

let testingNamespace;
let router;


describe('nog-multi-bucket', function () {
  before(function () {
    testingNamespace = {
      coll: `test${Random.id()}`,
    };

    router = createBucketRouterFromSettings({
      settings: fakeSettings,
      namespace: testingNamespace,
    });
  });

  describe('createBucketRouterFromSettings()', function () {
    it('uses collection namespace', function () {
      const ns = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({
        settings: fakeSettings,
        namespace: ns,
      });
      expect(rtr.urlCache._name).to.contain(ns.coll);
      expect(rtr.urlCache._name).to.contain('urlcache');
    });

    it('supports non-AWS endpoints with path style.', function () {
      const settings = {
        readPrefs: [],
        writePrefs: [],
        fallback: 'ceph',
        buckets: [
          {
            name: 'ceph',
            endpoint: 'https://objs.local',
            accessKeyId: 'AKCEPHxxxxxxxxxxxxxx',
            secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
          },
        ],
      };
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const filename = 'foo.dat';
      const url = rtr.getDownloadUrl({ blob, filename });
      expect(url).to.contain('objs.local/ceph');
      expect(url).to.contain('=AKCEPHxxxxxxxxxxxxxx');
    });
  });

  describe('getDownloadUrl()', function () {
    it('creates content-disposition url.', function () {
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const filename = 'foo.dat';

      const url = router.getDownloadUrl({ blob, filename });

      expect(url).to.contain(blob._id);
      expect(url).to.contain('content-disposition');
      expect(url).to.contain(filename);
    });

    it('uses the fallback bucket.', function () {
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const filename = 'foo.dat';

      const url = router.getDownloadUrl({ blob, filename });

      expect(url).to.contain(`https://${fakeSettings.fallback}.s3`);
    });

    it('uses read order preferences for locs that are online.', function () {
      const { readPrefs, fallback } = fakeSettings;
      const filename = 'bar.dat';

      const specs = [
        {
          locs: [
            { bucket: readPrefs[2], status: 'online' },
            { bucket: readPrefs[1], status: 'online' },
            { bucket: readPrefs[0], status: 'online' },
          ],
          choose: readPrefs[0], key: 'AK3',
        },
        {
          locs: [
            { bucket: readPrefs[2], status: 'online' },
            { bucket: readPrefs[1], status: 'online' },
          ],
          choose: readPrefs[1], key: 'AK0',
        },
        {
          locs: [
            { bucket: readPrefs[2], status: 'online' },
          ],
          choose: readPrefs[2], key: 'AK2',
        },
        {
          locs: [
            { bucket: readPrefs[2], status: 'online' },
            { bucket: readPrefs[1], status: 'online' },
            { bucket: readPrefs[0], status: 'copying' },
          ],
          choose: readPrefs[1], key: 'AK0',
        },
        {
          locs: [
            { bucket: 'unknown' },
          ],
          choose: fallback, key: 'AK0',
        },
        {
          locs: [
            { bucket: readPrefs[2], status: 'copying' },
            { bucket: readPrefs[1], status: 'copying' },
            { bucket: readPrefs[0], status: 'copying' },
          ],
          choose: fallback, key: 'AK0',
        },
      ];

      for (const spec of specs) {
        const url = router.getDownloadUrl({
          filename,
          blob: {
            _id: 'acacacacacacacacacacacacacacacacacacacac',
            locs: spec.locs,
          },
        });
        expect(url).to.contain(`https://${spec.choose}.s3`);
        expect(url).to.contain(`=${spec.key}`);
      }
    });

    it('does not cache urls', function () {
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const filename = 'foo.dat';

      const url = router.getDownloadUrl({ blob, filename });
      Meteor._sleepForMs(1100);  // Ensure expire changed.
      const url2 = router.getDownloadUrl({ blob, filename });

      expect(url).to.not.eql(url2);
    });
  });

  describe('getImgSrc()', function () {
    it('creates mimetype url.', function () {
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const specs = [
        { ext: 'png', mime: 'image/png' },
        { ext: 'tif', mime: 'image/tiff' },
        { ext: 'tiff', mime: 'image/tiff' },
        { ext: 'jpg', mime: 'image/jpeg' },
        { ext: 'jpeg', mime: 'image/jpeg' },
        { ext: 'unknown', mime: 'image/png' },
      ];

      for (const spec of specs) {
        const filename = `foo.${spec.ext}`;
        const url = router.getImgSrc({ blob, filename });
        expect(url).to.contain(blob._id);
        expect(url).to.contain('response-content-type=');
        expect(url).to.contain(spec.mime.replace('/', '%2F'));
      }
    });

    it('uses cache control', function () {
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const filename = 'foo.png';
      const url = router.getImgSrc({ blob, filename });
      expect(url).to.contain('response-cache-control=');
    });

    it('caches urls', function () {
      const blob = { _id: 'fefefefefefefefefefefefefefefefefefefefe' };
      const filename = 'foo.png';

      const url = router.getImgSrc({ blob, filename });
      Meteor._sleepForMs(1100);  // Ensure expire changed.
      const url2 = router.getImgSrc({ blob, filename });

      // `urlCache` is an implementation detail.  Verify it nonetheless to
      // confirm that the state is persistet to the database.

      expect(url).to.eql(url2);
      expect(router.urlCache.findOne({ url })).to.exist;
    });
  });

  // The AWS SDK functions that would access the AWS API are replaced by stubs
  // for testing the multi-part upload in order to avoid requests to the real
  // AWS API.

  describe('multi-part upload', function () {
    const bucket = fakeSettings.writePrefs[0];
    let uploadId;
    let stubs;
    const request = {
      on(event, cb) {
        this[event] = cb;
      },
      send() {
        Meteor.defer(() => this[this.resultEvent](this.result));
      },
    };

    beforeEach(function () {
      uploadId = Random.id();
      stubs = {
        createMultipartUpload: sinon.stub(
          router.s3s[bucket], 'createMultipartUpload',
        ).callsFake((opts) => {
            request.resultEvent = 'success';
            request.result = {
              data: { UploadId: uploadId, ...opts },
            };
            return request;
          },
        ),

        completeMultipartUpload: sinon.stub(
          router.s3s[bucket], 'completeMultipartUpload',
        ).callsFake(() => {
            request.resultEvent = 'success';
            request.result = { data: {} };
            return request;
          },
        ),

        abortMultipartUpload: sinon.stub(
          router.s3s[bucket], 'abortMultipartUpload',
        ).callsFake(() => {
            request.resultEvent = 'success';
            request.result = { data: {} };
            return request;
          },
        ),
      };
    });

    afterEach(function () {
      stubs.createMultipartUpload.restore();
      stubs.completeMultipartUpload.restore();
      stubs.abortMultipartUpload.restore();
    });

    it('complete code path', function () {
      const key = 'abababababababababababababababababababab';
      const upload = router.createMultipartUpload({ key });
      expect(stubs.createMultipartUpload).to.have.been.calledWith(
          sinon.match({ Bucket: bucket, Key: key }),
      );
      expect(upload.UploadId).to.eql(uploadId);
      expect(upload.Bucket).to.eql(bucket);
      expect(upload.Key).to.eql(key);

      const url = router.getSignedUploadPartUrl({ PartNumber: 1, ...upload });
      expect(url).to.contain(`https://${bucket}`);
      expect(url).to.contain(key);
      expect(url).to.contain('partNumber=1');
      expect(url).to.contain(`uploadId=${uploadId}`);

      const parts = [
        { PartNumber: 1, ETag: Random.id() },
      ];
      router.completeMultipartUpload({
        ...upload,
        MultipartUpload: { Parts: parts },
      });
      expect(stubs.completeMultipartUpload).to.have.been.calledWith(
          sinon.match({
            Bucket: bucket,
            Key: key,
            UploadId: uploadId,
            MultipartUpload: { Parts: parts },
          }),
      );
    });

    it('abort code path', function () {
      const key = 'abababababababababababababababababababab';
      const upload = router.createMultipartUpload({ key });
      expect(stubs.createMultipartUpload).to.have.been.calledWith(
          sinon.match({ Bucket: bucket, Key: key }),
      );

      router.abortMultipartUpload(upload);
      expect(stubs.abortMultipartUpload).to.have.been.calledWith(
          sinon.match(upload),
      );
    });
  });

  describe('health checks', function () {
    function settingsWithActive(opts) {
      const settings = {
        readPrefs: ['active'],
        writePrefs: ['active'],
        fallback: 'fallback',
        buckets: [
          {
            name: 'active',
            region: 'eu-central-1',
            accessKeyId: 'AK0xxxxxxxxxxxxxxxxx',
            secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
            ...opts,
          },
          {
            name: 'fallback',
            region: 'eu-central-1',
            accessKeyId: 'AK0xxxxxxxxxxxxxxxxx',
            secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
          },
        ],
      };
      return settings;
    }

    const blob = {
      _id: 'fefefefefefefefefefefefefefefefefefefefe',
      locs: [{ bucket: 'active', status: 'online' }],
    };
    const filename = 'foo.png';

    it('check="healthy" selects bucket', function () {
      const settings = settingsWithActive({ check: 'healthy' });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const url = rtr.getDownloadUrl({ blob, filename });

      expect(url).to.contain('active');
    });

    it('check="unhealthy" rejects bucket', function () {
      const settings = settingsWithActive({ check: 'unhealthy' });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const url = rtr.getDownloadUrl({ blob, filename });

      expect(url).to.contain('fallback');
    });

    it('check="toggle" starts healthy and can be controlled', function () {
      const settings = settingsWithActive({ check: 'toggle' });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const url = rtr.getDownloadUrl({ blob, filename });
      expect(url).to.contain('active');

      rtr.checks.active.status = false;
      const url2 = rtr.getDownloadUrl({ blob, filename });
      expect(url2).to.contain('fallback');
    });

    it('check="random", failure prob 0', function () {
      const settings = settingsWithActive({
        check: 'random',
        checkFailureProb: 0,
      });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const url = rtr.getDownloadUrl({ blob, filename });
      expect(url).to.contain('active');
    });

    it('check="random", failure prob 1', function () {
      const settings = settingsWithActive({
        check: 'random',
        checkFailureProb: 1,
      });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const url = rtr.getDownloadUrl({ blob, filename });
      expect(url).to.contain('fallback');
    });

    it('check="getObject" starts unhealthy', function () {
      const settings = settingsWithActive({
        check: 'getObject',
        checkKey: 'can-read',
        checkContent: 'from-bucket-active',
        checkInterval: '15s',
      });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const url = rtr.getDownloadUrl({ blob, filename });
      expect(url).to.contain('fallback');

      // Toggling to healthy is currently not automatically tested.  It needs
      // to be tested manually.  Since we rely on bucket health checks in
      // production, we should quickly discover if something is broken.
    });

    // We test only one upload case, because the test setup with stubs is a bit
    // awkward.  One test should be sufficient to verify that the bucket
    // selection does something at all.

    it('check="unhealthy" rejects upload bucket', function () {
      const settings = settingsWithActive({ check: 'unhealthy' });
      const namespace = { coll: `test${Random.id()}` };
      const rtr = createBucketRouterFromSettings({ settings, namespace });

      const uploadId = Random.id();
      const request = {
        on(event, cb) {
          this[event] = cb;
        },
        send() {
          Meteor.defer(() => this[this.resultEvent](this.result));
        },
      };
      const stubs = {};
      for (const [bkt, s3] of _.pairs(rtr.s3s)) {
        stubs[bkt] = sinon.stub(
          s3, 'createMultipartUpload',
        ).callsFake((opts) => {
          request.resultEvent = 'success';
          request.result = {
            data: { UploadId: uploadId, ...opts },
          };
          return request;
        });
      }

      const key = 'abababababababababababababababababababab';
      rtr.createMultipartUpload({ key });

      expect(stubs.fallback).to.have.been.calledWith(
          sinon.match({ Bucket: 'fallback', Key: key }),
      );

      for (const stub of _.values(stubs)) {
        stub.restore();
      }
    });
  });
});
