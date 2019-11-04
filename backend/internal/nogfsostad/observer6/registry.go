package observer6

import (
	"context"
	"io"
	"strings"

	registryevents "github.com/nogproject/nog/backend/internal/fsoregistry/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

func (o *RegistryObserver) ProcessRegistryEvents(
	ctx context.Context,
	registry string,
	tail ulid.I,
	stream pb.Registry_EventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		// Reset state when starting from epoch.
		o.observedRepos = make(map[uuid.I]struct{})

		reg, err := o.loadRegistryView(ctx, stream)
		if err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		if err := o.processView(ctx, reg); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		tail = reg.vid
	}

	return o.watchStream(ctx, tail, stream)
}

func (o *RegistryObserver) loadRegistryView(
	ctx context.Context,
	stream pb.Registry_EventsClient,
) (*registryView, error) {
	reg := newRegistryView()
	if err := o.loadRegistryViewStream(ctx, reg, stream); err != nil {
		return nil, err
	}
	return reg, nil
}

func (o *RegistryObserver) loadRegistryViewStream(
	ctx context.Context,
	reg *registryView,
	stream pb.Registry_EventsClient,
) error {
	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		for _, ev := range rsp.Events {
			err := o.loadRegistryViewEvent(reg, ev)
			if err != nil {
				return err
			}
		}

		if rsp.WillBlock {
			return nil
		}
	}
}

func (o *RegistryObserver) watchStream(
	ctx context.Context,
	tail ulid.I,
	stream pb.Registry_EventsClient,
) (ulid.I, error) {
	for {
		rsp, err := stream.Recv()
		if err != nil {
			return tail, err
		}

		n := o.repoDepMap.Gc()
		if n > 0 {
			o.lg.Infow(
				"Repo activity chains done.",
				"n", n,
			)
		}

		for _, ev := range rsp.Events {
			vid, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				return tail, err
			}
			if err := o.watchEvent(ctx, vid, ev); err != nil {
				return tail, err
			}
			tail = vid
		}
	}
}

