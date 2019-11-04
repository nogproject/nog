# Package `nog-content`

`nog-content` implements a git-like, content-addressable data model.

The design overview from March 2015 in
[2015_fuimages_meteor-spikes:design-overview_2015-03.md](./../../../design-overview_2015-03.md)
may contain further ideas that are not yet implemented.

The REST API is described in detail in the separate doc:
[apidoc](./apidoc.md).

## Data model

The entry point is a repo.  It contains mutable state, primarily `refs`.  `refs`
are like git refs.  Initially, `branches/*` is used for branches (instead of
`heads/` in git).  Other prefixes may be useful later (like tags).

`staging` will be used for preparing a commit before actually committing it.
`staging` is not yet implemented in nog-content; it has been used in spikes and
is described in the design overview document.  `staging` works like a local
temporary branch with a commit that is repeatedly amended until it is ready to
be committed.  It seems reasonable to use a temporary commit.  It can be used to
store the state of the edit form, such as the draft of the commit message.  An
alternative would be to use a different data structure for preparing commits
that would be more similar to git's index.  This could be useful if the editing
operation needs more state, such as multiple versions of a file for conflict
resolution.  For now, a temporary commit seems fine.

The next level is a commit.  Like a git commit, it contains information like
authors, dates, a message, and so on.  A commit points to parent commits and to
a tree.

A tree contains a dictionary of metadata and a list of entries of format
`{type: object|tree, sha1: <id>}`.  It is a recursive data structure.  The leaf
nodes are objects.  Objects also contain metadata, and can point to a blob.
A blob represents a binary object that is stored in object storage (like S3).

All immutable objects have ids that are computed as sha1s over a canonical EJSON
representation.  The documents stored in MongoDB may contain additional
non-essential fields that are not part of the canonical representation.  The
most obvious example is `_id`, which is the computed sha1.  Another candidate is
`touchTime` to store when the document was used, which might be relevant when
implementing a time-based garbage collection scheme.

### `NogContent.repos` (server, subset at client)

The Mongo collection `repos` contains the repositories.  Repositories contain
mutable state.  A repo has a random `_id` and a unique full repo name, which is
composed from the repo owner name and the repo name: `<ownerName>/<repoName>`.

### `NogContent.commits` (server, subset at client)

The Mongo collection `commits` contains the immutable commit entries.

### `NogContent.trees` (server, subset at client)

The Mongo collection `trees` contains the immutable tree entries.

### `NogContent.objects` (server, subset at client)

The Mongo collection `objects` contains the immutable object entries.

### `NogContent.blobs` (server, subset at client)

The Mongo collection `blobs` is a reference to `NogBlob.blobs` if the packages
`nog-blob` is available (weak dependency) and `null` otherwise.

### `NogContent.contentId(content)` (anywhere)

`contentId(content)` computes the id for `content`, which must contain only
valid fields.  The calling code, for example, must remove fields that start
with underscore.  One way is to use `NogContent.stripped()` as in
`contentId(stripped(content))`.

### `NogContent.strip(content)` (anywhere)

`strip(content)` removes special internal fields like `_id` and `_idversion`
from `content`.  `content` is modified in place.

### `NogContent.stripped(content)` (anywhere)

`stripped(content)` returns a copy of `content` without internal fields.

## `NogContent.configure(opts)` (server)

`configure()` updates the active configuration with the provided `opts`:

 - `checkAccess` (`Function`, default `NogAccess.checkAccess` if available) is
   used for access control.

 - `testAccess` (`Function`, default `NogAccess.testAccess` if available) is
   used for access control.

### `NogContent.checkAccess(user, action, opts)` (server)

The hook `NogContent.checkAccess(user, action, opts)` is called to check whether
a user has the necessary upload and download permissions.  See package
`nog-access`.

### `Meteor.settings.optStrictRepoMembership` (server)

The feature toggle `optStrictRepoMembership` (default: `true`) controls whether
strict repo membership checks are enabled.  If active, entries can only be
accessed via a repo when they are reachable via a ref or when they have been
recently added to the repo.  This check should be activated if some kind of
strict content sharing permissions are used, such as sharing only with selected
user circles.  If active, `nog-content` will configure `nog-blob` to check
whether blobs are reachable from a repo.

## `NogContent.api.repos.actions()` (server)

`NogContent.api.repos.actions()` returns an action array that can be plucked
into `nog-rest` to provide a REST API.

The REST API is described in detail in the separate doc:
[apidoc](./apidoc.md).

The Python example `content-testapp/public/tools/bin/test-create-content-py`
demonstrates the REST API.

If `nog-blob` is used, `NogBlob.api.blobs.actions()` and
`NogBlob.api.upload.actions()` must be mounted at corresponding paths.

Usage example:

```{.coffee}
if Meteor.isServer
  NogRest.actions '/api/repos', NogContent.api.repos.actions()
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.blobs.actions()
  NogRest.actions '/api/repos/:ownerName/:repoName/db/blobs',
    NogBlob.api.upload.actions()
```

## `NogContent.call.*` (anywhere, internal use)

The object `NogContent.call` provides Meteor methods that are used internally,
such as `NogContent.createRepo()`.
