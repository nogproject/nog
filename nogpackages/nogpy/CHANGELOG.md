# nogpy - Changelog

nogpy-0.0.19, 2017-xx-xx (unreleased)

* `postObject()` now rejects idversion-1 objects that use `meta['content']`.

nogpy-0.0.18, 2017-04-21

* Improved post buffer size control: `nog.POST_BUFFER_SIZE` now control the
  buffer size for batching small requests.  `nog.POST_BUFFER_SIZE_LIMIT` allows
  exceptions for large objects.

nogpy-0.0.17, 2017-02-22

* The default for Object.idversion is now 1. Backward compatibility with
  version 0 is retained.

nogpy-0.0.16, 2017-01-16

* ETag verification during S3 upload.
* New `nog.POST_BUFFER_SIZE` controls the size of the upload buffer, which can
  be used to avoid HTTP timeouts.

nogpy-0.0.15, 2016-10-10:

* Initial support for errata.  Entry errata will raise an `ErrataError` unless
  configured otherwise via the environment variable `NOG_ERRATA`; see comment
  in `nog.py` source.

nogpy-0.0.14, 2016-09-12:

* Increased 300s timeout for S3 PUT to fix upload to Ceph S3.

nogpy-0.0.13:

* Fixed Unicode handling.

nogpy-0.0.12:

* Compatibility with Python 2.7 and 3.

nogpy-0.0.11:

* Switch to API v1.  nogpy now uses the new API and supports commit format
  1 and object format 1.  See apidoc for details.
* Fixed reporting blob prefetch errors.
* Fixed retry post for entries that are copied from other repo.
* Fixed sha1 caching to avoid outdated sha1s when modifying entries that were
  initialized from remote.
* Fixed caching of stat info to avoid false missing entry errors.
* New `Object.text` property.
* New `Object.idversion` and `Commmit.idversion` properties.
* New `Object.format()` to convert object between format 0 and 1.
* New `Tree.collapse()` to detach children.
* New `postObject()`.
* New properties for missing commit fields.
* Switched to cryptographically secure nonces.

nogpy-0.0.9:

* Changed the default to a short HTTP connection timeout with a default of
  5 retries.  The number of retries can be controlled by `NOG_MAX_RETRIES`.

nogpy-0.0.8:

* The package default location changed to `nog/packages`.  Previous versions
  are in `sprohaska/nogpackages`.

nogpy-0.0.6:

* Fix `commitTree()` variants to return new master commit.

nogpy-0.0.5:

* `nog.createRepo()` now creates an initial commit.
* The signature of `nog.commitTree()` changed to expect a single commit in
  `parent` instead of a list of commits in `parents`.

nogpy-0.0.2:

* Extracted nog.py into a separate nog package.

nogpy-0.0.1:

* photo-gallery example works.  The API seems reasonably complete to do real
  work.
