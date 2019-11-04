package observer6

import (
	"context"

	pbevents "github.com/nogproject/nog/backend/internal/fsorepos/pbevents"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type watchRepoActivity struct {
	chainedRepoActivity
	initer            Initializer
	proc              Processor
	workflowEngine    grpcentities.RepoWorkflowEngine
	conn              *grpc.ClientConn
	sysRPCCreds       grpc.CallOption
	opts              watchRepoActivityOptions
	state             watchRepoActivityState
	nStorageOpRetries int
}

type watchRepoActivityOptions struct {
	moveRepoWorkflowAcquire uuid.I
}

type watchRepoActivityState struct {
	hasError            bool
	pendingInitShadow   bool
	pendingEnableGitlab bool
}

type watchRepoView struct {
	vid                  ulid.I
	hasError             bool
	wantsShadow          bool
	hasShadow            bool
	pendingEnableGitlab  bool
	repoMoveWorkflowId   uuid.I
	moveShadowWorkflowId uuid.I
	storageOpAuthorName  string
	storageOpAuthorEmail string
}

func (a *watchRepoActivity) ProcessRepoEvents(
	ctx context.Context,
	repoId uuid.I,
	tail ulid.I,
	stream pb.Repos_EventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		repo := watchRepoView{}
		if err := loadRepoStreamEventsNoBlock(
			&repo, stream,
		); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		a.state = watchRepoActivityState{}
		done, err := a.processView(ctx, repo)
		switch {
		case err != nil:
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		case done:
			return repo.vid, nil
		}

		tail = repo.vid
	}

	return watchRepoStreamEvents(a, ctx, tail, stream)
}

func (repo *watchRepoView) loadRepoEvent(vid ulid.I, ev *pb.RepoEvent) error {
	// Ok to update `vid` early, because the code below cannot fail.
	repo.vid = vid

	switch ev.Event {
	case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
		repo.wantsShadow = true
		return nil

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
		repo.hasShadow = true
		return nil

	case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
		repo.pendingEnableGitlab = true
		return nil

	case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
		repo.pendingEnableGitlab = false
		return nil

	case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
		repo.hasError = true
		return nil

	case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
		repo.hasError = false
		return nil

	case pb.RepoEvent_EV_FSO_REPO_MOVE_STARTED:
		x := pbevents.FromPbMust(*ev).(*pbevents.EvRepoMoveStarted)
		repo.repoMoveWorkflowId = x.WorkflowId
		return nil

	case pb.RepoEvent_EV_FSO_REPO_MOVED:
		repo.repoMoveWorkflowId = uuid.Nil
		return nil

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		x := pbevents.FromPbMust(*ev).(*pbevents.EvShadowRepoMoveStarted)
		repo.moveShadowWorkflowId = x.WorkflowId
		return nil

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED:
		repo.moveShadowWorkflowId = uuid.Nil
		return nil

	// Legacy events of the preliminary repo-freeze implementation.
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED:
		return nil
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED:
		return nil

	default: // Ignore other.
		return nil
	}
}

func (a *watchRepoActivity) processView(
	ctx context.Context,
	repo watchRepoView,
) (bool, error) {
	if repo.hasError {
		a.state.hasError = true
	}

	// If there is something pending related to a workflow, handle it and
	// do nothing else.  Assume that the repo was fully initialized without
	// error before the workflow started.

	// For the move-repo workflow acquire part, the repo will be activated
	// in `watchRepoEvent()` on `RepoEvent_EV_FSO_REPO_MOVED` after the
	// workflow acquire part completes.
	//
	// If the activity starts with the move-repo workflow acquire part, run
	// it if the start event has been observed.  Otherwise continue to
	// `watchRepoEvent()` and handle the start event there.
	if a.opts.moveRepoWorkflowAcquire != uuid.Nil {
		if repo.repoMoveWorkflowId == a.opts.moveRepoWorkflowAcquire {
			return a.doRunMoveRepoAcquireWorkflowContinue(
				ctx, repo.repoMoveWorkflowId,
			)
		}
		return a.doContinue()
	}
	// This activity does not handle the move-repo workflow acquire part;
	// so it handles the release part.  If there is an active workflow, run
	// the release part and quit after the workflow completes.
	if repo.repoMoveWorkflowId != uuid.Nil {
		return a.doRunMoveRepoReleaseWorkflowQuit(
			ctx, repo.repoMoveWorkflowId,
		)
	}

	if repo.moveShadowWorkflowId != uuid.Nil {
		return a.doRunMoveShadowWorkflowContinue(
			ctx, repo.moveShadowWorkflowId,
		)
	}

	// If there is a stored error, do not retry init operations.  But
	// enable the repo if the shadow exists, because the repo may be useful
	// despite the stored error.
	pendingInitShadow := repo.wantsShadow && !repo.hasShadow
	if repo.hasError {
		a.state.pendingInitShadow = pendingInitShadow
		a.state.pendingEnableGitlab = repo.pendingEnableGitlab
		if repo.hasShadow {
			return a.doEnableRepoContinue(ctx)
		}
		return a.doContinue()
	}

	// If there is a pending init operation, run it and defer enabling the
	// repo to `watchRepoEvent()`.  If init succeeds, it will cause
	// `RepoEvent_EV_FSO_SHADOW_REPO_CREATED` and/or
	// `RepoEvent_EV_FSO_GIT_REPO_CREATED`, which are triggers to enable
	// the repo in `watchRepoEvent()`.
	if pendingInitShadow || repo.pendingEnableGitlab {
		return a.doInitShadowEnableGitlabContinue(
			ctx, pendingInitShadow, repo.pendingEnableGitlab,
		)
	}

	// If there is no shadow, wait for more events.
	if !repo.hasShadow {
		return a.doContinue()
	}

	// The repo must be enabled before some pending operations can be
	// executed.
	done, err := a.doEnableRepoContinue(ctx)
	if err != nil {
		return done, err
	}

	// XXX Pending operations that require an enabled repo would be added
	// here.

	return a.doContinue()
}

