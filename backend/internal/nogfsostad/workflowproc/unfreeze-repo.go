package workflowproc

import (
	"context"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/nogfsostad"
	wfevents "github.com/nogproject/nog/backend/internal/workflows/events"
	wfstreams "github.com/nogproject/nog/backend/internal/workflows/eventstreams"
	"github.com/nogproject/nog/backend/internal/workflows/unfreezerepowf"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const ConfigMaxUnfreezeRepoRetries = 5

type unfreezeRepoWorkflowActivity struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
	repoProc    RepoProcessor
	registry    string
	done        chan<- struct{}
	view        unfreezeRepoWorkflowView
	tail        ulid.I
	nRetries    int
}

type unfreezeRepoWorkflowView struct {
	workflowId  uuid.I
	vid         ulid.I
	scode       unfreezerepowf.StateCode
	repoId      uuid.I
	authorName  string
	authorEmail string
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
		view.repoId = x.RepoId
		view.authorName = x.AuthorName
		view.authorEmail = x.AuthorEmail
		return nil

	case *wfevents.EvUnfreezeRepoFilesStarted:
		view.scode = unfreezerepowf.StateFiles
		return nil

	case *wfevents.EvUnfreezeRepoFilesCompleted:
		if x.StatusCode == 0 {
			view.scode = unfreezerepowf.StateFilesCompleted
		} else {
			view.scode = unfreezerepowf.StateFilesFailed
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
		return a.doContinue()

	case unfreezerepowf.StateFiles:
		return a.doUnfreezeRepoThenQuit(
			ctx,
			view.workflowId, view.vid,
			view.repoId,
			view.authorName, view.authorEmail,
		)

	case unfreezerepowf.StateFilesCompleted:
		return a.doQuit()

	case unfreezerepowf.StateFilesFailed:
		return a.doQuit()

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

func (a *unfreezeRepoWorkflowActivity) doUnfreezeRepoThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	repoId uuid.I,
	authorName, authorEmail string,
) (bool, error) {
	// Synchronize with observer6 on a per-repo basis during startup to
	// ensure that a repo is enabled before trying to unfreeze it.  See
	// comment at `WaitEnableRepo4()`.
	if err := a.repoProc.WaitEnableRepo4(ctx, repoId); err != nil {
		return a.doRetry(err)
	}

	author := nogfsostad.GitUser{
		Name:  authorName,
		Email: authorEmail,
	}
	err := a.repoProc.UnfreezeRepo(ctx, repoId, author)
	if err != nil {
		// Retry a few times, because recovering from repo errors is
		// relatively expensive, and some errors might be temporary,
		// for example sudoudod might not yet be ready.
		if a.nRetries < ConfigMaxUnfreezeRepoRetries {
			a.nRetries++
			return a.doRetry(err)
		}
		return a.doAbortUnfreezeFilesThenQuit(
			ctx, workflowId, vid,
			int32(pb.StatusCode_SC_STAD_UNFREEZE_REPO_FAILED),
			truncateErrorMessage(err.Error()),
		)
	}
	a.nRetries = 0
	return a.doCommitUnfreezeFilesThenQuit(
		ctx, workflowId, vid,
	)
}

func (a *unfreezeRepoWorkflowActivity) doCommitUnfreezeFilesThenQuit(
	ctx context.Context,
	workflowId uuid.I,
	vid ulid.I,
) (bool, error) {
	c := pb.NewUnfreezeRepoClient(a.conn)
	i := &pb.CommitUnfreezeRepoFilesI{
		Workflow:    workflowId[:],
		WorkflowVid: vid[:],
	}
	_, err := c.CommitUnfreezeRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
	}
	return a.doQuit()
}

func (a *unfreezeRepoWorkflowActivity) doAbortUnfreezeFilesThenQuit(
	ctx context.Context,
	workflowId uuid.I, vid ulid.I,
	statusCode int32, statusMessage string,
) (bool, error) {
	c := pb.NewUnfreezeRepoClient(a.conn)
	i := &pb.AbortUnfreezeRepoFilesI{
		Workflow:      workflowId[:],
		WorkflowVid:   vid[:],
		StatusCode:    statusCode,
		StatusMessage: statusMessage,
	}
	_, err := c.AbortUnfreezeRepoFiles(ctx, i, a.sysRPCCreds)
	if err != nil {
		return a.doRetry(err)
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
