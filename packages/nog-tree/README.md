# Package `nog-tree`

`nog-tree` implements a tree browsing user interface to nog repositories for
users that want to access the technical details.  Some elements are inspired by
GitHub's web UI to browse git repositories.

Only a subset of functions is documented here.  See the source for more
information.

## `NogTree.registerEntryRepr(spec)` (client)

`registerEntryRepr()` registers a new representation for entries to be used
for tree browsing.  `spec` must have a function `spec.selector(context)`, which
is passed the tree data context.  The selector must return either the name of
the Meteor template to use for rendering, or it returns `null` to indicate that
the representation should be ignored.

The most relevant fields in the tree data context are (see the code in
`nog-tree-ui.*` for details):

 - `commitId` (sha1) and `commit` (full object).
 - `namePath` (Array of Strings): Names of entries along the resolved path
   from the tree root.
 - `numericPath` (Array of Numbers): Indices of entries along the resolved
   path from the tree root.
 - `contentPath` (Array of Objects): Full information about entries along the
   resolved path from the tree root.  Each object contains the index in the
   parent tree entries in `idx`; the name of the entry in `name`; the type of
   the entry in `type`; the entry content in `content`.
 - `last`: An alias to the last entry in `contentPath`.
 - `tree`: The root tree in the same format as the `contentPath` elements.
   The root tree itself is not part of the `contentPath`.
 - `repo`: The repo object.
 - `ref`, `refTreePath`, `refType`, and `treePath`: Information about the
   path (see code for details).

Individual packages can directly register representations, which requires
a package dependency on nog-tree.  An alternative approach is to configure the
repr spec in the main app.  We do not yet know, which architecture works better
in practice.

## `NogTree.selectEntryRepr(context)` (client)

`selectEntryRepr()` is internally used to select, based on the tree data
context, a template for rendering an entry .  It returns `null` if no entry
repr selector matches, which indicates that the default view should be used.
