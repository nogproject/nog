// `nog-multi-bucket` uses npm aws-sdk directly.  Copy implementation from
// `nog-s3` and adapt it here as necessary.  Eventually `nog-multi-bucket`
// should completely replace `nog-s3`, and `nog-s3` should be deleted.
//
// `aws-sdk` is expected to be installed as a peer dependency in the app.  See
// `package-peer-versions.js` for the version check.
//
// The AWS API is used through explicit events `success` and `error` to avoid
// the callback API, since callbacks have been observed (with `aws-sdk@2.2.42`
// in `nog-s3`) to be spuriously called, causing false results and duplicate
// resolves of futures.


// The following workaround is no longer necessary with `aws-sdk@2.5.4`.
// It was necessary with `aws-sdk@2.2.42` in `nog-s3`.
//
// `process.browser` is deleted before importing `aws-sdk` as a workaround to
// avoid `Cannot read property of 'Stream'` during package testing with Meteor
// 1.3, see
// <https://forums.meteor.com/t/error-in-test-mode-on-1-3-typeerror-cannot-read-property-stream-of-undefined/21696/7>.
// The workaround that resets `process.browser` as described at the link does
// not work reliably.  Instead, we simply delete `process.browser` and keep it
// deleted, since this is server-only code.

// delete process.browser;


import AWS from 'aws-sdk';

import Future from 'fibers/future';
import { Meteor } from 'meteor/meteor';
import { check, Match } from 'meteor/check';
import { _ } from 'meteor/underscore';

import { NogError } from 'meteor/nog-error';
const {
  ERR_LOGIC,
  ERR_S3_ABORT_MULTIPART,
  ERR_S3_COMPLETE_MULTIPART,
  ERR_S3_CREATE_MULTIPART,
  createError,
  nogthrow,
} = NogError;

import {
  matchMultiBucketSettings, matchInterval,
} from './multi-bucket-settings.js';


console.log(`[nog-multi-bucket] Using aws-sdk@${AWS.VERSION}.`);


// `urlsExpireAfterSeconds` controls how long signed S3 URLs are cached in a
// TTL collection, so that the URLs remain stable and browsers can cache
// images.  A consistent expire time must be used for the TTL, the signed URL
// (valid a bit longer), and the cache control.
//
// A good explanation of cache control is 'Google Developers / HTTP Caching /
// Cache-Control' at
// <https://developers.google.com/web/fundamentals/performance/optimizing-content-efficiency/http-caching?hl=en#cache-control>

// `reportHealthEveryN` controls how frequently health check status is printed
// to the log.

const config = {
  urlsExpireAfterSeconds: 5 * 24 * 60 * 60,
  reportHealthEveryN: 100,
};


// `guessMime(filename)` returns the image mimetype for known filename
// extension.  The default for unknown extensions is `image/png`.

function guessMime(filename) {
  const extMap = {
    gif: 'image/gif',
    jpeg: 'image/jpeg',
    jpg: 'image/jpeg',
    png: 'image/png',
    tif: 'image/tiff',
    tiff: 'image/tiff',
  };
  const ext = _.last(filename.split('.')).toLowerCase();
  return extMap[ext] || 'image/png';
}


function selectBucket({ blob, readPrefs, fallback, checks }) {
  if (!blob.locs) {
    return fallback;
  }
  const available = {};
  blob.locs.forEach((loc) => {
    available[loc.bucket] = (loc.status === 'online');
  });
  for (const bkt of readPrefs) {
    if (available[bkt] && checks[bkt].isHealthy()) {
      return bkt;
    }
  }
  console.log(
    `[nog-multi-bucket] No healthy bucket for blob ${blob._id}; ` +
    `using fallback bucket '${fallback}'.`
  );
  return fallback;
}


function selectUploadBucket({ writePrefs, fallback, checks }) {
  for (const bkt of writePrefs) {
    if (checks[bkt].isHealthy()) {
      return bkt;
    }
  }
  console.log(
    `[nog-multi-bucket] No healthy upload bucket ` +
    `using fallback bucket '${fallback}'.`
  );
  return fallback;
}


