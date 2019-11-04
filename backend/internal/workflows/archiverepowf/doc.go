/*

Package `archiverepowf` implements the archive-repo ephemeral workflow.

Workflow Events

The workflow is initiated by gRPC `BeginArchiveRepo()`.  It starts the workflow
with `WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED` on the workflow and a
corresponding `WorkflowEvent_EV_FSO_ARCHIVE_REPO_STARTED` on the ephemeral
registry workflow index.

Nogfsoregd observes the workflow.  It changes the repo state to archiving in
the registry with `RegistryEvent_EV_FSO_ARCHIVE_REPO_STARTED` and on the repo
with `RepoEvent_EV_FSO_ARCHIVE_REPO_STARTED`.  It then posts
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_STARTED` on the workflow to notify
Nogfsostad.

Nogfsostad observes the workflow.  It polls the shadow repo until the full
Tartt archive appears and then posts
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_TARTT_COMPLETED`.

Nogfsostad determines the temporary placeholder location and the garbage
location and saves them in `WorkflowEvent_EV_FSO_ARCHIVE_REPO_SWAP_STARTED` for
a possible restart.  It then prepares the placeholder, swaps it with the
realdir, moves the original data to the garbage location, runs `git-fso
archive` to update the shadow repo, and posts
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMPLETED`.  Errors may be handled by
retrying or by aborting the workflow.

Nogfsoregd then completes the main workflow work:
`RepoEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` on the repo,
`RegistryEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` on the registry, and
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_FILES_COMMITTED`.

Nogfsostad regularly checks whether the garbage has expired.  When it has
expired, Nogfsostad removes the garbage and posts
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_GC_COMPLETED`.

Nogfsoregd then completes the workflow:
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` on the workflow,
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMPLETED` on the ephemeral registry
workflow index, and a final `WorkflowEvent_EV_FSO_ARCHIVE_REPO_COMMITTED` on
the workflow.

The final workflow event has no observable side effect.  Its only purpose is to
explicitly confirm termination of the workflow history.  The final event may be
missing if a multi-step command to complete the workflow was interrupted.

The workflow is eventually deleted from the index with
`WorkflowEvent_EV_FSO_ARCHIVE_REPO_DELETED` on the ephemeral registry workflow
index.  A workflow may be deleted with or without the final workflow event.

Possible State Paths

Successful archive: StateInitialized, StateFiles, StateTarttCompleted,
StateSwapStarted, StateFilesCompleted, StateFilesEnded, StateGcCompleted,
StateCompleted, StateTerminated.

Error during begin registry or begin repo: StateInitialized, StateFailed,
StateTerminated.

Error when archiving files: StateInitialized, StateFiles, maybe
StateTarttCompleted, maybe StateSwapStarted, StateFilesFailed, StateFilesEnded,
StateGcCompleted, StateFailed, StateTerminated.

*/
package archiverepowf
