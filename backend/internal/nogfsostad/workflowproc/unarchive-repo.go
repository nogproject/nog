package workflowproc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/pkg/timex"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const ConfigMaxUnarchiveRepoRetries = 5

const ConfigUnarchiveRepoGCDelayDays = 7
const ConfigUnarchiveRepoGCDelay = ConfigUnarchiveRepoGCDelayDays * 24 * time.Hour

type unarchiveRepoWorkflowActivity struct {
	lg                 Logger
	conn               *grpc.ClientConn
	sysRPCCreds        grpc.CallOption
	done               chan<- struct{}
	repoProc           RepoProcessor
	aclPropagator      AclPropagator
	privs              UnarchiveRepoPrivileges
	unarchiveRepoSpool string
	view               unarchiveRepoWorkflowView
	tail               ulid.I
	nRetries           int
}

type unarchiveRepoWorkflowView struct {
	workflowId         uuid.I
	vid                ulid.I
	scode              unarchiverepowf.StateCode
	repoId             uuid.I
	startTime          time.Time
	workingDir         string
	authorName         string
	authorEmail        string
	aclPolicy          *pb.RepoAclPolicy
	filesCommittedTime time.Time
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
		view.repoId = x.RepoId
		view.startTime = ulid.Time(vid)
		view.authorName = x.AuthorName
		view.authorEmail = x.AuthorEmail
		return nil

	case *wfevents.EvUnarchiveRepoFilesStarted:
		view.scode = unarchiverepowf.StateFiles
		view.aclPolicy = x.AclPolicy
		return nil

	case *wfevents.EvUnarchiveRepoTarttStarted:
		view.scode = unarchiverepowf.StateTartt
		view.workingDir = x.WorkingDir
		return nil

	case *wfevents.EvUnarchiveRepoTarttCompleted:
		if x.StatusCode == 0 {
			view.scode = unarchiverepowf.StateTarttCompleted
		} else {
			view.scode = unarchiverepowf.StateTarttFailed
		}
		return nil

	case *wfevents.EvUnarchiveRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = unarchiverepowf.StateFilesCompleted
		} else {
			view.scode = unarchiverepowf.StateFilesFailed
		}
		return nil

	case *wfevents.EvUnarchiveRepoFilesCommitted:
		view.filesCommittedTime = ulid.Time(vid)
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
		return a.doContinue()

	case unarchiverepowf.StateFiles:
		return a.doPrepareUnarchiveThenContinue(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.startTime,
		)

	// Wait for nogfsorstd to restore from tartt archive.
	case unarchiverepowf.StateTartt:
		return a.doContinue()

	case unarchiverepowf.StateTarttCompleted:
		return a.doUnarchiveRepoThenContinue(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.workingDir,
			view.aclPolicy,
			view.authorName, view.authorEmail,
		)

	case unarchiverepowf.StateTarttFailed:
		return a.doContinue()

	case unarchiverepowf.StateFilesCompleted:
		return a.doContinue()

	case unarchiverepowf.StateFilesFailed:
		return a.doContinue()

	case unarchiverepowf.StateFilesEnded:
		return a.doGcThenQuit(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.workingDir,
			view.filesCommittedTime,
		)

	case unarchiverepowf.StateGcCompleted:
		return a.doQuit()

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

func (a *unarchiveRepoWorkflowActivity) doPrepareUnarchiveThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	startTime time.Time,
) (bool, error) {
	if a.unarchiveRepoSpool == "" {
		return a.doAbortUnarchiveFilesThenContinue(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_STAD_UNARCHIVE_REPO_FAILED),
			"unarchive-repo spool dir not configured",
		)
	}

	// The working directory contains subdirs for the restore and the
	// swapped placeholder.  The working directory will be deleted during
	// garbage collection.
	dir := filepath.Join(
		a.unarchiveRepoSpool,
		fmt.Sprintf(
			"%s_r-%s_w-%s",
			startTime.Format(timex.ISO8601Basic),
			repoId, workflowId,
		),
	)
	if err := a.ensureUnarchiveRepoWorkingDir(dir); err != nil {
		// XXX Maybe inspect error to detect fatal errors and abort.
		return a.doRetry(err)
	}

	c := pb.NewExecUnarchiveRepoClient(a.conn)
	i := &pb.BeginUnarchiveRepoTarttI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
		WorkingDir:  dir,
	}
	_, err := c.BeginUnarchiveRepoTartt(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *unarchiveRepoWorkflowActivity) ensureUnarchiveRepoWorkingDir(
	dir string,
) error {
	// Remove existing dir to handle restart.
	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	// Create and prepare working dir, changing group permissions to allow
	// Nogfsorstd to write to restore and log dir.
	if err := os.Mkdir(dir, 0777); err != nil {
		return err
	}

	restore := filepath.Join(dir, "restore")
	if err := os.Mkdir(restore, 0777); err != nil {
		return err
	}
	if err := os.Chmod(restore, 0770); err != nil {
		return err
	}

	log := filepath.Join(dir, "log")
	if err := os.Mkdir(log, 0777); err != nil {
		return err
	}
	if err := os.Chmod(log, 0770); err != nil {
		return err
	}

	// Fsync to ensure durability.
	return fsyncPaths([]string{
		restore,
		log,
		dir,
		filepath.Dir(dir),
	})
}

