# Nog Starter Pack

<!-- toc -->

## Introduction

The nog starter pack contains files that may initially be useful when getting
started with nog.  The [main documentation](/nog/doc/files) provides more
details.

Some of the files are duplicated from elsewhere (like `nog.py`), so that the
starter pack is reasonably self-contained in order to get a first impression
quickly.  A more advanced setup with nogpm should be used for real development
work.

## Python

`nog.py` is a Python package that wraps the REST API.  It is a copy from the
nog package [nogpy](/nog/packages/files/programs/nogpy/0).  The copy is
included here for convenience.  See the full package for details.

## Shell

`bin/sign-req` is a bash script to sign URL for curl.  See `sign-req -h` for
more.

## Experimental Command Line

`bin/nog` is a command line tool that can be used to manage a local working
copy of a nog repo as a git history (see `nog --help`).

The following scripts can be used to export a stdrepo to JSON and then import
the JSON to nog.  Prefer the nog command.  The scripts are included primarily
to illustrate how to write programs that use nog:

```
bin/stdtools-repo-to-nog-tree-json
bin/nog-post-tree-json
```

## Changelog

nog-starter-pack-0.0.5, 2016-06-15:

* Updated bundled nogpy-0.0.13 to fix checksum errors related to latest content
  idversion.

nog-starter-pack-0.0.4:

* Added prefetching of blobs.

nog-starter-pack-0.0.3:

* Added tutorial program `photo-gallery-simple`.

nog-starter-pack-0.0.2:

* Added tutorial program `file-listing`.
