# Photo Gallery

photo-gallery is a nog program that creates a thumbnail gallery from
a workspace datalist.  Its primary purpose is to illustrate how to implement
nog programs.

## Parameters

 - `maxWidth`: the maximum thumbnail width.
 - `maxHeight`: the maximum thumbnail height.

## Changelog

photo-gallery-0.0.40, 2017-07-12:

* Fix Numpy indexing issue that caused photo-gallery to crash with newer Numpy
  versions.

photo-gallery-0.0.39, 2016-06-15:

* Updated nogpy-0.0.13 to fix potential Unicode issues.

photo-gallery-0.0.38:

* Upgrade to nogpy-0.0.12 for Python 2.7 and 3 compatibility.

photo-gallery-0.0.37:

* Fix typo in program warning

photo-gallery-0.0.36:

* Upgrade to nogjobpy@0.0.11.

photo-gallery-0.0.35:

* Use version 0.0.10 of nogjob.py that adds the program version to the metadata
  of the result folder.

photo-gallery-0.0.33:

* add example link to result document

photo-gallery-0.0.32:

* fix handling of multiple files with same name
* fix composition of parameter variants
* warn if parameters are missing

photo-gallery-0.0.31:

* Switched to nogpy@0.0.11 to use API v1.

photo-gallery-0.0.30:

* Use version 0.0.9 of nogjob.py that adds the job id to the `jobResult` field
  of result folder metadata.

photo-gallery-0.0.29:

* Use version 0.0.8 of nogjob.py that automatically adds the job id to the
  result folder metadata.

photo-gallery-0.0.28:

* Remove out-of-range parameter example.

photo-gallery-0.0.27:

* Fix bug when executing program with single parameter set.
* Result collection names do not contain colons.
* Progress information takes parameter variants into account.
* Parameter range check of all variants before computations start.

photo-gallery-0.0.26:

* Allow computation of multiple scans by specifying parameter variants.

photo-gallery-0.0.25:

* More robust HTTP connection handling with retries.

photo-gallery-0.0.24:

* A dict literal is used instead of JSON for the default params.

photo-gallery-0.0.23:

* The file `photo-gallery_simple-nog` was moved to the nog-starter-pack
  package.

photo-gallery-0.0.22:

* The package default location changed to `nog/example_programs_2015` with deps
  from `nog/packages`.

photo-gallery-0.0.20:

* Switched to managing deps with nogpm.

photo-gallery-0.0.19:

* Dropped support for per-entry versions.  Apply 'Edit / Drop Versions' on
  versioned entries in the datalist before running the program.