func (a *unarchiveRepoWorkflowActivity) doUnarchiveRepoThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	workingDir string,
	aclPolicy *pb.RepoAclPolicy,
	authorName, authorEmail string,
) (bool, error) {
	if err := a.setfaclChattrRestore(
		ctx, workingDir, aclPolicy,
	); err != nil {
		// XXX Maybe inspect error to detect fatal errors and abort.
		return a.doRetry(err)
	}

	// Synchronize with observer6 on a per-repo basis during startup to
	// ensure that the repo is enabled before trying to unarchive it.  See
	// comment at `WaitEnableRepo4()`.
	if err := a.repoProc.WaitEnableRepo4(ctx, repoId); err != nil {
		return a.doRetry(err)
	}

	author := nogfsostad.GitUser{
		Name:  authorName,
		Email: authorEmail,
	}
	err := a.repoProc.UnarchiveRepo(ctx, repoId, workingDir, author)
	if err != nil {
		// Retry a few times, because recovering from repo errors is
		// relatively expensive, and some errors might be temporary,
		// for example sudoudod might not yet be ready.
		if a.nRetries < ConfigMaxUnarchiveRepoRetries {
			a.nRetries++
			return a.doRetry(err)
		}
		return a.doAbortUnarchiveFilesThenContinue(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_STAD_UNARCHIVE_REPO_FAILED),
			truncateErrorMessage(err.Error()),
		)
	}
	a.nRetries = 0
	return a.doCommitUnarchiveFilesThenContinue(
		ctx, workflowId, vid,
	)
}

func (a *unarchiveRepoWorkflowActivity) setfaclChattrRestore(
	ctx context.Context,
	workingDir string,
	aclPolicy *pb.RepoAclPolicy,
) error {
	restore := filepath.Join(workingDir, "restore")

	switch aclPolicy.Policy {
	case pb.RepoAclPolicy_P_PROPAGATE_ROOT_ACLS:
		if a.aclPropagator == nil {
			return ErrAclsDisabled
		}
		if err := a.aclPropagator.PropagateAcls(
			ctx, aclPolicy.FsoRootInfo.HostRoot, restore,
		); err != nil {
			return err
		}
	}

	sudo, err := a.privs.AcquireUdoChattr(ctx, "root")
	if err != nil {
		return err
	}
	defer sudo.Release()
	if err := sudo.ChattrTreeSetImmutable(ctx, restore); err != nil {
		return err
	}

	return nil
}

func (a *unarchiveRepoWorkflowActivity) doCommitUnarchiveFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewExecUnarchiveRepoClient(a.conn)
	i := &pb.CommitUnarchiveRepoFilesI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitUnarchiveRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *unarchiveRepoWorkflowActivity) doAbortUnarchiveFilesThenContinue(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	statusCode int32, statusMessage string,
) (bool, error) {
	c := pb.NewExecUnarchiveRepoClient(a.conn)
	i := &pb.AbortUnarchiveRepoFilesI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortUnarchiveRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doContinue()
}

func (a *unarchiveRepoWorkflowActivity) doGcThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	workingDir string,
	filesCommittedTime time.Time,
) (bool, error) {
	due := filesCommittedTime.Add(ConfigUnarchiveRepoGCDelay)
	if time.Now().Before(due) {
		return a.doRetrySilentAfter(due)
	}

	if exists(workingDir) {
		sudo, err := a.privs.AcquireUdoChattr(ctx, "root")
		if err != nil {
			return a.doRetry(err)
		}
		defer sudo.Release()
		if err := sudo.ChattrTreeUnsetImmutable(
			ctx, workingDir,
		); err != nil {
			return a.doRetry(err)
		}
	}
	if err := os.RemoveAll(workingDir); err != nil {
		return a.doRetry(err)
	}

	c := pb.NewExecUnarchiveRepoClient(a.conn)
	i := &pb.CommitUnarchiveRepoGcI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	if _, err := c.CommitUnarchiveRepoGc(
		ctx, i, a.sysRPCCreds,
	); err != nil {
		return a.doRetry(err)
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

func (a *unarchiveRepoWorkflowActivity) doRetrySilent() (bool, error) {
	return false, grpcentities.SilentRetry
}

func (a *unarchiveRepoWorkflowActivity) doRetrySilentAfter(
	after time.Time,
) (bool, error) {
	return false, &grpcentities.SilentRetryAfter{After: after}
}
