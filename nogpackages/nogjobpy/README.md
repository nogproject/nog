# nogjob.py

The package nogjob.py helps writing nog programs that can either be executed
locally for testing or as nog batch jobs.

Use nogpm to install, and import in Python with:

    import nogjob

The recommended way to get started is to read through one of the usage
examples and the related tutorial text.

The package is in alpha stage.  We maintain a changelog (see below) but do not
yet provide semver API stability.

## Usage Examples

 - `photo-gallery`: A variant of photo gallery that uses nogjob.py, which wraps
   some of the low-level API into higher-level convenient functions.

## Overview

nogjob.py assumes that a program will be executed in a workspace that contains
the trees `datalist` and `programs`.  The output tree will be named after the
program and stored to `results`.

When executed as a nog batch job, the execution environment will pass the
program parameters as `--params <JSON>`.  When executing locally for testing,
you need to provide reasonable fake parameters (see photo-gallery example).

The class `NogJob` is instantiated with the params.  It will maintain the
helper state. `NogJob.prepareComputation()` creates a temporary working dir
(accessible via property `NogJob.tmpdir`), prefetches the input data, and
returns a list of input data objects of class `nog.Object`.  You may report
progress via `NogJob.progress(completed, total)`.  Complete the computation
with `NogJob.commitResult()`.

## Changelog

nogjob-0.0.11:

* Nogjob evaluates the program version only in slurm mode.

nognogjob-0.0.10:

* Nogjob evaluates the version of program that is committed, and sets it to
  meta field 'programVersion' of the result tree.

nognogjob-0.0.9:

* Changed the name of the metadata field with the job id that generated
  a result to `jobResult` (instead of `job`), so it has a unique and more
  suitable kind identifier.

nognogjob-0.0.8:

* Nogjob validates the name of the result folder that is committed, and sets it
  to the program name if the folder name is not set.
* Nogjob adds the job id to the metadata of the result folder that is
  committed, so a result can be associated with the job that created it in the
  nog GUI.

nognogjob-0.0.7:

* Creation and use of temporary directory is not managed by nogjob anymore, but
  should be handled by the client program.

nognogjob-0.0.6:

* The package default location changed to `nog/packages`.  Previous versions
  are in `sprohaska/nogpackages`.

nognogjob-0.0.2:

* Extracted `nogjob.py` into a separate nog package.

nognogjob-0.0.1:

* photo-gallery example works.  The API seems reasonably complete to do real
  work.
