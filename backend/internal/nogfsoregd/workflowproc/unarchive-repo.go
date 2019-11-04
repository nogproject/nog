package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/pkg/errorsx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type unarchiveRepoWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	registry    string
	done        chan<- struct{}
	view        unarchiveRepoWorkflowView
	tail        ulid.I
}

type unarchiveRepoWorkflowView struct {
	workflowId       uuid.I
	vid              ulid.I
	scode            unarchiverepowf.StateCode
	registryName     string
	startRegistryVid ulid.I
	repoId           uuid.I
	startRepoVid     ulid.I
	tarttCode        int32
	tarttMessage     string
	filesCode        int32
	filesMessage     string
}

func (a *unarchiveRepoWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := unarchiveRepoWorkflowView{
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

func (a *unarchiveRepoWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *unarchiveRepoWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	// Do not call a successful `processView()` again without new event.
	// This is not only an optimization but also necessary to handle
	// successful non-idempotent operations correctly.  For example,
	// `RegistryBeginUnarchiveRepo()` with `RegistryVid` must be called only
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

func (view *unarchiveRepoWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvUnarchiveRepoStarted:
		view.scode = unarchiverepowf.StateInitialized
		view.registryName = x.RegistryName
		view.startRegistryVid = x.StartRegistryVid
		view.repoId = x.RepoId
		view.startRepoVid = x.StartRepoVid
		return nil

	case *wfevents.EvUnarchiveRepoFilesStarted:
		view.scode = unarchiverepowf.StateFiles
		return nil

	case *wfevents.EvUnarchiveRepoTarttStarted:
		view.scode = unarchiverepowf.StateTartt
		return nil

	case *wfevents.EvUnarchiveRepoTarttCompleted:
		if x.StatusCode == 0 {
			view.scode = unarchiverepowf.StateTarttCompleted
		} else {
			view.scode = unarchiverepowf.StateTarttFailed
			view.tarttCode = x.StatusCode
			view.tarttMessage = x.StatusMessage
		}
		return nil

	case *wfevents.EvUnarchiveRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = unarchiverepowf.StateFilesCompleted
		} else {
			view.scode = unarchiverepowf.StateFilesFailed
			view.filesCode = x.StatusCode
			view.filesMessage = x.StatusMessage
		}
		return nil

	case *wfevents.EvUnarchiveRepoFilesCommitted:
		view.scode = unarchiverepowf.StateFilesEnded
		return nil

	case *wfevents.EvUnarchiveRepoGcCompleted:
		view.scode = unarchiverepowf.StateGcCompleted
		return nil

	case *wfevents.EvUnarchiveRepoCompleted:
		if x.StatusCode == 0 {
			view.scode = unarchiverepowf.StateCompleted
		} else {
			view.scode = unarchiverepowf.StateFailed
		}
		return nil

	case *wfevents.EvUnarchiveRepoCommitted:
		view.scode = unarchiverepowf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *unarchiveRepoWorkflowActivity) processView(
	ctx context.Context,
	view unarchiveRepoWorkflowView,
) (bool, error) {
	switch view.scode {
	case unarchiverepowf.StateUninitialized:
		return a.doContinue()

	case unarchiverepowf.StateInitialized:
		return a.doBeginUnarchiveThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.startRegistryVid,
			view.repoId,
			view.startRepoVid,
		)

	// Wait for nogfsostad to `CommitUnarchiveRepoFiles()` or
	// `AbortUnarchiveRepoFiles()`.
	case unarchiverepowf.StateFiles:
		return a.doContinue()
	case unarchiverepowf.StateTartt:
		return a.doContinue()
	case unarchiverepowf.StateTarttCompleted:
		return a.doContinue()

	case unarchiverepowf.StateTarttFailed:
		return a.doAbortFilesThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
			view.tarttCode,
			view.tarttMessage,
		)

	case unarchiverepowf.StateFilesCompleted:
		return a.doCommitFilesThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
		)

	case unarchiverepowf.StateFilesFailed:
		return a.doAbortFilesThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
			view.filesCode,
			view.filesMessage,
		)

	case unarchiverepowf.StateFilesEnded:
		// Wait for nogfsostad to `CommitUnarchiveRepoGc()`.
		return a.doContinue()

	case unarchiverepowf.StateGcCompleted:
		if view.tarttCode != 0 {
			return a.doAbortThenQuit(
				ctx,
				view.workflowId,
				view.registryName,
				view.repoId,
				view.tarttCode,
				view.tarttMessage,
			)
		}
		if view.filesCode != 0 {
			return a.doAbortThenQuit(
				ctx,
				view.workflowId,
				view.registryName,
				view.repoId,
				view.filesCode,
				view.filesMessage,
			)
		}
		return a.doCommitThenQuit(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
		)

	case unarchiverepowf.StateCompleted:
		return a.doQuit()

	case unarchiverepowf.StateFailed:
		return a.doQuit()

	case unarchiverepowf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *unarchiveRepoWorkflowActivity) doBeginUnarchiveThenContinue(
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
			"registry error: cannot unarchive repo",
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
		c := pb.NewRegistryUnarchiveRepoClient(a.conn)
		i := &pb.RegistryBeginUnarchiveRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRegistryVid != ulid.Nil {
			i.RegistryVid = startRegistryVid[:]
		}
		_, err := c.RegistryBeginUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalRegistryError):
			return a.doAbortThenQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REGISTRY_BEGIN_UNARCHIVE_REPO_FAILED),
				"registry begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewReposUnarchiveRepoClient(a.conn)
		i := &pb.ReposBeginUnarchiveRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRepoVid != ulid.Nil {
			i.RepoVid = startRepoVid[:]
		}
		_, err := c.ReposBeginUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalReposError):
			return a.doAbortThenQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REPOS_BEGIN_UNARCHIVE_REPO_FAILED),
				"repo begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	var aclPolicy *pb.RepoAclPolicy
	{
		c := pb.NewRegistryClient(a.conn)
		i := &pb.GetRepoAclPolicyI{
			Registry: registryName,
			Repo:     repoId[:],
		}
		o, err := c.GetRepoAclPolicy(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
		aclPolicy = o.Policy
	}

	{
		c := pb.NewExecUnarchiveRepoClient(a.conn)
		i := &pb.BeginUnarchiveRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
			AclPolicy:   aclPolicy,
		}
		_, err := c.BeginUnarchiveRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

func (a *unarchiveRepoWorkflowActivity) doCommitFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	repoId uuid.I,
) (bool, error) {
	{
		c := pb.NewReposUnarchiveRepoClient(a.conn)
		i := &pb.ReposCommitUnarchiveRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.ReposCommitUnarchiveRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryUnarchiveRepoClient(a.conn)
		i := &pb.RegistryCommitUnarchiveRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.RegistryCommitUnarchiveRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewExecUnarchiveRepoClient(a.conn)
		i := &pb.EndUnarchiveRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.EndUnarchiveRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

func (a *unarchiveRepoWorkflowActivity) doCommitThenQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	repoId uuid.I,
) (bool, error) {
	{
		c := pb.NewExecUnarchiveRepoClient(a.conn)
		i := &pb.CommitUnarchiveRepoI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.CommitUnarchiveRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

// Similar to `doAbortThenQuit()` but continue with GC.
func (a *unarchiveRepoWorkflowActivity) doAbortFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
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

	{
		c := pb.NewReposUnarchiveRepoClient(a.conn)
		i := &pb.ReposAbortUnarchiveRepoI{
			Repo:          repoId[:],
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.ReposAbortUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredReposError):
			a.lg.Infow(
				"Ignored ReposAbortUnarchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryUnarchiveRepoClient(a.conn)
		i := &pb.RegistryAbortUnarchiveRepoI{
			Registry:   registryName,
			Repo:       repoId[:],
			Workflow:   workflowId[:],
			StatusCode: statusCode,
		}
		_, err := c.RegistryAbortUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredRegistryError):
			a.lg.Infow(
				"Ignored RegistryAbortUnarchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewExecUnarchiveRepoClient(a.conn)
		i := &pb.EndUnarchiveRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.EndUnarchiveRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

// `doAbortThenQuit()` cleans up all aggregates that may have pending
// operations: the registry, the repo, and the workflow itself.
//
// `doAbortThenQuit()` avoids assumptions about the aggregate state.  If a
// BeginX() fails, an operation may or may not be pending, depending on where
// the error happened, for example the begin event may have been stored
// although the reply was lost due to a restart.  So we do not try to infer
// whether AbortX() is required.  Instead, we unconditionally call AbortX() and
// analyze its result.  AbortX() is considered done if it succeeds or if the
// error indicates that there is no pending operation, i.e. BeginX() had no
// effect.  We call AbortX() without version control, because we only care
// about the final state.
func (a *unarchiveRepoWorkflowActivity) doAbortThenQuit(
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
			"unarchive-repo workflow: already terminated",
		})
	}

	{
		c := pb.NewReposUnarchiveRepoClient(a.conn)
		i := &pb.ReposAbortUnarchiveRepoI{
			Repo:          repoId[:],
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.ReposAbortUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredReposError):
			a.lg.Infow(
				"Ignored ReposAbortUnarchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryUnarchiveRepoClient(a.conn)
		i := &pb.RegistryAbortUnarchiveRepoI{
			Registry:   registryName,
			Repo:       repoId[:],
			Workflow:   workflowId[:],
			StatusCode: statusCode,
		}
		_, err := c.RegistryAbortUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredRegistryError):
			a.lg.Infow(
				"Ignored RegistryAbortUnarchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewExecUnarchiveRepoClient(a.conn)
		i := &pb.AbortUnarchiveRepoI{
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.AbortUnarchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredWorkflowError):
			a.lg.Infow(
				"Ignored AbortUnarchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

func (a *unarchiveRepoWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *unarchiveRepoWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *unarchiveRepoWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