function createBucketRouterWithDeps({
  readPrefs, writePrefs, fallback, s3s, checks, urlCache,
}) {
  return {
    readPrefs,
    writePrefs,
    fallback,
    s3s,
    checks,
    urlCache,

    startChecks() {
      for (const chk of _.values(this.checks)) {
        if (chk.start) {
          chk.start();
        }
      }
    },

    stopChecks() {
      for (const chk of _.values(this.checks)) {
        if (chk.stop) {
          chk.stop();
        }
      }
    },

    // See also `nog-blob-server.coffee` and `nog-tree.coffee`.
    //
    // Content disposition suggests to the web browser that the file should be
    // saved with the specified filename.
    //
    // Cache control allows the browser to reuse content, but only if the URL
    // is identical.  URLs are, therefore, cached on the server.  See comment
    // at `urlsExpireAfterSeconds` for details.

    getDownloadUrl({ blob, filename }) {
      const bucket = selectBucket({
        blob,
        readPrefs: this.readPrefs,
        fallback: this.fallback,
        checks: this.checks,
      });

      return this.s3s[bucket].getSignedUrl(
        'getObject',
        {
          Bucket: bucket,
          Key: blob._id,
          ResponseContentDisposition: `attachment; filename="${filename}"`,
        },
      );
    },

    getImgSrc({ blob, filename }) {
      const bucket = selectBucket({
        blob,
        readPrefs: this.readPrefs,
        fallback: this.fallback,
        checks: this.checks,
      });

      const mime = guessMime(filename);

      const cacheKey = `${bucket}/${blob._id};${mime}`;
      const cached = this.urlCache.findOne(cacheKey);
      if (cached) {
        return cached.url;
      }

      const expires = config.urlsExpireAfterSeconds;
      const url = this.s3s[bucket].getSignedUrl(
        'getObject',
        {
          Bucket: bucket,
          Key: blob._id,
          Expires: expires + 15 * 60,  // A bit longer than TTL.
          ResponseContentType: mime,
          ResponseCacheControl: `private, max-age=${expires}`,
        },
      );

      this.urlCache.upsert(
        cacheKey,
        {
          $set: { url },
          $currentDate: { ts: true },
        },
      );

      return url;
    },

    createMultipartUpload({ key }) {
      const bucket = selectUploadBucket({
        writePrefs: this.writePrefs,
        fallback: this.fallback,
        checks: this.checks,
      });

      const fut = new Future;
      const req = this.s3s[bucket].createMultipartUpload({
        Bucket: bucket, Key: key,
      });
      req.on('success', (res) => fut.return(res.data));
      req.on('error', (err) => {
        fut.throw(createError(
          ERR_S3_CREATE_MULTIPART,
          { cause: err, s3Bucket: bucket, s3ObjectKey: key }
        ));
      });
      req.send();
      return fut.wait();
    },

    getSignedUploadPartUrl(opts) {
      const bucket = opts.Bucket;
      return this.s3s[bucket].getSignedUrl('uploadPart', opts);
    },

    completeMultipartUpload(opts) {
      const bucket = opts.Bucket;
      const fut = new Future;
      const req = this.s3s[bucket].completeMultipartUpload(opts);
      req.on('success', (res) => fut.return(res.data));
      req.on('error', (err) => {
        fut.throw(createError(
          ERR_S3_COMPLETE_MULTIPART,
          {
            cause: err,
            s3Bucket: opts.Bucket,
            s3ObjectKey: opts.Key,
            s3UploadId: opts.UploadId,
          }
        ));
      });
      req.send();
      return fut.wait();
    },

    abortMultipartUpload(opts) {
      const bucket = opts.Bucket;
      const fut = new Future;
      const req = this.s3s[bucket].abortMultipartUpload(opts);
      req.on('success', (res) => fut.return(res.data));
      req.on('error', (err) => {
        fut.throw(createError(
          ERR_S3_ABORT_MULTIPART,
          {
            cause: err,
            s3Bucket: opts.Bucket,
            s3ObjectKey: opts.Key,
            s3UploadId: opts.UploadId,
          }
        ));
      });
      req.send();
      return fut.wait();
    },
  };
}

// See `nog-s3.coffee` for further, potentially useful implementation details.

function createS3(cfg) {
  const aws = {
    accessKeyId: cfg.accessKeyId,
    secretAccessKey: cfg.secretAccessKey,
  };
  const region = cfg.region;
  if (region) {
    aws.region = region;
    aws.signatureVersion = 'v4';
  } else {
    aws.endpoint = cfg.endpoint;
    aws.s3ForcePathStyle = true;
    aws.signatureVersion = cfg.signatureVersion || 'v2';
  }

  return new AWS.S3(aws);
}


function createFixedCheck(status) {
  return {
    isHealthy() {
      return status;
    },
  };
}


function createToggleCheck() {
  return {
    status: true,
    isHealthy() {
      return this.status;
    },
  };
}