func (o *RegistryObserver) loadRegistryViewEvent(
	reg *registryView, ev *pb.RegistryEvent,
) error {
	vid, err := ulid.ParseBytes(ev.Id)
	if err != nil {
		return err
	}
	reg.vid = vid

	// The result of loading is:
	//
	//  - active repos that are below a prefix;
	//  - move-repo workflow acquire parts;
	//
	// A repo is deactivated if a move-repo workflow starts, as if it never
	// existed.  A repo is activated if a move-repo workflow completes, as
	// if it had always been at the new location.
	switch ev.Event {
	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		globalPath := ev.FsoRepoInfo.GlobalPath
		if !pathIsEqualOrBelowPrefixAny(globalPath, o.prefixes) {
			return nil // Ignore repo with foreign prefix.
		}
		repoId, err := uuid.FromBytes(ev.FsoRepoInfo.Id)
		if err != nil {
			return err
		}
		reg.repos[repoId] = repoView{
			id: repoId,
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED:
		x := registryevents.FromPbMust(*ev).(*registryevents.EvRepoMoveAccepted)
		repoId := x.RepoId
		workflowId := x.WorkflowId
		newGlobalPath := x.NewGlobalPath

		// The same Nogfsostad can release and acquire.  Release is the
		// default of `watchRepoActivity`.  Acquire must be specified
		// in `watchRepoActivityOptions`.  We only set state for
		// acquire, because we need no state for the default.
		repo, ok := reg.repos[repoId]
		if pathIsEqualOrBelowPrefixAny(newGlobalPath, o.prefixes) {
			if ok {
				repo.moveRepoWorkflowAcquire = workflowId
				reg.repos[repoId] = repo
			} else {
				reg.repos[repoId] = repoView{
					moveRepoWorkflowAcquire: workflowId,
				}
			}
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVED:
		x := registryevents.FromPbMust(*ev).(*registryevents.EvRepoMoved)
		repoId := x.RepoId
		_ = x.WorkflowId
		globalPath := x.GlobalPath

		// If the new path is not ours, remove the repo.  If it is
		// ours, add the repo as on `RegistryEvent_EV_FSO_REPO_ADDED`.
		if !pathIsEqualOrBelowPrefixAny(globalPath, o.prefixes) {
			delete(reg.repos, repoId)
			return nil
		}
		reg.repos[repoId] = repoView{
			id: repoId,
		}
		return nil

	default:
		return nil
	}
}

func (o *RegistryObserver) processView(
	ctx context.Context,
	reg *registryView,
) error {
	for _, repo := range reg.repos {
		if err := o.processRepoView(ctx, repo); err != nil {
			return err
		}
	}
	return nil
}

func (o *RegistryObserver) processRepoView(
	ctx context.Context,
	repo repoView,
) error {
	// Enable `watchEvent()` processing.
	o.observedRepos.Add(repo.id)

	if err := o.startWatchRepoActivity(
		ctx, repo.id, watchRepoActivityOptions{},
	); err != nil {
		return err
	}
	// If there is an active move-repo workflow acquire part, start a
	// second activity that will take over after the first activity
	// released the repo.  The second activity will start with
	// `moveRepoWorkflowAcquireActivity` and then watch the repo.
	if repo.moveRepoWorkflowAcquire != uuid.Nil {
		if err := o.startWatchRepoActivity(
			ctx, repo.id,
			watchRepoActivityOptions{
				moveRepoWorkflowAcquire: repo.moveRepoWorkflowAcquire,
			},
		); err != nil {
			return err
		}
	}

	return nil
}

func (o *RegistryObserver) watchEvent(
	ctx context.Context, vid ulid.I, ev *pb.RegistryEvent,
) error {
	switch ev.Event {
	// Handle `RegistryEvent_EV_FSO_REPO_ADDED`, which appears in the
	// registry history after the repo history has been initialized, to
	// avoid "unitialized history" errors when processing the repo events,
	// which would happen with `RegistryEvent_EV_FSO_REPO_ACCEPTED`.
	case pb.RegistryEvent_EV_FSO_REPO_ADDED:
		globalPath := ev.FsoRepoInfo.GlobalPath
		repoId, err := uuid.FromBytes(ev.FsoRepoInfo.Id)
		if err != nil {
			return err
		}
		if !pathIsEqualOrBelowPrefixAny(globalPath, o.prefixes) {
			o.lg.Infow(
				"Ignored repo with foreign prefix.",
				"repoId", repoId.String(),
				"repo", globalPath,
			)
			return nil
		}
		if err := o.startWatchRepoActivity(
			ctx, repoId, watchRepoActivityOptions{},
		); err != nil {
			return err
		}
		o.observedRepos.Add(repoId)
		return nil

	// Ignore `RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED`.
	// `watchRepoActivity` will automatically retry pending operations on
	// `RepoEvent_EV_FSO_REPO_ERROR_CLEARED`.
	case pb.RegistryEvent_EV_FSO_REPO_REINIT_ACCEPTED:
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED:
		x := registryevents.FromPbMust(*ev).(*registryevents.EvRepoMoveAccepted)
		repoId := x.RepoId
		workflowId := x.WorkflowId
		newGlobalPath := x.NewGlobalPath

		// The same Nogfsostad can release and acquire.
		ignored := true
		if o.observedRepos.Has(repoId) {
			// A `watchRepoActivity` for the repo is already
			// running.  Its will run
			// `moveRepoWorkflowReleaseActivity` to completion and
			// quit.
			ignored = false
		}
		if pathIsEqualOrBelowPrefixAny(newGlobalPath, o.prefixes) {
			ignored = false
			// If the repo remains below a prefix, start a new repo
			// activity that will start with
			// `moveRepoWorkflowAcquireActivity` and then watch the
			// repo.
			if err := o.startWatchRepoActivity(
				ctx, repoId,
				watchRepoActivityOptions{
					moveRepoWorkflowAcquire: workflowId,
				},
			); err != nil {
				return err
			}
		}
		if ignored {
			o.lg.Infow(
				"Ignored unknown moving repo with foreign prefix.",
				"repoId", repoId.String(),
				"repo", newGlobalPath,
			)
		}
		return nil

	case pb.RegistryEvent_EV_FSO_REPO_MOVED:
		x := registryevents.FromPbMust(*ev).(*registryevents.EvRepoMoved)
		repoId := x.RepoId
		_ = x.RepoEventId
		globalPath := x.GlobalPath

		// If the new path is ours, add the repo to the registry event
		// processing.  Otherwise remove it.
		//
		// Do not `startWatchRepoActivity()` here, because it has
		// already been called on
		// `RegistryEvent_EV_FSO_REPO_MOVE_ACCEPTED` if we are
		// acquiring the repo.
		if pathIsEqualOrBelowPrefixAny(globalPath, o.prefixes) {
			o.observedRepos.Add(repoId)
		} else {
			o.observedRepos.Delete(repoId)
		}
		return nil

	default: // Silently ignore unknown events.
		return nil
	}
}

// `prefixes` with trailing slash.
func pathIsEqualOrBelowPrefixAny(path string, prefixes []string) bool {
	path = ensureTrailingSlash(path)
	for _, pfx := range prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if s[len(s)-1] == '/' {
		return s
	}
	return s + "/"
}
