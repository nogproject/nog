package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/pkg/errorsx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type unfreezeRepoWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	registry    string
	done        chan<- struct{}
	view        unfreezeRepoWorkflowView
	tail        ulid.I
}

type unfreezeRepoWorkflowView struct {
	workflowId       uuid.I
	vid              ulid.I
	scode            unfreezerepowf.StateCode
	registryName     string
	startRegistryVid ulid.I
	repoId           uuid.I
	startRepoVid     ulid.I
	filesCode        int32
	filesMessage     string
}

func (a *unfreezeRepoWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := unfreezeRepoWorkflowView{
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

func (a *unfreezeRepoWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *unfreezeRepoWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	// Do not call a successful `processView()` again without new event.
	// This is not only an optimization but also necessary to handle
	// successful non-idempotent operations correctly.  For example,
	// `RegistryBeginUnfreezeRepo()` with `RegistryVid` must be called only
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

func (view *unfreezeRepoWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvUnfreezeRepoStarted2:
		view.scode = unfreezerepowf.StateInitialized
		view.registryName = x.RegistryName
		view.startRegistryVid = x.StartRegistryVid
		view.repoId = x.RepoId
		view.startRepoVid = x.StartRepoVid
		return nil

	case *wfevents.EvUnfreezeRepoFilesStarted:
		view.scode = unfreezerepowf.StateFiles
		return nil

	case *wfevents.EvUnfreezeRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = unfreezerepowf.StateFilesCompleted
		} else {
			view.scode = unfreezerepowf.StateFilesFailed
			view.filesCode = x.StatusCode
			view.filesMessage = x.StatusMessage
		}
		return nil

	case *wfevents.EvUnfreezeRepoCompleted2:
		if x.StatusCode == 0 {
			view.scode = unfreezerepowf.StateCompleted
		} else {
			view.scode = unfreezerepowf.StateFailed
		}
		return nil

	case *wfevents.EvUnfreezeRepoCommitted:
		view.scode = unfreezerepowf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *unfreezeRepoWorkflowActivity) processView(
	ctx context.Context,
	view unfreezeRepoWorkflowView,
) (bool, error) {
	switch view.scode {
	case unfreezerepowf.StateUninitialized:
		return a.doContinue()

	case unfreezerepowf.StateInitialized:
		return a.doBeginUnfreezeAndContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.startRegistryVid,
			view.repoId,
			view.startRepoVid,
		)

	case unfreezerepowf.StateFiles:
		// Wait for nogfsostad to `CommitUnfreezeRepoFiles()` or
		// `AbortUnfreezeRepoFiles()`.
		return a.doContinue()

	case unfreezerepowf.StateFilesCompleted:
		return a.doCommitAndQuit(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
		)

	case unfreezerepowf.StateFilesFailed:
		return a.doAbortAndQuit(
			ctx,
			view.workflowId,
			view.registryName,
			view.repoId,
			view.filesCode,
			view.filesMessage,
		)

	case unfreezerepowf.StateCompleted:
		return a.doQuit()

	case unfreezerepowf.StateFailed:
		return a.doQuit()

	case unfreezerepowf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *unfreezeRepoWorkflowActivity) doBeginUnfreezeAndContinue(
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
			"registry error: cannot unfreeze repo",
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
		c := pb.NewRegistryUnfreezeRepoClient(a.conn)
		i := &pb.RegistryBeginUnfreezeRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRegistryVid != ulid.Nil {
			i.RegistryVid = startRegistryVid[:]
		}
		_, err := c.RegistryBeginUnfreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalRegistryError):
			return a.doAbortAndQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REGISTRY_BEGIN_UNFREEZE_REPO_FAILED),
				"registry begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewReposUnfreezeRepoClient(a.conn)
		i := &pb.ReposBeginUnfreezeRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRepoVid != ulid.Nil {
			i.RepoVid = startRepoVid[:]
		}
		_, err := c.ReposBeginUnfreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalReposError):
			return a.doAbortAndQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REPOS_BEGIN_UNFREEZE_REPO_FAILED),
				"registry begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewUnfreezeRepoClient(a.conn)
		i := &pb.BeginUnfreezeRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.BeginUnfreezeRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

func (a *unfreezeRepoWorkflowActivity) doCommitAndQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	repoId uuid.I,
) (bool, error) {
	{
		c := pb.NewReposUnfreezeRepoClient(a.conn)
		i := &pb.ReposCommitUnfreezeRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.ReposCommitUnfreezeRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryUnfreezeRepoClient(a.conn)
		i := &pb.RegistryCommitUnfreezeRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.RegistryCommitUnfreezeRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewUnfreezeRepoClient(a.conn)
		i := &pb.CommitUnfreezeRepoI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.CommitUnfreezeRepo(ctx, i, a.sysRPCCreds)
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
func (a *unfreezeRepoWorkflowActivity) doAbortAndQuit(
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
			"unfreeze-repo workflow: already terminated",
		})
	}

	{
		c := pb.NewReposUnfreezeRepoClient(a.conn)
		i := &pb.ReposAbortUnfreezeRepoI{
			Repo:          repoId[:],
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.ReposAbortUnfreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredReposError):
			a.lg.Infow(
				"Ignored ReposAbortUnfreezeRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryUnfreezeRepoClient(a.conn)
		i := &pb.RegistryAbortUnfreezeRepoI{
			Registry:   registryName,
			Repo:       repoId[:],
			Workflow:   workflowId[:],
			StatusCode: statusCode,
		}
		_, err := c.RegistryAbortUnfreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredRegistryError):
			a.lg.Infow(
				"Ignored RegistryAbortUnfreezeRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewUnfreezeRepoClient(a.conn)
		i := &pb.AbortUnfreezeRepoI{
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.AbortUnfreezeRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredWorkflowError):
			a.lg.Infow(
				"Ignored AbortUnfreezeRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

func (a *unfreezeRepoWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *unfreezeRepoWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *unfreezeRepoWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
