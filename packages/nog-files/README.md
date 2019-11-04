# Package `nog-files`

`nog-files` implements a file browser, which should be as non-technical as
possible.  A UI inspired by Dropbox is probably suitable.

Only a subset of functions is documented here.  See the source for further
details.

## `NogFiles.registerEntryRepr(spec)` (client)

`spec` must provide selector functions and may provide access control
functions.  The selectors return either the name of a Meteor template, which
will be used for rendering, or `null` to indicate that it should be ignored.
The access controls either return a control object or `null` to indicate that
it should be ignored.

The following selectors must be present:

 - `view(treeContext)`: The view template.  You can return `nogFilesBundleView`
   to use a generic bundle view for a tree that should not be modified.

 - `icon(entryContext)`: The icon template for a list view.  You can return
   `nogFilesBundleIcon` to use a generic bundle icon for a tree that should not
   be modified.

The following access controls may be present:

 - `treePermissions(treeContext)`: Can be used to restrict the actions that may
   be performed in a file listing.  Return `{write: false}` to disable all
   operations that would modify the tree.

The most relevant fields in `treeContext` are (see the code in `nog-files-ui.*`
for details; the context is the same as for nog-tree reprs):

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

The relevant fields in `entryContext` are:

 - `context.parent`: The `treeContext` (as described above) for the parent
   tree.
 - `context.child.content`: The content of the entry for which the icon
   shall be returned.

Individual packages can directly register representations, which requires
a package dependency on `nog-files`.  An alternative approach is to configure
the repr spec in the main app.  We do not yet know, which architecture works
better in practice.

## `NogTree.entryView(treeContext)` (client, internal)

`entryView()` is used internally to select the template for displaying an
entry.

## `NogTree.entryIcon(entryContext)` (client, internal)

`entryIcon()` is used internally to select the template used for the icon in
a file list view.
