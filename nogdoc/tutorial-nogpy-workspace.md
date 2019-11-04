# Manipulating a Workspace Repository from Python

Input data and results can be organized in a workspace as explained in [getting
started with the Nog web UI](tutorial-ui.md).  This tutorial shows how to write
a Python program that adds results to a workspace.

<!-- toc -->

## Setup

Get the starter pack and configure the environment for nog.py as described in
[using the Python REST API Wrapper](tutorial-nogpy.md).

## Creating a workspace

Create a workspace and add files to the datalist as described in [getting
started with the web UI](tutorial-ui.md).

## Running a Python program locally

Running a Python program locally is a straightforward way to manipulate a Nog
workspace.

The started pack contains the program `file-listing`.  Run it on your workspace
to create a file listing:

```
./file-listing <your/repository>
```

The program should have added a new object `results/file-listing/index.md` with
a listing of the objects from the datalist.

## Annotated Python code for manipulating a workspace

The program `file-listing` (see program code below or in the started pack)
illustrates the key steps to manipulate a workspace.  It is written such that
you can read the source code from top to bottom.

The main steps are:

 - Open the repository and get the root tree.
 - Get an existing `result` tree or create a new one.
 - Create the output tree below `result`.
 - Get the input `datalist`.
 - Prefetch blob content from S3 to hide latency.
 - Iterate over the `datalist` and populate the output tree.
 - Commit the updated root tree.

You will see the same general pattern in programs that do real work, like image
processing.

```python
{{{code}}}
```
