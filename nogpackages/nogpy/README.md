# nogpy

The package nogpy wraps the nog REST API to provide a convenient Python
interface, including caching of immutable content.

Use nogpm to install, and import in Python with:

    import nogpy

The recommended way to get started is to read through one of the usage
examples and the related tutorial text.

The package is in alpha stage.  We do not yet provide semver API stability.

See [CHANGELOG](./CHANGELOG.md) for the version history.

## Usage Examples

 - `photo-gallery_simple-nog`: Simple program that illustrates the direct use
   of nog.py.
 - `photo-gallery`: A variant of photo gallery that uses nogjob.py, which wraps
   some of the low-level API into higher-level convenient functions.

## Configuration

nog.py expects the following environment variables:

 - `NOG_API_URL`: The URL for the API; usually `https://nog.zib.de/api`.
 - `NOG_CACHE_PATH`: The local directory that is used for caching.
 - `NOG_KEYID` and `NOG_SECRETKEY`: Nog access key.

The following variables are optional:

 - `NOG_USERNAME`: The nog username, which is used to prefix short repo names
   if necessary.
 - `NOG_MAX_RETRIES` (default 5): The number of retries of the initial HTTP
   connect.  Connections that have already transferred data will not be
   retried.

The Python variables `nog.POST_BUFFER_SIZE` and `nog.POST_BUFFER_SIZE_LIMIT`
control the size of the upload buffer in bytes.  A larger buffer reduces the
number of HTTP POST request, which usually increases throughput.  If the buffer
large and contains many small objects, the server, however, may take too long
to process it, causing an HTTP timeout.  The buffer must be large enough to
hold the largest object or tree to be posted.  The two variables together allow
a flexible setup: small POST requests are batched up to a preferred size of
`POST_BUFFER_SIZE`.  `POST_BUFFER_SIZE_LIMIT` limits the maximum size of an
individual POST.

Default buffer settings:

 - `nog.POST_BUFFER_SIZE = 10000`
 - `nog.POST_BUFFER_SIZE_LIMIT = 200000`

## API Overview

The functions `openRepo(name)` or `createRepo(name)` are used to access a repo.
They both return an instance of class `RemoteRepo`.

The class `RemoteRepo` provides access to the repo content.  The repo content
is wrapped into Python objects of class `Commit`, `Tree`, and `Object`, which
provide convenient methods for accessing and modifying the content.

Many methods of `RemoteRepo` correspond to the low-level REST API routes, such
as `getObject()`, `postObject()`, `getTree()`, or `postTree()`.  They are,
however, rarely used directly.  Instead, `RemoteRepo.getMaster()` is usually
used to get an instance of class `Commit` for branch master.  The content is
then accessed through the property `Commit.tree` (see below).  A new commit
can later be created and committed with `RemoteRepo.commitTree()` (see details
at the method's documentation).  New blobs are uploaded in parallel and content
is grouped into bulk posts to minimize the number of API accesses in order to
achieve good performance.  `RemoteRepo.prefetchBlobs()` fetches several blobs
in parallel, which is important to achieve good performance in practice.  All
immutable content is cached in `NOG_CACHE_PATH` and preferably used from there
to minimize network access.

The classes `Commit`, `Tree`, and `Object` (all subclasses of `Entry`) provide
access to the repo content.  Instances can be manipulated with reference
semantics.  Use `copy()` or `deepcopy()` as usual.

`Entry` objects have the following properties in common:

 - `sha1`: Hex string of content id.
 - `content`: A content dict.  `Tree.content` contains entry instances instead
   of the raw JSON returned from the API.
 - `type`: A string with the entry type 'commit', 'tree', or 'object'.
 - `name` (writable): The entry name.
 - `meta`: The meta dict, which can be modified to change the entry.

`Commit` in addition has the following property:

 - `Commit.tree`: An instance of `Tree`.
 - `Commit.idversion`: The JSON format used for computing the sha1.

`Tree` in addition has the following properties and methods:

 - `Tree.entries()` returns an iterator for the entries, with variants
   `Tree.objects()` and `Tree.trees()` to restrict the iteration to entries of
   a certain type; and variants `enumerateEntries()`, `enumerateObjects()` and
   `enumerateTrees()` that yield `(index, entry)` pairs.
 - `Tree.append()`, `Tree.insert()`, and `Tree.pop()` to manipulate the
   entries.  The arguments are similar to the list methods.
 - `Tree.collapse()` detaches the tree from its children.

`Object` in addition has:

 - `Object.blob` is used to get or set the blob.  The property can be set to a
   hex sha1, a path to a file that contains the content, or a `bytes` object
   with the binary data.
 - `Object.text` is used to get or set the text content of an object.  Either
   use blob or text or none of them, but do not use both at the same time.
 - `Object.openBlob()` returns a file object to the blob content.
 - `Object.linkBlob()` creates a hardlink to the blob content at a provided
   path.
 - `Object.copyBlob()` copies the blob content to a provided path.
 - `Object.idversion`: The JSON format used for computing the sha1.
 - `Object.format()`: Change the JSON format used for computing the sha1.

The following functions are provided for nog job management:

 - `postJobProgress()` sends job progress information.
 - `postJobLog()` sends log messages to the job display.
 - `postJobStatus()` is used to implement the execution environment (nogexecd).
   It is not directly used in an analysis programs.

Classes, functions or methods that have not been described so far are primarily
intended for internal use.  In brief:  Class `PostStream` groups content for
posting.  Classes `EntryCache`, `EntryMemCache`, `EntryDiskCache`, and
`BlobCache` handle caching.  Classes `BlobBuf` and `BlobFile` represent blob
content after an assignment to `Object.blob` until upload.
