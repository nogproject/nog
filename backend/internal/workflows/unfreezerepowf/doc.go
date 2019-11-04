/*

Package `unfreezerepowf` implements the unfreeze-repo ephemeral workflow.

Workflow Events

Some events have a suffix `2` to distinguish them from events that were used in
an earlier, experimental implementation (see NOE-23).

The workflow is initiated by gRPC `BeginUnfreezeRepo()`.  It starts the
workflow with `WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2` on the workflow and
a corresponding `WorkflowEvent_EV_FSO_UNFREEZE_REPO_STARTED_2` on the ephemeral
registry workflow index.

Nogfsoregd observes the workflow.  It changes the repo state to freezing in the
registry with `RegistryEvent_EV_FSO_UNFREEZE_REPO_STARTED_2` and on the repo with
`RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED_2`.  It then posts
`WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_STARTED` on the workflow to notify
Nogfsostad.

Nogfsostad observes the workflow.  It unsets the immutable file attributes,
commits the shadow repo, and posts
`WorkflowEvent_EV_FSO_UNFREEZE_REPO_FILES_COMPLETED` on the workflow.

Nogfsoregd then completes the workflow:
`RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2` on the repo,
`RegistryEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2` on the registry,
`WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2` on the workflow,
`WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMPLETED_2` on the ephemeral registry
workflow index, and a final `WorkflowEvent_EV_FSO_UNFREEZE_REPO_COMMITTED` on
the workflow.

The final workflow event has no observable side effect.  Its only purpose is to
explicitly confirm termination of the workflow history.  The final event may be
missing if a multi-step command to complete the workflow was interrupted.

The workflow is eventually deleted from the index with
`WorkflowEvent_EV_FSO_UNFREEZE_REPO_DELETED` on the ephemeral registry workflow
index.  A workflow may be deleted with or without the final workflow event.

*/
package unfreezerepowf
