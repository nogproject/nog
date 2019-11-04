# Welcome to Nog

Welcome to Nog, an application for collaborative data science.

<!-- toc -->

## DISCLAIMER

Nog is preview software.  We will not deliberately delete your data.  But we
run without backup.  So please keep a copy of your data elsewhere.

## Introduction

Nog stores content in repositories.  A repository contains a tree of nested
ordered lists of objects.  Entries can have metadata, which can be used to
search.  Objects can have binary opaque blobs attached, whose content is
ignored during search.

Only the owner can modify a repository.  You can organize other users in
circles (see [settings](/settings)) and share repositories (see top of
repository browsing view) with all or some of your circles, or publicly with
all confirmed users.

Some meta fields are special: `meta.description` is displayed in listings.
`meta.content` is displayed when viewing an object; it is rendered as markdown
if reasonable.  We will handle more meta fields in a special way as we add
support for specific use cases.

Nog is in an early development stage.  We'd appreciate feedback and suggestions
how to improve Nog to make it useful to you.

## Howtos

 - [howto-upload](./howto-upload.md): Describes how to upload and share files.
 - [howto-organize-data](./howto-organize-data.md): Contains suggestions how to
   organize project data.

## Starter Pack

The [nog starter pack](/nog/packages/files/programs/nog-starter-pack/index!0)
contains a few files that may be used in the tutorials or that may otherwise be
useful when getting started with nog.  The starter pack is available for
download at:

 - [nog-starter-pack/index!0/content.tar.xz](/nog/packages/files/programs/nog-starter-pack/index!0/content.tar.xz)

## User Tutorials

- [tutorial-ui](./tutorial-ui.md): learn how to create a repository, add
  data to it, apply an existing analysis program and view the results.

## Developer Tutorials

- [tutorial-rest](./tutorial-rest.md): Learn how to explore the REST API from the
  command line.
- [tutorial-nogpy](./tutorial-nogpy.md): Learn how to use the Python REST API
  wrapper.
- [tutorial-nogpy-workspace](./tutorial-nogpy-workspace.md): Learn how to use the
  Python API to manipulate a workspace.
- [tutorial-python-api-basics](./tutorial-python-api-basics.md): learn how nog
  stores its data in repositories, and how to edit a nog repository using the
  Python API.
- [tutorial-coding](./tutorial-coding.md): learn how to create an analysis
  program that can be applied to data stored in nog, how to test it locally,
  and how to publish it to nog for use in the nog webapp.

## Developer Reference

- [apidoc](./apidoc.md): reference of the nog REST API.

We switched to API v1 in early 2016.  See
[api-transition-v0-v1](./api-transition-v0-v1.md) for details.

## Reference for Developers of Nog Itself

- [devdoc](./devdoc.md): reference of the Meteor packages that nog uses.

## The Nog Package Manager nogpm

nogpm is used to manage code for nog compute jobs (currently only Python
programs).  nogpm itself is maintained with nogpm in the nogpackage
[nogpm](/nog/packages/files/programs/nogpm/index!0).

Getting Started with nogpm:

 - Create and download an API key (see [settings](/settings)).
 - Configure your shell environment: export the API key by sourcing the
   downloaded file (or copy paste the export commands), `export
   NOG_USERNAME=<username>`, `export
   NOG_API_URL=https://<this-host>/api`.
 - Create a cache directory and set `NOG_CACHE_PATH` to it.
 - Follow the instructions at
   [noginstaller](/nog/packages/files/programs/noginstaller/index!0).

## REST API

The REST API is described in the separate document [apidoc](./apidoc.md).

### Python API

`nog.py` is a Python package that wraps the REST API to provide a higher-level,
more pythonic API.  It is maintained in the nog package
[nogpy](/nog/packages/files/programs/nogpy/index!0).

`nogjob.py` may be useful when developing analysis programs: It is maintained
in the nog package [nogjobpy](/nog/packages/files/programs/nogjobpy/index!0).

## Nog Markdown

Pandoc title blocks with the following format are handled:

```
% Title
% byline (optional)
```

<!-- BANG is used below to hide the special comment from the markdown parser,
which would otherwise replaced it with a toc . -->

The special HTML comment `< BANG -- toc -->` (use the character `!` instead of
` BANG `) is replaced by a table of contents.

Backslashes at the end of a line are mapped to `<br>`.

The stdtools token ` AT AT VERSIONINC AT AT ` is removed (` AT ` stands for the
character `@`).

## Command Line

### Nog Command

The `nog` command (included in the starter pack) can be used to manage a local
working copy of a nog repo as a git history (see `nog --help`).

Getting started:

 - Download the starte pack, which contains `bin/nog`.
 - Check that `nog` runs with Python 3.  Install Python packages if necessary.
 - Create and download an API key (see [settings](/settings)).
 - Configure your shell environment: export the API key by sourcing the
   downloaded file (or copy paste the export commands), `export
   NOG_USERNAME=<username>`, `export NOG_API_URL=https://<this-host>/api`.
 - Continue with instructions from `nog --help` how to create a repo.

### Experimental Scripts

Prefer `nog` over these experimental scripts.  The following two experimental
scripts, which are both contained in the started pack, can be used to export
a stdrepo to JSON and then import the JSON to nog:

```
bin/stdtools-repo-to-nog-tree-json
bin/nog-post-tree-json
```

## Known Issues

### SSL certificate validation errors with Python 2.7 package 'requests'

The package 'requests' for Python 2.7 does not support SNI, which may cause SSL
certificate validation errors.  SNI support can be added by installing the
following packages:

 - pyOpenSSL (only version 0.13)
 - ndg-httpsclient
 - pyasn1
 - urllib3

See <https://github.com/kennethreitz/requests/issues/749#issuecomment-19187417>
for more information.
