# Nogpm

## Changelog

See [CHANGELOG](./CHANGELOG.md).

## Introduction

The purpose of the nog package manager 'nogpm' is to help maintaining programs
that can be executed as nog jobs.  The goal is to provide as little
functionality as possible.  nogpm provides basic dependency handling.  We do
not intend to add a full-scale semantic-version-constraints solver.  If the
number of packages becomes larger, or more powerful dependency management is
required for other reasons, platform-specific solution should be considered
(like pip for Python or npm for node).

A package is declared in `nogpackage.json`.  The most important fields are:

 - `package.content.files`: A list of files to be published as content.
 - `package.content.objects`: A list of objects to be published as children of
   the package tree.  Example object: `{ "text": "CHANGELOG.md" }`.
 - `program.code.files`: A list of files to be published as code (see below);
   the files are also published as content as if they were listed in
   `package.content.files`.
 - `package.dependencies`: A list of dependencies.  The concept is similar to
   npm dependencies.  But nogpm does not manage version constraints.
 - `package.frozen`: A list of dependencies that are frozen to a specific
   version.  The general idea is similar to npm's shrinkwrap, although the file
   format is considerably different.

nogpm maintains a file layout as illustrated below. Dependencies are installed
by unpacking their content into a subfolder `nogpackages/<name>`.  Files that
are listed in `program.code.files` are symlinked to the toplevel folder.  The
symlinks mimic how code archives are unpacked on top of each other into
a single folder during nog job execution.  Example:

    photo-gallery/nogpackage.json
    photo-gallery/nogpackages/nogpy/nogpackage.json
    photo-gallery/nogpackages/nogpy/nog.py
    photo-gallery/nog.py -> nogpackages/nogpy/nog.py
    photo-gallery/photo-gallery  # Can use `import nog`

## Getting Started

Install a package from a specific repository:

    nogpm install --registry nog/example_programs_2015 photo-gallery
    cd nogpackages/photo-gallery

List how dependencies are resolved:

    nogpm resolve --list

Create symlinks to dependencies (the files that are declared as code):

    nogpm link

Install frozen dependencies into a local subfolder and change symlinks to the
subfolder:

    nogpm install --frozen
    nogpm link

## How to maintain a nog job program?

The program should be in version control (usually git).  You should have
a clean working copy before running nogpm.  `nogpm link`, for example, will
modify existing files without warning.

Create a `nogpackage.json` with the program dependencies (usually `nogpy`).
Consider copying from the photo-gallery example to get started.

Dependencies can use `programRegistry` to explicitly specify a package origin.
The default is `nog/packages`.

Install the dependencies locally and create symlinks:

    nogpm install --local
    nogpm link

There is no general rule how to manage the files created by nogpm in version
control.  You may commit them to have full control over the changes, or you may
configure git to ignore them and only rely on nogpm.  A good compromise is
probably to ignore `nogpackages/` but track symlinks in git.

When the program is ready, freeze the dependencies:

    nogpm freeze
    git commit -- nogpackage.json

Increase the version and update the changelog.

Then publish:

    nogpm publish

To update dependencies, manually delete their files in `nogpackages/` and
reinstall, since nogpm does not yet have a sub-command `update`.  Freeze and
publish again.

## How to maintain a library?

Proceed as described for a program, but without freeze.

## Known Issues

 - Updating packages has not been implemented.  As a workaround, remove the
   package directory and use `nogpm install` to get the latest version.

 - The semantic is undefined for packages that have frozen packages attached
   recursively.  You should use freeze only for toplevel programs but not for
   libraries.

 - `nogpm link` does not cleanup stale links if the code of dependencies
   changes.  You have to manually clean up.

 - The format of `nogpackage.json` is undocumented and may change.  Check
   examples and the implementation of `nogpm` and `nogexecd`.

 - The implementation is rather brittle.  Preconditions are usually not checked
   and violations may result in Python exceptions that are hard to understand.
   We hope that this will improve as we add more reasonable error handling for
   the usual problems.  Patches are welcome.
