package workflowproc

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/unarchiverepowf"
	"github.com/nogproject/nog/backend/pkg/execx"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const ConfigMaxUnarchiveRepoRetries = 5

var tarttTool = execx.MustLookTool(execx.ToolSpec{
	"tartt",
	[]string{"--version"},
	"tartt-",
})

type unarchiveRepoWorkflowActivity struct {
	lg            Logger
	conn          *grpc.ClientConn
	sysRPCCreds   grpc.CallOption
	expectedHosts map[string]struct{}
	capPath       string
	tarttLimiter  Limiter
	view          unarchiveRepoWorkflowView
	tail          ulid.I
	nRetries      int
}

type unarchiveRepoWorkflowView struct {
	workflowId     uuid.I
	vid            ulid.I
	scode          unarchiverepowf.StateCode
	repoId         uuid.I
	repoArchiveURL string
	tsPath         string
	workingDir     string
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
		view.repoArchiveURL = x.RepoArchiveURL
		view.tsPath = x.TarttTarPath
		return nil

	case *wfevents.EvUnarchiveRepoFilesStarted:
		view.scode = unarchiverepowf.StateFiles
		return nil

	case *wfevents.EvUnarchiveRepoTarttStarted:
		view.scode = unarchiverepowf.StateTartt
		view.workingDir = x.WorkingDir
		return nil

	// Handle all further progress as terminated.
	case *wfevents.EvUnarchiveRepoTarttCompleted:
		view.scode = unarchiverepowf.StateTerminated
		return nil
	case *wfevents.EvUnarchiveRepoFilesCompleted:
		view.scode = unarchiverepowf.StateTerminated
		return nil
	case *wfevents.EvUnarchiveRepoFilesCommitted:
		view.scode = unarchiverepowf.StateTerminated
		return nil
	case *wfevents.EvUnarchiveRepoGcCompleted:
		view.scode = unarchiverepowf.StateTerminated
		return nil
	case *wfevents.EvUnarchiveRepoCompleted:
		view.scode = unarchiverepowf.StateTerminated
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
		return a.doContinue()

	case unarchiverepowf.StateTartt:
		return a.doTarttRestoreThenQuit(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.repoArchiveURL, view.tsPath,
			view.workingDir,
		)

	case unarchiverepowf.StateTerminated:
		return a.doQuit()

	default:
		panic("invalid StateCode")
	}
}

func (a *unarchiveRepoWorkflowActivity) doTarttRestoreThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	repoArchiveURL, tsPath string,
	workingDir string,
) (bool, error) {
	err := a.tarttRestore(ctx, repoArchiveURL, tsPath, workingDir)
	switch {
	case err == context.Canceled:
		return a.doRetry(err)
	case err != nil:
		// Retry other errors a few times, because recovering from repo
		// errors is relatively expensive, and some errors might be
		// temporary, for example sudoudod might not yet be ready.
		if a.nRetries < ConfigMaxUnarchiveRepoRetries {
			a.nRetries++
			return a.doRetry(err)
		}
		return a.doAbortTarttThenQuit(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_RSTD_UNARCHIVE_REPO_FAILED),
			truncateErrorMessage(err.Error()),
		)
	}
	a.nRetries = 0
	return a.doCommitTarttThenQuit(
		ctx, workflowId, vid,
	)
}

func (a *unarchiveRepoWorkflowActivity) tarttRestore(
	ctx context.Context,
	repoArchiveURL, tsPath string,
	workingDir string,
) error {
	repo, err := url.Parse(repoArchiveURL)
	if err != nil {
		return err
	}
	if _, ok := a.expectedHosts[repo.Host]; !ok {
		return ErrWrongHost
	}

	restore := filepath.Join(workingDir, "restore")
	log := filepath.Join(workingDir, "log/tartt-restore.log")

	if err := a.tarttLimiter.Acquire(ctx, 1); err != nil {
		return err
	}
	defer a.tarttLimiter.Release(1)

	logFp, err := os.OpenFile(
		log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666,
	)
	if err != nil {
		return err
	}
	logFpClose := func() error {
		if logFp == nil {
			return nil
		}
		err := logFp.Close()
		logFp = nil
		return err
	}
	defer func() { _ = logFpClose() }()

	if _, err := fmt.Fprintf(
		logFp,
		"Started tartt restore.\n"+
			"startTime: %s\n"+
			"tarttRepo: %s\n"+
			"tspath: %s\n",
		time.Now().Format(time.RFC3339),
		repo.Path,
		tsPath,
	); err != nil {
		return err
	}
	if err := logFp.Sync(); err != nil {
		return err
	}

	cmd := exec.CommandContext(
		ctx,
		tarttTool.Path,
		"-C", repo.Path,
		"restore",
		fmt.Sprintf("--dest=%s", restore),
		tsPath,
	)
	if a.capPath != "" {
		path := fmt.Sprintf("PATH=%s:%s", a.capPath, os.Getenv("PATH"))
		cmd.Env = append(os.Environ(), path)
	}
	cmd.Stdout = logFp
	cmd.Stderr = logFp
	if err := cmd.Run(); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(
		logFp,
		"endTime: %s\n"+
			"Completed tartt restore.\n",
		time.Now().Format(time.RFC3339),
	); err != nil {
		return err
	}
	if err := logFp.Sync(); err != nil {
		return err
	}

	return logFpClose()
}

func (a *unarchiveRepoWorkflowActivity) doCommitTarttThenQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewExecUnarchiveRepoClient(a.conn)
	i := &pb.CommitUnarchiveRepoTarttI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitUnarchiveRepoTartt(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *unarchiveRepoWorkflowActivity) doAbortTarttThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	statusCode int32, statusMessage string,
) (bool, error) {
	c := pb.NewExecUnarchiveRepoClient(a.conn)
	i := &pb.AbortUnarchiveRepoTarttI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortUnarchiveRepoTartt(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *unarchiveRepoWorkflowActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *unarchiveRepoWorkflowActivity) doQuit() (bool, error) {
	return true, nil
}

func (a *unarchiveRepoWorkflowActivity) doRetry(err error) (bool, error) {
	return false, err
}