func (a *watchRepoActivity) watchRepoEvent(
	ctx context.Context, vid ulid.I, ev *pb.RepoEvent,
) (bool, error) {
	switch ev.Event {
	case pb.RepoEvent_EV_FSO_REPO_INIT_STARTED:
		if a.state.hasError {
			return a.doContinue()
		}
		return a.doInitShadowContinue(ctx)

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_CREATED:
		return a.doEnableRepoContinue(ctx)

	case pb.RepoEvent_EV_FSO_ENABLE_GITLAB_ACCEPTED:
		return a.doEnableGitlabContinue(ctx)

	case pb.RepoEvent_EV_FSO_GIT_REPO_CREATED:
		return a.doEnableRepoContinue(ctx)

	case pb.RepoEvent_EV_FSO_REPO_ERROR_SET:
		a.state.hasError = true
		return a.doContinue()

	case pb.RepoEvent_EV_FSO_REPO_ERROR_CLEARED:
		a.state.hasError = false
		return a.doInitShadowEnableGitlabContinue(
			ctx,
			a.state.pendingInitShadow,
			a.state.pendingEnableGitlab,
		)

	// Run the workflow release part and quit the activity.
	// `RegistryObserver` starts a new activity for the acquire part of the
	// workflow.
	//
	// An activity that starts with acquire may have raced to here without
	// observing the start event during load.  If so, `workflowId` equals
	// the expected acquire workflow.  If it is unequal, run the release
	// part, because an activity may start by acquiring the repo and later
	// release it.
	case pb.RepoEvent_EV_FSO_REPO_MOVE_STARTED:
		x := pbevents.FromPbMust(*ev).(*pbevents.EvRepoMoveStarted)
		workflowId := x.WorkflowId
		if workflowId == a.opts.moveRepoWorkflowAcquire {
			return a.doRunMoveRepoAcquireWorkflowContinue(
				ctx, workflowId,
			)
		}
		return a.doRunMoveRepoReleaseWorkflowQuit(ctx, workflowId)

	case pb.RepoEvent_EV_FSO_REPO_MOVED:
		return a.doEnableRepoContinue(ctx)

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
		x := pbevents.FromPbMust(*ev).(*pbevents.EvShadowRepoMoveStarted)
		workflowId := x.WorkflowId
		return a.doRunMoveShadowWorkflowContinue(
			ctx, workflowId,
		)

	case pb.RepoEvent_EV_FSO_SHADOW_REPO_MOVED:
		return a.doEnableRepoContinue(ctx)

	// Legacy events of the preliminary repo-freeze implementation.
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_STARTED:
		return a.doContinue()
	case pb.RepoEvent_EV_FSO_FREEZE_REPO_COMPLETED:
		return a.doContinue()
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_STARTED:
		return a.doContinue()
	case pb.RepoEvent_EV_FSO_UNFREEZE_REPO_COMPLETED:
		return a.doContinue()

	default:
		return a.doContinue()
	}
}

func (a *watchRepoActivity) doInitShadowContinue(
	ctx context.Context,
) (bool, error) {
	return a.doInitShadowEnableGitlabContinue(ctx, true, false)
}

func (a *watchRepoActivity) doEnableGitlabContinue(
	ctx context.Context,
) (bool, error) {
	return a.doInitShadowEnableGitlabContinue(ctx, false, true)
}

