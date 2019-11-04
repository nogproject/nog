/*

Package `splitrootwf` implements the split-root ephemeral workflow, which
analyzes the disk usage below a root an suggests new repos.

Registry Configuration Events

The configuration is stored as essential registry state.

`RegistryEvent_EV_FSO_SPLIT_ROOT_ENABLED` enables the workflow for a root, so
that an admin can use `nogfsoctl split-root begin`.

`RegistryEvent_EV_FSO_SPLIT_ROOT_DISABLED` disables the workflow.

`RegistryEvent_FSO_SPLIT_ROOT_PARAMS_UPDATED` sets new workflow parameters for
a root.  A workflow uses default parameters if the parameters were never set.

`RegistryEvent_EV_FSO_PATH_FLAG_SET` adds a path flag, such as
`DONT_SPLIT`.  Path flags are conceptually a set `{ (path, flag) }`.  The flags
are encoded as a bit mask.  `RegistryEvent_EV_FSO_PATH_FLAG_UNSET` removes a
path flag.

Workflow Events

An admin uses `nogfsoctl split-root begin` to initialize a workflow to
determine the current disk usage and compare it to the existing repos in order
to suggest new repos.  The first events are
`WorkflowEvent_EV_FSO_SPLIT_REPO_STARTED` on the workflow and a corresponding
`WorkflowEvent_EV_FSO_SPLIT_REPO_STARTED` on the ephemeral registry workflow
index.

Nogfsostad observes the workflow.  It posts the `du` output as multiple
`WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_APPENDED`, followed by
`WorkflowEvent_EV_FSO_SPLIT_ROOT_DU_COMPLETED`.

Nogfsoregd analyzes the disk usage and posts suggestions as a series of
`WorkflowEvent_EV_FSO_SPLIT_ROOT_SUGGESTION_APPENDED` followed by
`WorkflowEvent_EV_FSO_SPLIT_ROOT_ANALYSIS_COMPLETED`.

The admin uses `nogfsoctl split-root get` and `nogfsoctl split-root decide` to
post the conclusion as a series of
`WorkflowEvent_EV_FSO_SPLIT_ROOT_DECISION_APPENDED`, which may initialize new
repos and modify the registry split-root configuration.

The admin finally uses `nogfsoctl split-root commit` to complete the workflow
with `WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED` on the workflow,
`WorkflowEvent_EV_FSO_SPLIT_ROOT_COMPLETED` on the workflow index, and a final
`WorkflowEvent_EV_FSO_SPLIT_ROOT_COMMITTED` on the workflow.

The admin can use `nogfsoctl split-root abort` to cancel the workflow without
decisions.

XXX Nogfsoregd could be change to automatically commit the workflow when there
are no more pending decisions.

The final workflow event has no observable side effect.  Its only purpose is to
explicitly confirm termination of the workflow history.  The final event may be
missing if a multi-step command to complete the workflow gets interrupted.

Workflows are eventually deleted from the index with
`WorkflowEvent_EV_FSO_SPLIT_ROOT_DELETED` on the index.  Workflows may be
deleted with or without the final `WorkflowEvent_EV_FSO_SPLIT_ROOT_COMMITTED`
on the workflow.

Authorization

See implementation in `nogfsoregd/registryd/split-root.go` for details.

Workflow config:

 - A root admin may read the config.
 - But only an registry admin may change it.

 - read: action `fso/read-root` on root path.
 - create, update, delete: action `fso/admin-registry` on registry name.

Path flags:

 - A root admin may read and add flags.
 - But only an registry admin may delete flags.
 - Nogfsoregd may list the flags for workflow execution.

 - create: action `fso/admin-root` on root path.
 - delete: action `fso/admin-registry` on registry name.
 - list: action `fso/read-root` or `fso/exec-split-root` on root path.

Workflow:

 - A root admin may start a workflow.
 - Nogfsostad and Nogfsoregd may execute their tasks.
 - A root admin may append decisions and commit.
 - Nogfsoregd may abort the workflow to handle errors.

 - begin: action `fso/admin-root` on root path.
 - append, commit, abort du: action `fso/exec-du` on root path.
 - append, commit, abort suggestions: action `fso/exec-split-root` on root
   path.
 - decisions: action `fso/admin-root` on root path and `fso/init-repo` for
   new repo paths.
 - commit workflow: action `fso/admin-root` on root path.
 - abort workflow: action `fso/admin-root` or `fso/exec-split-root` on root
   path.

*/
package splitrootwf
