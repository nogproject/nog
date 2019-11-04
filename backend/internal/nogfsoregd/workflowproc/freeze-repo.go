package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/pkg/errorsx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type freezeRepoWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	registry    string
	done        chan<- struct{}
	view        freezeRepoWorkflowView
	tail        ulid.I
}

type freezeRepoWorkflowView struct {
	workflowId       uuid.I
	vid              ulid.I
	scode            freezerepowf.StateCode
	registryName     string
	startRegistryVid ulid.I
	repoId           uuid.I
	startRepoVid     ulid.I
	filesCode        int32
	filesMessage     string
}

func (a *freezeRepoWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := freezeRepoWorkflowView{
			workflowId: workflowId,
		}
		if err := wfstreams.LoadRegistryWorkflowEventsNoBlock(
			stream, &view,
		); err != nil {
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		}

		done, err := a.processView(ctx, view)
		switch {
		case err != nil:
			// Return `ulid.Nil` to restart from epoch.
			return ulid.Nil, err
		case done:
			return view.vid, nil
		}

		tail = view.vid
		a.view = view
		a.tail = view.vid
	}

	return wfstreams.WatchRegistryWorkflowEvents(
		ctx, tail, stream, a, a,
	)
}

func (a *freezeRepoWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *freezeRepoWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	// Do not call a successful `processView()` again without new event.
	// This is not only an optimization but also necessary to handle
	// successful non-idempotent operations correctly.  For example,
	// `RegistryBeginFreezeRepo()` with `RegistryVid` must be called only
	// once.
	//
	// XXX Could this logic be moved to the caller of `WillBlock()`?
	if a.view.vid == a.tail {
		return a.doContinue()
	}
	done, err := a.processView(ctx, a.view)
	if err == nil {
		a.tail = a.view.vid
	}
	return done, err
}

func (view *freezeRepoWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvFreezeRepoStarted2:
		view.scode = freezerepowf.StateInitialized
		view.registryName = x.RegistryName
		view.startRegistryVid = x.StartRegistryVid
		view.repoId = x.RepoId
		view.startRepoVid = x.StartRepoVid
		return nil

	case *wfevents.EvFreezeRepoFilesStarted:
		view.scode = freezerepowf.StateFiles
		return nil

	case *wfevents.EvFreezeRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = freezerepowf.StateFilesCompleted
		} else {
			view.scode = freezerepowf.StateFilesFailed
			view.filesCode = x.StatusCode
			view.filesMessage = x.StatusMessage
		}
		return nil

	case *wfevents.EvFreezeRepoCompleted2:
		if x.StatusCode == 0 {
			view.scode = freezerepowf.StateCompleted
		} else {
			view.scode = freezerepowf.StateFailed
		}
		return nil

	case *wfevents.EvFreezeRepoCommitted:
		view.scode = freezerepowf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *freezeRepoWorkflowActivity) processView(
	ctx context.Context,
	view freezeRepoWorkflowView,
) (bool, error) {
	switch view.scode {
	case freezerepowf.StateUninitialized:
		return a.doContinue()

	case freezerepowf.StateInitialized:
		return a.doBeginFreezeAndContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.startRegistryVid,
			view.repoId,
			view.startRepoVid,
		)

	case freezerepowf.StateFiles:
		// Wait for nogfsostad to `CommitFreezeRepoFiles()` or
		// `AbortFreezeRepoFiles()`.
		return a.doContinue()

	case freezerepowf.StateFilesCompleted:
		return a.doCommitAndQuit(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
		)

	case freezerepowf.StateFilesFailed:
		return a.doAbortAndQuit(
			ctx,
			view.workflowId,
			view.registryName,
			view.repoId,
			view.filesCode,
			view.filesMessage,
		)

	case freezerepowf.StateCompleted:
		return a.doQuit()

	case freezerepowf.StateFailed:
		return a.doQuit()

	case freezerepowf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *freezeRepoWorkflowActivity) doBeginFreezeAndContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	startRegistryVid ulid.I,
	repoId uuid.I,
	startRepoVid ulid.I,
) (bool, error) {
	isFatalRegistryError := func(err error) bool {
		return errorContainsAny(err, []string{
			"registry error: cannot freeze repo",
			"registry error: workflow conflict",
			"version conflict",
		})
	}

	isFatalReposError := func(err error) bool {
		return errorContainsAny(err, []string{
			"version conflict",
		})
	}

	{
		c := pb.NewRegistryFreezeRepoClient(a.conn)
		i := &pb.RegistryBeginFreezeRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRegistryVid != ulid.Nil {
			i.RegistryVid = startRegistryVid[:]
		}
		_, err := c.RegistryBeginFreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalRegistryError):
			return a.doAbortAndQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REGISTRY_BEGIN_FREEZE_REPO_FAILED),
				"registry begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewReposFreezeRepoClient(a.conn)
		i := &pb.ReposBeginFreezeRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRepoVid != ulid.Nil {
			i.RepoVid = startRepoVid[:]
		}
		_, err := c.ReposBeginFreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalReposError):
			return a.doAbortAndQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REPOS_BEGIN_FREEZE_REPO_FAILED),
				"registry begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewFreezeRepoClient(a.conn)
		i := &pb.BeginFreezeRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.BeginFreezeRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

