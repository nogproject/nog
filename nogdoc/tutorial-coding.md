# Getting Started with Developing Nog Job Programs

Nog allows users to run analysis programs from the web UI.  This tutorial show
how such a program is created in Python.

<!-- toc -->

## How to write an analysis program that can be started from the Nog web UI?

Follow the instructions below to see how to write, test, and publish a
(Python) program with Nog.

The example program can be started from the web UI to run as a job in a queuing
system, or it can be started locally during development.

This tutorial introduces the Nog Package Manager `nogpm` as a tool for
packaging, publishing, and installing Nog programs (and their dependencies).
Below you will learn how to create a custom Nog program and package it for use
in the Nog webapp.

### Nog development environment setup

Configure your Nog development environment as described in [Using the Python
REST API Wrapper](/tutorial-nogpy.md#configure-the-environment-for-nogpy).  

Install the required Python libraries (`numpy`, `scipy`, `scikit-image`,
`requests`) as described in [getting started with the Python
API](./tutorial-python-api-basics.md).  We recommend to use virtualenv to
setup a working environment with Python3.

#### The Nog package manager

In addition, use the `noginstaller` to install the Nog Package Manager `nogpm`.
Please follow the instructions in the [noginstaller
documentation](/nog/packages/files/programs/noginstaller/index!0).

Finally, use `nogpm` to download and install the `photo-gallery` program:

```bash
mkdir nog-example
cd nog-example
nogpm install --registry nog/example_programs_2015 photo-gallery
cd nogpackages/photo-gallery
nogpm install --frozen
nogpm link
```

### Running a Nog job program

When you run a program as described in [getting started with the web
UI](./tutorial-ui.md), a job is created and sent to to the job server.  Such
a job requires that the program has been published to a program registry and
can handle the parameters that the job server passes on the command line.

To make the process of developing and testing the program easier, there are
two ways to test the program:

- Running the program locally
- Publishing the program and starting it as a Nog job

#### Running a program locally

Ensure that you have a repository `<your/repository>` with some images in
`datalist`.  See [getting started with the web UI](tutorial-ui.md).

Open `photo-gallery` in a text editor and adjust
`paramsdefault.nog.workspaceRepo` to point to `<your/repository>`:

```python
paramsdefault = {
    "maxWidth": 200,
    "maxHeight": 200,
    "nog": {
        "program": {
            "name": "photo-gallery"
        },
        "workspaceRepo": "nog/example_photo-gallery_2015"
    }
}
```

Run the script:

```bash
python3 photo-gallery
```

The result can be viewed in the `results` folder of `<your/repository>` in the
Nog UI.

#### Running a program as a Nog job

Running the program as a Nog job requires the program to be published to a
program repository. Use the Nog Package Manager to take care of this. Each
program must be accompanied by a file `nogpackage.json`, which provides
information about program name, version, dependencies and the target repository
to `nogpm`.

In order to run the `photo-gallery` program as a Nog job, do the following:

Create a new repository of type *Program Registry*: `<your/program-repository>`.

Set the package `programRegistry` entry in `nogpackage.json` to your new
program repository.

If you have changed anything in the program, edit the package version in
`nogpackage.json` considering *semantic versioning*.

Publish the program:

```bash
 nogpm publish
```

Run the program as described in [getting started with the web UI](./tutorial-ui.md).


#### Running the program with multiple parameter sets

The program `photo-gallery` can be executed for multiple parameter sets in
both cases, locally and as Nog job.

To run locally with multiple parameter sets do the following:
Extend `paramsdefault` by a list of dictionaries containing the varying
parameters (example below). It expects a key `variants` and creates
subfolders in the results section of `<your/program-repository>` for each
parameter set. Your can set the names of the subfolders in each parameter set
 with the key `name` or they are automatically generated from the parameters.

```python
paramsdefault = {
    "maxWidth": 200,
    "maxHeight": 200,
    "variants": [
        { "name": "default" },
        {},
        { "name": "large", "maxHeight": 299 },
        { "name": "small", "maxHeight": 100 },
        { "maxWidth": 150}
        ],
    "nog": {
        "program": {
            "name": "photo-gallery"
        },
        "workspaceRepo": "nog/example_photo-gallery_2015"
    }
}

```

To run the program as Nog job with multiple parameter sets, your can adjust
the parameters using the web UI as described in [getting started with the web
UI](./tutorial-ui.md).

### Annotated Python code for a Nog job program

The example illustrates a possibility to get and set input data from Nog and
upload results.

This tutorial uses an experimental convenience library `nogjob.py`, which
provides an abstraction in the form of a `NogJob` object that allows to run the
program both locally and through the Nog webapp with the same code.

```python
{{{code}}}
```
