# nogpm - Changelog

nogpm-0.2.1, 2017-04-21

* Uses nogpy-0.0.18 with post buffer batch size and max size control.
* Fixed SHA1 mismatch during publish.
* Text is read as UTF-8.

nogpm-0.2.0, 2017-01-31

* Registry URLs may contain a host, like `nog.zib.de/nog/packages`.  Nogpm will
  verify that the host matches `NOG_API_URL`.
* Added alternate registries.
* `nogpm publish` checks that a version has not been published to an alternate
  registry.
* New sub-command `nogpm pull` to copy published versions from alternate
  registries.

nogpm-0.1.0, 2016-11-26

* The version check no longer refuses to publish if a prerelease has been
  published before.
* A `prerelease` that is null or an empty string will be handled as if no
  `prerelease` was present.

nogpm-0.0.19, 2016-11-14

* New field `nogpackage.package.content.objects` to create direct object
  children in published package tree.
* Semver pre-release versions are now specified in
  `nogpackage.package.version.prerelease`.  The old field `tag` is deprecated
  but still supported for backward compatibility.
* Experimental support for publishing protocols.
* nogpy-0.0.15 with errata support and increased S3 PUT 300s timeout.

nogpm-0.0.18, 2016-08-02:

* Polished README.

nogpm-0.0.17, 2016-06-15:

* Update dependencies to nogpy-0.0.13 to get Unicode fixes.

nogpm-0.0.16:

* Update dependencies to nogpy-0.0.11

nogpm-0.0.15:

* More robust HTTP connection handling with retries.

nogpm-0.0.14:

* Dependencies may have an optional field `programRegistry` denoting the
  program repository to install the package from. If not set, the package
  default location `nog/packages` is used.

nogpm-0.0.11:

* The package default location changed to `nog/packages`.  Previous versions
  are in `sprohaska/nogpackages`.

nogpm-0.0.9:

* nogpm now uses frozen dependencies to avoid potential incompatibilities with
  nogpy.

nogpm-0.0.8:

* Added a bit of documentation that should help getting started.

nogpm-0.0.7:

* Fixed handling of broken symlinks during `nogpm link`.

nogpm-0.0.6:

* Added support for freezing dependencies and attaching them during publish to
  create self-contained nog job programs without code duplication.

nogpm-0.0.5:

* Fixed publishing packages without program.
* New command `link` to manage symlinks to dependencies.
* New command `freeze` to store frozen dependencies.
* Add option `--local` to work with local packages.
* `install` can be used to install package dependencies.
* Changed default registry to `sprohaska/nogpackages`.