func (a *watchRepoActivity) doInitShadowEnableGitlabContinue(
	ctx context.Context,
	initShadow bool,
	enableGitlab bool,
) (bool, error) {
	// Set but do not clear pending flags.  The bool args indicate which
	// operations to perform.  If an operation is excluded, any side
	// effects related to that operation must be skipped.
	if initShadow {
		a.state.pendingInitShadow = true
	}
	if enableGitlab {
		a.state.pendingEnableGitlab = true
	}

	// Wait for upstream activity before causing side effects.
	if err := a.depWait(ctx); err != nil {
		return a.doRetry(ctx, err)
	}

	if initShadow {
		if _, err := a.initer.InitRepo(ctx, a.repoId); err != nil {
			return a.doHandleErrorContinueOrRetry(ctx, err)
		}
		a.state.pendingInitShadow = false
	}

	if enableGitlab {
		if _, err := a.initer.EnableGitlab(ctx, a.repoId); err != nil {
			return a.doHandleErrorContinueOrRetry(ctx, err)
		}
		a.state.pendingEnableGitlab = false
	}

	return a.doContinue()
}

func (a *watchRepoActivity) doEnableRepoContinue(
	ctx context.Context,
) (bool, error) {
	// Wait for upstream activity before causing side effects.
	if err := a.depWait(ctx); err != nil {
		return a.doRetry(ctx, err)
	}
	inf, err := a.initer.GetRepo(ctx, a.repoId)
	if err != nil {
		return a.doHandleErrorContinueOrRetry(ctx, err)
	}
	if err := a.proc.EnableRepo4(ctx, inf); err != nil {
		return a.doHandleErrorContinueOrRetry(ctx, err)
	}
	return a.doContinue()
}

// Retry all errors, assuming that an admin is monitoring the workflow
// execution and would fix issues right away.
func (a *watchRepoActivity) doRunMoveRepoReleaseWorkflowQuit(
	ctx context.Context,
	workflowId uuid.I,
) (bool, error) {
	// Wait for upstream activity before causing side effects.
	if err := a.depWait(ctx); err != nil {
		return a.doRetry(ctx, err)
	}

	if err := a.proc.DisableRepo4(ctx, a.repoId); err != nil {
		return a.doRetry(ctx, err)
	}

	// Run workflow and wait for completion.
	done := make(chan struct{})
	if err := a.workflowEngine.StartRepoWorkflowActivity(
		a.repoId, workflowId,
		&moveRepoWorkflowReleaseActivity{
			chainedRepoActivity: chainedRepoActivity{
				lg: a.lg,
				chain: DepChainNode{
					Dep:  nil,
					Done: done,
				},
				repoId:     a.repoId,
				errHandler: nil,
			},
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
		},
	); err != nil {
		return a.doRetry(ctx, err)
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-done:
	}

	// Quit the activity after releasing the repo.
	return a.doDepWaitQuit(ctx)
}

// Retry all errors, assuming that an admin is monitoring the workflow
// execution and would fix issues right away.
func (a *watchRepoActivity) doRunMoveRepoAcquireWorkflowContinue(
	ctx context.Context,
	workflowId uuid.I,
) (bool, error) {
	// Wait for upstream activity before causing side effects.
	if err := a.depWait(ctx); err != nil {
		return a.doRetry(ctx, err)
	}

	// Run workflow and wait for completion.
	done := make(chan struct{})
	if err := a.workflowEngine.StartRepoWorkflowActivity(
		a.repoId, workflowId,
		&moveRepoWorkflowAcquireActivity{
			chainedRepoActivity: chainedRepoActivity{
				lg: a.lg,
				chain: DepChainNode{
					Dep:  nil,
					Done: done,
				},
				repoId:     a.repoId,
				errHandler: nil,
			},
			initer:      a.initer,
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
		},
	); err != nil {
		return a.doRetry(ctx, err)
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-done:
	}

	// Quit the activity after releasing the repo.
	return a.doContinue()
}

// Retry all errors, assuming that an admin is monitoring the workflow
// execution and would fix issues right away.
func (a *watchRepoActivity) doRunMoveShadowWorkflowContinue(
	ctx context.Context,
	workflowId uuid.I,
) (bool, error) {
	// Wait for upstream activity before causing side effects.
	if err := a.depWait(ctx); err != nil {
		return a.doRetry(ctx, err)
	}

	if err := a.proc.DisableRepo4(ctx, a.repoId); err != nil {
		return a.doRetry(ctx, err)
	}

	// Run workflow and wait for completion.
	done := make(chan struct{})
	if err := a.workflowEngine.StartRepoWorkflowActivity(
		a.repoId, workflowId,
		&moveShadowWorkflowActivity{
			chainedRepoActivity: chainedRepoActivity{
				lg: a.lg,
				chain: DepChainNode{
					Dep:  nil,
					Done: done,
				},
				repoId:     a.repoId,
				errHandler: nil,
			},
			conn:        a.conn,
			sysRPCCreds: a.sysRPCCreds,
		},
	); err != nil {
		return a.doRetry(ctx, err)
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-done:
	}

	// Quit the activity after releasing the repo.
	return a.doContinue()
}
