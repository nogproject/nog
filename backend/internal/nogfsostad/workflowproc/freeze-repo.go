package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/freezerepowf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const ConfigMaxFreezeRepoRetries = 5

type freezeRepoWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	repoProc    RepoProcessor
	registry    string
	done        chan<- struct{}
	view        freezeRepoWorkflowView
	tail        ulid.I
	nRetries    int
}

type freezeRepoWorkflowView struct {
	workflowId  uuid.I
	vid         ulid.I
	scode       freezerepowf.StateCode
	repoId      uuid.I
	authorName  string
	authorEmail string
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
		view.repoId = x.RepoId
		view.authorName = x.AuthorName
		view.authorEmail = x.AuthorEmail
		return nil

	case *wfevents.EvFreezeRepoFilesStarted:
		view.scode = freezerepowf.StateFiles
		return nil

	case *wfevents.EvFreezeRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = freezerepowf.StateFilesCompleted
		} else {
			view.scode = freezerepowf.StateFilesFailed
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
		return a.doContinue()

	case freezerepowf.StateFiles:
		return a.doFreezeRepoThenQuit(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.authorName, view.authorEmail,
		)

	case freezerepowf.StateFilesCompleted:
		return a.doQuit()

	case freezerepowf.StateFilesFailed:
		return a.doQuit()

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

func (a *freezeRepoWorkflowActivity) doFreezeRepoThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	authorName, authorEmail string,
) (bool, error) {
	// Synchronize with observer6 on a per-repo basis during startup to
	// ensure that a repo is enabled before trying to freeze it.  See
	// comment at `WaitEnableRepo4()`.
	if err := a.repoProc.WaitEnableRepo4(ctx, repoId); err != nil {
		return a.doRetry(err)
	}

	author := nogfsostad.GitUser{
		Name:  authorName,
		Email: authorEmail,
	}
	err := a.repoProc.FreezeRepo(ctx, repoId, author)
	if err != nil {
		// Retry a few times, because recovering from repo errors is
		// relatively expensive, and some errors might be temporary,
		// for example sudoudod might not yet be ready.
		if a.nRetries < ConfigMaxFreezeRepoRetries {
			a.nRetries++
			return a.doRetry(err)
		}
		return a.doAbortFreezeFilesThenQuit(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_STAD_FREEZE_REPO_FAILED),
			truncateErrorMessage(err.Error()),
		)
	}
	a.nRetries = 0
	return a.doCommitFreezeFilesThenQuit(
		ctx, workflowId, vid,
	)
}

func (a *freezeRepoWorkflowActivity) doCommitFreezeFilesThenQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewFreezeRepoClient(a.conn)
	i := &pb.CommitFreezeRepoFilesI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitFreezeRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *freezeRepoWorkflowActivity) doAbortFreezeFilesThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	statusCode int32, statusMessage string,
) (bool, error) {
	c := pb.NewFreezeRepoClient(a.conn)
	i := &pb.AbortFreezeRepoFilesI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortFreezeRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
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
