# Package `nog-repo-toolbar`

`nog-repo-toolbar` provides a template `nogRepoTopBarPresenter` for repository
views that combines:

- viewer navigation,
- repository tools, e. g., repo forking, and
- repository information, such as path or `forkedFrom` information.

The templates are split into smart and reusable components:
`nogRepoTopBarPresenter` receives dependencies and data from the viewer
template. It manages the subscription of the current repository and passes the
data into the context of the sub-templates `nogRepoToolbar`, and `forkedFrom`,
and `viewerButtons` (from `nog-widget`) to load them as reusable components.
See `nog-catalog` as an example for the described layout.

Currently, the sub-templates also contain the deprecated (not reusable)
implementation, so that they can still be loaded by legacy parents in other
views (files, technical, and workspace).

If the subscribed repo does not exist or access was denied, the template
displays a warning.  Nonetheless, the parents that load
`nogRepoTopBarPresenter` should also do access checks and display general
`Access denied` messages instead of rendering the tool bar.