func (a *freezeRepoWorkflowActivity) doCommitAndQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	repoId uuid.I,
) (bool, error) {
	{
		c := pb.NewReposFreezeRepoClient(a.conn)
		i := &pb.ReposCommitFreezeRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.ReposCommitFreezeRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryFreezeRepoClient(a.conn)
		i := &pb.RegistryCommitFreezeRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.RegistryCommitFreezeRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewFreezeRepoClient(a.conn)
		i := &pb.CommitFreezeRepoI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.CommitFreezeRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

// `doAbortAndQuit()` cleans up all aggregates that may have pending
// operations: the registry, the repo, and the workflow itself.
//
// `doAbortAndQuit()` avoids assumptions about the aggregate state.  If a
// BeginX() fails, an operation may or may not be pending, depending on where
// the error happened, for example the begin event may have been stored
// although the reply was lost due to a restart.  So we do not try to infer
// whether AbortX() is required.  Instead, we unconditionally call AbortX() and
// analyze its result.  AbortX() is considered done if it succeeds or if the
// error indicates that there is no pending operation, i.e. the BeginX() had no
// effect.  We call AbortX() without version control, because we only care
// about the final state.
func (a *freezeRepoWorkflowActivity) doAbortAndQuit(
	ctx context.Context,
	workflowId uuid.I,
	registryName string,
	repoId uuid.I,
	statusCode int32,
	statusMessage string,
) (bool, error) {
	isIgnoredRegistryError := func(err error) bool {
		return errorContainsAny(err, []string{
			"registry error: workflow conflict",
		})
	}
	isIgnoredReposError := func(err error) bool {
		return errorContainsAny(err, []string{
			"repos error: storage workflow conflict",
		})
	}
	isIgnoredWorkflowError := func(err error) bool {
		return errorContainsAny(err, []string{
			"freeze-repo workflow: already terminated",
		})
	}

	{
		c := pb.NewReposFreezeRepoClient(a.conn)
		i := &pb.ReposAbortFreezeRepoI{
			Repo:          repoId[:],
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.ReposAbortFreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredReposError):
			a.lg.Infow(
				"Ignored ReposAbortFreezeRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryFreezeRepoClient(a.conn)
		i := &pb.RegistryAbortFreezeRepoI{
			Registry:   registryName,
			Repo:       repoId[:],
			Workflow:   workflowId[:],
			StatusCode: statusCode,
		}
		_, err := c.RegistryAbortFreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredRegistryError):
			a.lg.Infow(
				"Ignored RegistryAbortFreezeRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewFreezeRepoClient(a.conn)
		i := &pb.AbortFreezeRepoI{
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.AbortFreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredWorkflowError):
			a.lg.Infow(
				"Ignored AbortFreezeRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

func (a *freezeRepoWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *freezeRepoWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *freezeRepoWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
