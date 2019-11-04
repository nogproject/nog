package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/workflows/archiverepowf"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/pkg/errorsx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type archiveRepoWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	registry    string
	done        chan<- struct{}
	view        archiveRepoWorkflowView
	tail        ulid.I
}

type archiveRepoWorkflowView struct {
	workflowId       uuid.I
	vid              ulid.I
	scode            archiverepowf.StateCode
	registryName     string
	startRegistryVid ulid.I
	repoId           uuid.I
	startRepoVid     ulid.I
	filesCode        int32
	filesMessage     string
	tarPath          string
}

func (a *archiveRepoWorkflowActivity) ProcessRegistryWorkflowEvents(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.EphemeralRegistry_RegistryWorkflowEventsClient,
) (ulid.I, error) {
	if tail == ulid.Nil {
		view := archiveRepoWorkflowView{
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

func (a *archiveRepoWorkflowActivity) WatchWorkflowEvent(
	ctx context.Context, vid ulid.I, ev wfevents.WorkflowEvent,
) (bool, error) {
	if err := a.view.LoadWorkflowEvent(vid, ev); err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) WillBlock(
	ctx context.Context,
) (bool, error) {
	// Do not call a successful `processView()` again without new event.
	// This is not only an optimization but also necessary to handle
	// successful non-idempotent operations correctly.  For example,
	// `RegistryBeginArchiveRepo()` with `RegistryVid` must be called only
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

func (view *archiveRepoWorkflowView) LoadWorkflowEvent(
	vid ulid.I, ev wfevents.WorkflowEvent,
) error {
	view.vid = vid

	switch x := ev.(type) {
	case *wfevents.EvArchiveRepoStarted:
		view.scode = archiverepowf.StateInitialized
		view.registryName = x.RegistryName
		view.startRegistryVid = x.StartRegistryVid
		view.repoId = x.RepoId
		view.startRepoVid = x.StartRepoVid
		return nil

	case *wfevents.EvArchiveRepoFilesStarted:
		view.scode = archiverepowf.StateFiles
		return nil

	case *wfevents.EvArchiveRepoTarttCompleted:
		view.scode = archiverepowf.StateTarttCompleted
		view.tarPath = x.TarPath
		return nil

	case *wfevents.EvArchiveRepoSwapStarted:
		view.scode = archiverepowf.StateSwapStarted
		return nil

	case *wfevents.EvArchiveRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = archiverepowf.StateFilesCompleted
		} else {
			view.scode = archiverepowf.StateFilesFailed
			view.filesCode = x.StatusCode
			view.filesMessage = x.StatusMessage
		}
		return nil

	case *wfevents.EvArchiveRepoFilesCommitted:
		view.scode = archiverepowf.StateFilesEnded
		return nil

	case *wfevents.EvArchiveRepoGcCompleted:
		view.scode = archiverepowf.StateGcCompleted
		return nil

	case *wfevents.EvArchiveRepoCompleted:
		if x.StatusCode == 0 {
			view.scode = archiverepowf.StateCompleted
		} else {
			view.scode = archiverepowf.StateFailed
		}
		return nil

	case *wfevents.EvArchiveRepoCommitted:
		view.scode = archiverepowf.StateTerminated
		return nil

	default:
		return ErrUnknownEvent
	}
}

func (a *archiveRepoWorkflowActivity) processView(
	ctx context.Context,
	view archiveRepoWorkflowView,
) (bool, error) {
	switch view.scode {
	case archiverepowf.StateUninitialized:
		return a.doContinue()

	case archiverepowf.StateInitialized:
		return a.doBeginArchiveThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.startRegistryVid,
			view.repoId,
			view.startRepoVid,
		)

	// Wait for nogfsostad to `CommitArchiveRepoFiles()` or
	// `AbortArchiveRepoFiles()`.
	case archiverepowf.StateFiles:
		return a.doContinue()
	case archiverepowf.StateTarttCompleted:
		return a.doContinue()
	case archiverepowf.StateSwapStarted:
		return a.doContinue()

	case archiverepowf.StateFilesCompleted:
		return a.doCommitFilesThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
			view.tarPath,
		)

	case archiverepowf.StateFilesFailed:
		return a.doAbortFilesThenContinue(
			ctx,
			view.workflowId,
			view.vid,
			view.registryName,
			view.repoId,
			view.filesCode,
			view.filesMessage,
		)

	case archiverepowf.StateFilesEnded:
		// Wait for nogfsostad to `CommitArchiveRepoGc()`.
		return a.doContinue()

	case archiverepowf.StateGcCompleted:
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

	case archiverepowf.StateCompleted:
		return a.doQuit()

	case archiverepowf.StateFailed:
		return a.doQuit()

	case archiverepowf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *archiveRepoWorkflowActivity) doBeginArchiveThenContinue(
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
			"registry error: cannot archive repo",
			"registry error: workflow conflict",
			"version conflict",
		})
	}

	isFatalReposError := func(err error) bool {
		return errorContainsAny(err, []string{
			"unknown repo",
			"workflow conflict",
			"cannot proceed due to repo error",
			"storage workflow conflict",
			"no tartt repo",
		})
	}

	{
		c := pb.NewRegistryArchiveRepoClient(a.conn)
		i := &pb.RegistryBeginArchiveRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRegistryVid != ulid.Nil {
			i.RegistryVid = startRegistryVid[:]
		}
		_, err := c.RegistryBeginArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalRegistryError):
			return a.doAbortThenQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REGISTRY_BEGIN_ARCHIVE_REPO_FAILED),
				"registry begin failed",
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewReposArchiveRepoClient(a.conn)
		i := &pb.ReposBeginArchiveRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		if startRepoVid != ulid.Nil {
			i.RepoVid = startRepoVid[:]
		}
		_, err := c.ReposBeginArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isFatalReposError):
			return a.doAbortThenQuit(
				ctx, workflowId, registryName, repoId,
				int32(pb.StatusCode_SC_REPOS_BEGIN_ARCHIVE_REPO_FAILED),
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
		c := pb.NewArchiveRepoClient(a.conn)
		i := &pb.BeginArchiveRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
			AclPolicy:   aclPolicy,
		}
		_, err := c.BeginArchiveRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) doCommitFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	repoId uuid.I,
	tarPath string,
) (bool, error) {
	{
		c := pb.NewReposArchiveRepoClient(a.conn)
		i := &pb.ReposCommitArchiveRepoI{
			Repo:     repoId[:],
			Workflow: workflowId[:],
			TarPath:  tarPath,
		}
		_, err := c.ReposCommitArchiveRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryArchiveRepoClient(a.conn)
		i := &pb.RegistryCommitArchiveRepoI{
			Registry: registryName,
			Repo:     repoId[:],
			Workflow: workflowId[:],
		}
		_, err := c.RegistryCommitArchiveRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewArchiveRepoClient(a.conn)
		i := &pb.EndArchiveRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.EndArchiveRepoFiles(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doContinue()
}

func (a *archiveRepoWorkflowActivity) doCommitThenQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
	registryName string,
	repoId uuid.I,
) (bool, error) {
	{
		c := pb.NewArchiveRepoClient(a.conn)
		i := &pb.CommitArchiveRepoI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.CommitArchiveRepo(ctx, i, a.sysRPCCreds)
		if err != nil {
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

// Similar to `doAbortThenQuit()` but continue with GC.
func (a *archiveRepoWorkflowActivity) doAbortFilesThenContinue(
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
		c := pb.NewReposArchiveRepoClient(a.conn)
		i := &pb.ReposAbortArchiveRepoI{
			Repo:          repoId[:],
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.ReposAbortArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredReposError):
			a.lg.Infow(
				"Ignored ReposAbortArchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryArchiveRepoClient(a.conn)
		i := &pb.RegistryAbortArchiveRepoI{
			Registry:   registryName,
			Repo:       repoId[:],
			Workflow:   workflowId[:],
			StatusCode: statusCode,
		}
		_, err := c.RegistryAbortArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredRegistryError):
			a.lg.Infow(
				"Ignored RegistryAbortArchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewArchiveRepoClient(a.conn)
		i := &pb.EndArchiveRepoFilesI{
			Workflow:    workflowId[:],
			WorkflowVid: vid[:],
		}
		_, err := c.EndArchiveRepoFiles(ctx, i, a.sysRPCCreds)
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
func (a *archiveRepoWorkflowActivity) doAbortThenQuit(
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
			"archive-repo workflow: already terminated",
		})
	}

	{
		c := pb.NewReposArchiveRepoClient(a.conn)
		i := &pb.ReposAbortArchiveRepoI{
			Repo:          repoId[:],
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.ReposAbortArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredReposError):
			a.lg.Infow(
				"Ignored ReposAbortArchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewRegistryArchiveRepoClient(a.conn)
		i := &pb.RegistryAbortArchiveRepoI{
			Registry:   registryName,
			Repo:       repoId[:],
			Workflow:   workflowId[:],
			StatusCode: statusCode,
		}
		_, err := c.RegistryAbortArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredRegistryError):
			a.lg.Infow(
				"Ignored RegistryAbortArchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	{
		c := pb.NewArchiveRepoClient(a.conn)
		i := &pb.AbortArchiveRepoI{
			Workflow:      workflowId[:],
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		}
		_, err := c.AbortArchiveRepo(ctx, i, a.sysRPCCreds)
		switch {
		case errorsx.IsPred(err, isIgnoredWorkflowError):
			a.lg.Infow(
				"Ignored AbortArchiveRepo() error.",
				"err", err,
			)
		case err != nil:
			return a.doRetry(err)
		}
	}

	return a.doQuit()
}

func (a *archiveRepoWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *archiveRepoWorkflowActivity) doQuit() (bool, error) {
	if a.done != nil {
		close(a.done)
	}
	return true, nil
}

func (a *archiveRepoWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
