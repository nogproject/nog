# Getting Started with Nog and SciPy

The SciPy libraries are commonly used for when performing data analysis in
Python.  This tutorial illustrates how to use SciPy with Nog.

<!-- toc -->

## How to use the Nog Python API and SciPy to analyze data?

The example program used in this tutorial reads image data from Nog, uses SciPy
to do simple image processing, and writes the result data back to Nog.

At the end of this tutorial, you should have an idea how Nog stores data in
repositories, and how you can use Python to process it.  For a description of
the data structure that Nog uses to store data and metadata, see the [data
model section in the API
reference](/nog/doc/files/apidoc.md#data-model).

### Setup for the photo-gallery-simple example

The example program assumes that a Nog development setup has been configured as
described in [Using the Python REST API
Wrapper](tutorial-nogpy.md#configure-the-environment-for-nogpy), and a Python
environment has been set up, preferably with `virtualenv`. In the directory
where you unpacked the `nog-starter-pack` you find the `photo-gallery-simple`
script.

To execute this example, additional Python modules are needed.  Either use
a platform-specific installation process (see separate subsections below) or
use pip:

```bash
pip install numpy
pip install scipy
pip install scikit-image
pip install requests
```

Ensure that you have a repository `<your/repository>` with some images in
`datalist`.  See [getting started with the web UI](tutorial-ui.md).

#### Linux

Use pip.  You may, however, need to install `liblapackdev` and `gfortran`
system-wide to be able to install `scipy`.

#### Mac OS X

To install the Python 3 SciPy modules with brew, use:

```bash
brew install openblas
brew install --with-python3 --with-openblas numpy
brew install --with-python3 --with-openblas scipy
pip3 install scikit-image
```

### Using SciPy to process data in a Nog repository

`photo-gallery-simple` is a program that reads data from a Nog repository,
processes them, and writes the results to Nog. In particular, it reads images
from a *datalist* collection in the specified repository, computes thumbnails
of configurable size, and writes the result back to a *results* collection.
Run it as follows.

Open `photo-gallery-simple` and adjust the name of repository to the one of
your repo:

```python
repo_name = '<your/repository>'
```

Run the script with:

```bash
python3 photo-gallery-simple
```

The result can be viewed in the `result` folder of `<your/repository>` in the
Nog UI.

### Annotated Python code for using SciPy on a Nog repository

```python
{{{code}}}
```
