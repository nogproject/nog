/* eslint-env mocha */
/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */

import { expect } from 'chai';
import { NogBlobTest } from 'meteor/nog-blob';
const { Hasher } = NogBlobTest;
import { createMd5Hasher } from './nog-blob-client-md5.js';


// `SHA1s.zeroX` is the SHA1 of all-zero data of the indicated size.  Compute
// them, for examples, with:
//
// ```
// head -c 1k /dev/zero | sha1sum
// head -c 2G /dev/zero | sha1sum
// ```

const SHA1s = {
  zero1kB: '60cacbf3d72e1e7834203da608037b1bf83b40e8',
  zero2GiB: '91d50642dd930e9542c39d36f0516d45f4e1af0d',
};

// ```
// head -c 1k /dev/zero | md5sum
// head -c $((50 * 1024 * 1024 - 17)) /dev/zero | md5sum
// head -c 2G /dev/zero | md5sum
// ```

const MD5s = {
  zero1kB: '0f343b0931126a20f133d67c2b018a3b',
  zeroSub50MiB: '894655788ad4055cf9a97b4bf0e9ee59',
  zero2GiB: 'a981130cf2b7e09f4686dc273cf7187e',
};


// All tests work in Safari and Firefox.
//
// The 2 GiB tests are disabled in Chrome, because the Blob(Array) constructor
// does not work for unknown reasons.
//
// Some larger tests are disabled in PhantomJS, because it crashes.

const isChrome = !!window.chrome;
const isPhantomJs = !!window._phantom;

describe('nog-blob', function () {
  describe('client-side sha1 computation', function () {
    it('1 kiB', function (done) {
      const zero1k = new Int8Array(1024);
      const blob = new Blob([zero1k]);
      const hasher = new Hasher(blob);
      hasher.onsuccess = (sha) => {
        expect(sha).to.eql(SHA1s.zero1kB);
        done();
      };
      hasher.start();
    });

    it('2 GiB; skipped in Chrome and PhantomJS', function (done) {
      if (isChrome || isPhantomJs) {
        this.skip();
        return;
      }
      this.timeout(60 * 1000);
      const zero128M = new Int8Array(128 * 1024 * 1024);
      const zero2G = (new Array(2 * 8)).fill(zero128M);
      const blob = new Blob(zero2G);
      const hasher = new Hasher(blob);
      hasher.onsuccess = (sha) => {
        expect(sha).to.eql(SHA1s.zero2GiB);
        done();
      };
      hasher.start();
    });
  });

  describe('client-side md5 computation', function () {
    it('1 kiB', function (done) {
      const zero1k = new Int8Array(1024);
      const blob = new Blob([zero1k]);
      const hasher = createMd5Hasher(blob);
      hasher.onsuccess = (sha) => {
        expect(sha).to.eql(MD5s.zero1kB);
        done();
      };
      hasher.start();
    });

    it('sub 50 MiB; skipped in PhantomJS', function (done) {
      if (isPhantomJs) {
        this.skip();
        return;
      }
      this.timeout(10 * 1000);
      const zeros = new Int8Array(50 * 1024 * 1024 - 17);
      const blob = new Blob([zeros]);
      const hasher = createMd5Hasher(blob);
      hasher.onsuccess = (sha) => {
        expect(sha).to.eql(MD5s.zeroSub50MiB);
        done();
      };
      hasher.start();
    });

    it('2 GiB; skipped in Chrome and PhantomJS', function (done) {
      if (isChrome || isPhantomJs) {
        this.skip();
        return;
      }
      this.timeout(60 * 1000);
      const zero128M = new Int8Array(128 * 1024 * 1024);
      const zero2G = (new Array(2 * 8)).fill(zero128M);
      const blob = new Blob(zero2G);
      const hasher = createMd5Hasher(blob);
      hasher.onsuccess = (sha) => {
        expect(sha).to.eql(MD5s.zero2GiB);
        done();
      };
      hasher.start();
    });
  });
});
