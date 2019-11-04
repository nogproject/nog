```{.bg-warning}
RETIRED: The package has never been used in production.  It is only a proof of
concept and probably won't work with the latest Meteor version.  We may remove
it completely at some point in the future.
```

# Package `nog-sync` (experimental)

`nog-sync` implements content synchronization between Nog deployments that each
have a separate MongoDB.

It is an experimental implementation, which must not be used in production.

## Design

The state of all content repos at snapshot time is represented as
a content-addressable hash prefix tree.  The snapshot sequence is represented
as a commit history.  A new commit is only added if there are changes, so that
multiple deployments can converge on a single sync commit.  A repo that
contains a snapshot history is called a 'synchro'.

Sync state is maintained in separate MongoDB collections.  Some `nog-content`
code is used.  We use 'sync' and 'synchro' to distinguish entries from
'content' if needed for clarity; examples: sync tree vs content tree, synchro
vs repo, ... .

The repo snapshot representation is hierarchical to be obviously scalable to
larger number of repos (100k and more).  We currently use a fixed partition
scheme based on a SHA of the full repo name.

The fixed scheme would ideally be replaced by a hash trie whose depth adapts to
the number of repos.  The following scheme seems reasonable: Each level
represents one or two hex characters.  Leaf nodes are identified by a naming
convention.  Inner nodes are only added when needed to resolve collisions.

Commit date order is used when walking the history.  Git also uses this
approach.  It seems to be a reasonable heuristic to terminate walks as soon as
possible.  It is used for computing merge bases and when traversing history
during fetch.

A local history walk is used for fetching history from a stateless server.
This approach is easy to implement and seems simpler than a fetch cursor that
maintains state on the server.

Prefetching could later be added to hide latency.  The server could send
multiple commits in commit-date order.  It seems likely that the client will
use them.  The number of prefetch commits can be used to balance unnecessary
data transfer vs latency hiding.

Content is divided into three groups with different invariants: commits, trees
and objects, and blobs.

Commits no longer require that the full history is recursively stored.  This is
similar to a 'shallow' clone in Git.  Shallow history seems to be a good option
to avoid infinite data growth and limit data size to the currently relevant
information plus some overhead for the recent history.

The current implementation does not store information about the shallowness.
A fetch walk, for example, has to continue until it finds a ref that is
guaranteed to be reachable from a bottom ref that was already locally present
in order to guarantee that the complete history back to the old ref is fetched.
A walk cannot stop if it finds any commit that is available locally, since the
commit might be shallow.  This is different from Git, which explicitly stores
the limiting commits of a shallow history.

We may want to add some guarantees about the history: It might, for example, be
useful to guarantee that a repo always contains history for the last N days
plus M extra commits (in commit date order).  fsck could check such guarantees.

Trees and objects are required to be deep, which means they are required to
have all their children recursively stored, too.  This ensures that the current
repo tree is always complete.

Objects, however, may refer to blobs that are only placeholders for blob data
that is copied asynchronously.  The blob doc must be present, but the object
store data may not.  This limitation seems necessary to be able to obviously
achieve reasonably quick convergence of the metadata.  We want anyway to be
able to serve blobs from multiple locations or move them to offline tape
storage, keeping only a placeholder.  Clients must be able to handle blobs that
are not immediately available.  The placeholder approach to synchronization
fits into this general approach.

Synchronization between multiple Nog deployments works similar to a permanent
commit, fetch, merge, checkout loop in Git.  Each part must have an online
implementation that scales with the number of changes in order to get
a synchronization scheme that overall scales to a large number of repos.

Synchronization is handled in an event loop.  Observers, which can monitor
MongoDB to detect local changes, trigger updates.  Deployments connect via DDP.
Observers monitor the synchronization snapshots of other deployments via a DDP
subscription.  Since the full state is represented in a single sync snapshot,
the subscription will only contain a single document.

'commit' is implemented as an incremental update of the repo snapshot tree.
Recent changes are determined based on the timestamp `mtime` that is maintained
on the relevant mongo docs.  We may consider an alternative design in the
future that would use an event stream: imagine an oplog on the content store,
which can be tailed to process changes, or a Git reflog to.

'fetch' is implemented as a local commit walk that fetches remote entries via
DDP method calls.  It stores remote refs under `remotes/` in the local synchro.
It also fetches commits for the content repos that changed at the remote, so
that a merge is a completely local operation.

'merge' uses the same logic as Git's merge recursive.  The recursive logic is
necessary to avoid spurious conflicts with crisscross merges, which happen if
several peers are allowed to modify repos simultaneously.  See blob post 'Merge
recursive strategy" <http://goo.gl/rPGGKv> for an illustration of the general
problem and the Git source code of `merge_recursive` and `paint_down_to_common`
for details.

The merge bases are determined from the sync history by painting the history
from `ORIG_HEAD` (synonyms: a, us, ours) and `MERGE_HEAD` (synonyms: b, them,
theirs) in commit-date order until the paint meets.  Fast-forwards and
identical trees are trivial.  For a proper merge, a sync tree diff stream is
computed that contains all changes from the base to a and b.  If changes are
only on one side, they are applied.  If both sides change, a conflict is
recorded in the sync tree.

Caching sync trees in memory is necessary to get acceptable performance.

The main conflict handling is completely automatic.  Non-conflicting repos
continue to synchronize even if some repos have conflicts.  Conflicting repos
do not block the process.  Instead the conflict state is stored and propagated,
so that all deployments agree on the conflict state.  Conflict resolution can
be performed on a per-repo basis later.  A conflict is represented in the sync
tree as an array of alternative commits in `conflicts.<refName>`.

We decided against an alternative design that would associate conflicts with
their peer name to keep track of the conflict origin.  Simply primary reason
for the chosen design is simplicity: it stores alternatives without associating
them with a specific peer; merges are symmetric.

Conflicts are stored similarly in local repo.  The local ref is, however,
removed from the conflict set if possible.  Clients that are unaware of
conflicts continue to work as if the local branch was the only one.

Changes from the sync tree are applied to the local repos, which roughly
corresponds to Git checkout.  Apply must handle concurrent update to the local
repos.  To do so, it computes a diff between `ORIG_HEAD` and `HEAD` and applies
only the diff.  If the local repo has changed compared to `ORIG_HEAD`,
a conflict is stored in the local repo, which will be stored as a conflict in
the next sync snapshot.

The initial implementation only handles master and ignores all other branches.
The naming conventions are chosen such that supporting further refs is
straightforward.

Per-content-repo conflict resolution is not yet implemented.  A first approach
could be to use manual intervention with scripts based on nogpy.  The API may
need to be extended to provide atomic multi-ref updates, such as to remove all
`conflicts/*` and set `branches/master`.  Conflict resolution will then
propagate via the sync snapshots to the other remotes.

The sync snapshot state uses natural ids, like owner name and repo name.
MongoDB ids are considered local and will not be propagated to remotes.
Natural ids are used anyway in the API and in local copies of repos, so we need
to handle them correctly.  Any state that needs to be synchronized will use
natural ids or content addresses; examples: repo names, users, sharing groups.

Access control is relatively simple.  The initial implementation focuses on
synchronizing full deployments as a root user.  It seems sufficient to protect
the main entry points and then use low-level MongoDB operations on the entire
database without repo set checks.

The sync endpoint will be run as a separate app that is connected to the same
MongoDB as the main app.  The main app need not include the sync code.  The
sync app endpoint can be protected by additional security measures such as
IP-based access control.  A single separate sync instance also ensures, without
locking, that only a single thread of execution drives the sync loop.
