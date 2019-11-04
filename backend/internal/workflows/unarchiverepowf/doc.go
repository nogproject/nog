/*

Package `unarchiverepowf` implements the unarchive-repo ephemeral workflow.

Workflow Events

The workflow is initiated by gRPC `BeginUnarchiveRepo()`.  It starts the
workflow with `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED` on the workflow and
a corresponding `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_STARTED` on the ephemeral
registry workflow index.

Nogfsoregd observes the workflow.  It changes the repo state to archiving in
the registry with `RegistryEvent_EV_FSO_UNARCHIVE_REPO_STARTED` and on the repo
with `RepoEvent_EV_FSO_UNARCHIVE_REPO_STARTED`.  It then posts
`WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_STARTED` on the workflow to notify
Nogfsostad.

Nogfsostad observes the workflow.  It creates the working directory and saves
it in `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_STARTED` to tell Nogfsorstd to
start `tartt restore`.

Nogfsorstd observes the workflow.  It restores the tartt archive to the working
directory and then posts `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_TARTT_COMPLETED`
to notify Nogfsostad.  Errors may be handled by retrying or aborting the
workflow.

Nogfsostad applies ACLs, swaps the restored data with the realdir placeholder,
and posts `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMPLETED`.  Errors may be
handled by retrying or by aborting the workflow.

Nogfsoregd then completes the main workflow work:
`RepoEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` on the repo,
`RegistryEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` on the registry, and
`WorkflowEvent_EV_FSO_UNARCHIVE_REPO_FILES_COMMITTED`.

Nogfsostad regularly checks whether the garbage has expired.  When it has
expired, Nogfsostad removes the garbage and posts
`WorkflowEvent_EV_FSO_UNARCHIVE_REPO_GC_COMPLETED`.

Nogfsoregd then completes the workflow:
`WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` on the workflow,
`WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMPLETED` on the ephemeral registry
workflow index, and a final `WorkflowEvent_EV_FSO_UNARCHIVE_REPO_COMMITTED` on
the workflow.

The final workflow event has no observable side effect.  Its only purpose is to
explicitly confirm termination of the workflow history.  The final event may be
missing if a multi-step command to complete the workflow was interrupted.

The workflow is eventually deleted from the index with
`WorkflowEvent_EV_FSO_UNARCHIVE_REPO_DELETED` on the ephemeral registry
workflow index.  A workflow may be deleted with or without the final workflow
event.

Possible State Paths

Successful unarchive: StateInitialized, StateFiles, StateTartt,
StateTarttCompleted, StateFilesCompleted, StateFilesEnded, StateGcCompleted,
StateCompleted, StateTerminated.

Error during begin registry or begin repo: StateInitialized, StateFailed,
StateTerminated.

Error during tartt restore: StateInitialized, StateFiles, StateTartt,
StateTarttFailed, StateFilesEnded, StateGcCompleted, StateFailed,
StateTerminated.

Error while moving swapping restored files with the realdir placeholder:
StateInitialized, StateFiles, StateTartt, StateTarttCompleted,
StateFilesFailed, StateFilesEnded, StateGcCompleted, StateFailed,
StateTerminated.

*/
package unarchiverepowf
