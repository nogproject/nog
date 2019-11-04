### Endpoint

The API is accessible at the same URL as the app, using a prefix.  The paths
for the API methods are described relative to the prefix.  Clients are
encouraged to request a specific API version.

The current version is at:

    https://<host>/api/v1

### Data Model

Data is organized in repos, like in git.  A repo contains mutable state,
primarily refs.  refs are like git refs.  By convention, we currently use only
`branches/master` (which corresponds to `heads/master` in git).  We will
probably define a meaning for other prefixes later (like tags).

As in git, a ref points to a commit.  Like a git commit, a nog commit contains
information about authors, dates, a message, and so on.  A commit points to
parent commits and to a tree.  Unlike a git commit, a nog commit can contain
a dictionary of metadata, which may be useful when implementing specific
workflows.

A tree has a name, a dictionary of metadata, and a list of child `entries` of
format `{type: object|tree, sha1: <id>}`.  Tree is a recursive data structure.
The leaf nodes are objects.  Objects also have a name and a metadata
dictionary; and they can point to a blob.  A blob represents a binary object
that is stored in object storage (currently S3).

Trees can be used similar to a file system or a git tree.  A main difference is
that the nog tree entries are an ordered list and may contain entries with
duplicate names.  By convention, we usually avoid duplicate names and use
a tree like a hierarchical file system.

We use conventions to give some trees and objects a certain meaning.  Objects
with name `*.md` are assumed to contain markdown in `text` with `blob=null`.
Objects that are named like an image file, like `*.png`, are expected to have
a blob that contains the binary image data.  An example for a convention on
trees is `meta.workspace`.  If it is present, the tree is expected to represent
a workspace with certain entries, like `datalist`, `programs`, `jobs`, and
`results`.

Immutable content has an id that is computed as the sha1 over a canonical JSON
format (see below for technical details).  The documents stored in the database
may contain additional non-essential fields that are not part of the canonical
format.  The most obvious example is `_id`, which is the computed sha1.

Updates to a repo work similar to git on a low level: Get the ref, then the
commit for `branches/master`.  Then get the tree and modify it; or construct
a new tree from scratch; only the result matters.  Construct a commit that
points to the tree and to the previous commit.  Post everything in dependency
order, and finally update the ref, passing in the previous state as a nonce in
order to protect against concurrent writes.  It should be clear how to do
this with the API routes below.  Language bindings may offer convenience
functions that operate on a higher level and use caching.  Since all content
is immutable (except for repos), caching is easy.

### Content Ids

All immutable content entries (such as objects, trees and commits) have ids
that are computed as sha1s over a canonical EJSON format.  The input `content`
is a minimal format (without `href`) that includes all optional fields.  See
examples below at 'create a commit', 'create an object', and 'create a tree'.
The canonical EJSON format is JSON with UTF-8-encoded strings, sorted keys, and
separators without whitespace.

There may be several different canonical formats for entry types.  The format
version that must be used to reproduce the sha1 id is indicated by an integer
`_idversion`.  Clients should always be updated as soon as possible to handle
new versions correctly and keep code to handle older versions for
compatibility.  Clients should check the `_idversion` and handle an unknown
version as an error.

The details for each version are documented below at the respective 'create
a ...' sections.  Briefly:

 - Commit format 0 supported only UTC Z date times.
 - Commit format 1 added timezone support.
 - Object format 0 by convention used `meta.content` for fulltext.
 - Object format 1 added a toplevel field `text` to store fulltext.
 - Tree format 0 is the only tree format.

`_idversion` is not part of the canonical content and must be removed before
computing the sha1.  `errata` (see below) must also be removed.

Computing an id in CoffeeScript:

```{.coffee}
sha1Hex = (d) -> CryptoJS.SHA1(d).toString()
contentId = (content) -> sha1Hex(EJSON.stringify(content, {canonical: true}))
```

Computing an id in Python:

```{.python}
def stringify_canonical(content):
    return json.dumps(
        content, sort_keys=True, ensure_ascii=False, separators=(',', ':'),
    ).encode('utf-8')

def contentId(content):
    h = hashlib.sha1()
    h.update(stringify_canonical(content))
    return h.hexdigest()
```

### Errata

Due to a bug in the client-side SHA1 computation in browsers, correct blob data
was stored under an incorrect blob id in a few cases during early development.
The blobs and objects became part of the commit history.  We wanted to keep the
history but somehow mark the incorrect objects.

Since entries are immutable, the inconsistent ids cannot be modified but must
remain part of the immutable history.  To handle such situations, content
entries can have an optional field `errata` with a list of errata codes.
`errata` must be removed when verifying the entry's id.  The meaning of the
errata codes is deployment-specific.
