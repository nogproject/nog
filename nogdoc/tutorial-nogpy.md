# Using the Python REST API Wrapper

`nog.py` is a Python package that wraps the REST API to provide a higher-level,
more pythonic API.  It is maintained in the Nog package
[nogpy](/nog/packages/files/programs/nogpy/index!0).  This tutorial shows how to
use `nog.py` to access Nog content.  Nog packages and the Nog package manager
will be explained in a separate tutorial.

<!-- toc -->

## The Nog starter pack

The [Nog starter pack](/nog/packages/files/programs/nog-starter-pack/index!0)
contains files that are used in the tutorials.  Get the starter pack and unpack
it unless you already have it.  The starter pack archive is available at:

- [nog-starter-pack/index!0/content.tar.xz](/nog/packages/files/programs/nog-starter-pack/index!0/content.tar.xz)

## Configuring the environment for nog.py

`nog.py` caches data on the local file system to reduce the number of HTTP
requests.  Create a cache directory:

```bash
mkdir <path/to/>nogcache
```

You need to configure a few environment variables to use `nog.py`.  Create an
API key at [settings](/settings), save the export statements in a file, and
source it in a shell before using `nog.py`.  `nog.py` requires the following
environment variables:

```bash
export NOG_API_URL=https://nog.zib.de/api
export NOG_CACHE_PATH='<path/to/>nogcache'
export NOG_USERNAME=<username>
export NOG_KEYID=<keyid>
export NOG_SECRETKEY=<secretkey>
```

`nog.py` uses the Python package `requests`.  You may need to install it with:

```bash
pip install requests
```

Consider using a virtualenv if you do not have admin rights to install requests
on your system. See [Virtual
Environments](http://docs.python-guide.org/en/latest/dev/virtualenvs/) for an
introduction to virtualenv.  The following commands should create a virtualenv
for Python 3, activate it, and install `requests`:

```bash
cd /your/working/directory/
virtualenv -p python3 nogvenv  # You may choose another name.
source nogvenv/bin/activate
pip install requests
```

## Exploring Nog content from Python

An interactive Python session with `nog.py` can be used to explore the content
of a repository.  The API is described in details in the README of the package
[nogpy](/nog/packages/files/programs/nogpy/index!0).

The following example explores the content of the documentation repository and
prints the markdown text of the documentation index.  `nog.py` must be in the
current directory, so that `import nog` will find it.

```
$ python3
>>> import nog
>>> repo = nog.openRepo('nog/doc')
>>> master = repo.getMaster()
>>> root = master.tree
>>> [r.name for r in root.entries()]
['index.md', 'tutorial-ui.md', 'tutorial-rest.md', 'tutorial-python-api-basics.md', 'tutorial-coding.md', 'apidoc.md', 'devdoc.md']
>>> idx = next(root.entries())
>>> print(idx.meta['content'])
```

You can inspect the cache directory.  `nog.py` should have placed some files
there by now, which will be used to avoid future HTTP requests.

`nog.py` provides an API to get data from nog and to modify content, including
the upload of binary files.  See the package documentation of
[nogpy](/nog/packages/files/programs/nogpy/index!0) for details.
