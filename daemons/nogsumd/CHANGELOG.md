# nogsumd - Changelog
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## nogsumd-0.3.1, 2018-07-18

* Log config `signatureVersion` cleartext.

## nogsumd-0.3.0, 2018-07-16

* Add config `signatureVersion` to choose AWS signature algorithm `v2` or `v4`.

## nogsumd-0.2.0, 2017-07-04

* Fixed loading config from file.
* Secrets are hidden from logs.
* Vault is disabled if not used in the config.
* MongoDB SSL config options.
* Fixed Ceph S3 with botocore >= 1.5.71.  The S3 signature version is now
  explicitly set to V2.  botocore-1.5.71 changed the default to V4; see
  <https://github.com/boto/botocore/blob/develop/CHANGELOG.rst#1571>

## nogsumd-0.1.4, 2016-11-14

* Fixed background Vault token replacement.

## nogsumd-0.1.3, 2016-10-10

* Initial Prometheus metrics.

## nogsumd-0.1.2, 2016-09-15

* Fixed credentials renewal during long-running operations.

## nogsumd-0.1.1, 2016-09-14

* Duplicate checksum computation is avoided by rechecking blob state.

## nogsumd-0.1.0, 2016-09-14

* AWS S3 ETag verification based on auto-detected multi-part upload parameters.
* SHA1 and SHA256 checksums.
* Basic status HTTP endpoint.