function createRandomCheck({ cfg }) {
  const { checkFailureProb = 0.1 } = cfg;
  const bucket = cfg.name;
  return {
    bucket,
    failureProb: checkFailureProb,
    isHealthy() {
      if (this.failureProb === 1) {
        return false;
      }
      if (Math.random() >= this.failureProb) {
        return true;
      }
      console.log(
        '[nog-multi-bucket] Simulating random health failure on ' +
        `bucket '${this.bucket}'.`
      );
      return false;
    },
  };
}


function createGetObjectCheck({ cfg, s3 }) {
  check(cfg, Match.ObjectIncluding({
    name: String,
    checkKey: String,
    checkContent: String,
    checkInterval: matchInterval,
  }));
  const bucket = cfg.name;
  const key = cfg.checkKey;
  const content = cfg.checkContent;
  const intervalS = Number(cfg.checkInterval.replace('s', ''));
  const S_INIT = -1;
  const S_UNHEALTHY = 0;
  const S_HEALTHY = 1;

  const healthCheck = {
    bucket, key, content, intervalS, s3,
    successCount: 0,
    failureCount: 0,
    status: S_INIT,
    active: false,

    isHealthy() {
      return this.status === S_HEALTHY;
    },

    start() {
      this.active = true;
      this._tick();
    },

    stop() {
      this.active = false;
      clearTimeout(this._timeout);
    },

    _tick() {
      this.s3.getObject({ Bucket: this.bucket, Key: this.key }, (err, res) => {
        if (err) {
          if (this.status !== S_UNHEALTHY) {
            console.error(
              '[nog-multi-bucket] Health check switched to unhealthy: ' +
              `GET '${this.bucket}/${this.key}' returned error: ` +
              `${err.message}`
            );
          }
          this.failureCount += 1;
          this.status = S_UNHEALTHY;
        } else if (res.Body.toString('utf8') === this.content) {
          if (this.status !== S_HEALTHY) {
            console.log(
              '[nog-multi-bucket] ' +
              `Health check bucket '${this.bucket}' switched to healthy.`
            );
          }
          this.successCount += 1;
          this.status = S_HEALTHY;
        } else {
          if (this.status !== S_UNHEALTHY) {
            console.error(
              '[nog-multi-bucket] Health check switched to unhealthy: ' +
              `GET '${this.bucket}/${this.key}' content mismatch.`
            );
          }
          this.failureCount += 1;
          this.status = S_UNHEALTHY;
        }

        // Report after first check and then every `reportHealthEveryN`.

        const total = this.successCount + this.failureCount;
        if (total % config.reportHealthEveryN === 1) {
          console.log(
            '[nog-multi-bucket] ' +
            `Health status for bucket '${this.bucket}': ` +
            `${this.isHealthy() ? 'healthy' : 'unhealthy'}, ` +
            `successCount=${this.successCount}, ` +
            `failureCount=${this.failureCount}, ` +
            `reporting every ${config.reportHealthEveryN} checks.`
          );
        }

        if (this.active) {
          this._timeout = setTimeout(
            () => this._tick(),
            this.intervalS * 1000
          );
        }
      });
    },
  };

  return healthCheck;
}


function createCheck(cfg, s3) {
  if (cfg.check === 'toggle') {
    return createToggleCheck();
  } else if (cfg.check === 'random') {
    return createRandomCheck({ cfg });
  } else if (cfg.check === 'getObject') {
    return createGetObjectCheck({ cfg, s3 });
  } else if (cfg.check === 'healthy') {
    return createFixedCheck(true);
  } else if (cfg.check === 'unhealthy') {
    return createFixedCheck(false);
  } else if (cfg.check === undefined) {
    return createFixedCheck(true);
  }
  nogthrow(ERR_LOGIC);
  return undefined;
}


function createBucketRouterFromSettings({ settings, namespace }) {
  check(settings, matchMultiBucketSettings);

  function makeCollName(basename) {
    const ns = namespace.coll;
    if (ns) {
      return `${ns}.${basename}`;
    }
    return basename;
  }

  const urlCache = new Meteor.Collection(makeCollName('urlcache'));
  urlCache._ensureIndex(
    { ts: 1 },
    { expireAfterSeconds: config.urlsExpireAfterSeconds },
  );

  const s3s = {};
  for (const cfg of settings.buckets) {
    s3s[cfg.name] = createS3(cfg);
  }

  const checks = {};
  for (const cfg of settings.buckets) {
    checks[cfg.name] = createCheck(cfg, s3s[cfg.name]);
  }

  return createBucketRouterWithDeps({
    s3s, checks, urlCache,
    readPrefs: settings.readPrefs,
    writePrefs: settings.writePrefs,
    fallback: settings.fallback,
  });
}

export { createBucketRouterFromSettings };
