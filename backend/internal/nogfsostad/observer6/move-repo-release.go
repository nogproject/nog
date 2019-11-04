package observer6

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type moveRepoWorkflowReleaseActivity struct {
	chainedRepoActivity
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
}

// `moveRepoWorkflowReleaseActivity`:
//
//  - disable the repo;
//  - post `EV_FSO_REPO_MOVE_STA_RELEASED` if necessary.
//
// Always restart from epoch, i.e. `return ulid.Nil, ...`, to keep logic
// simple.
//
// Retry all errors, assuming that an admin is monitoring the workflow
// execution and would fix issues right away.
func (a *moveRepoWorkflowReleaseActivity) ProcessRepoWorkflowEvents(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	tail ulid.I,
	stream pb.Repos_WorkflowEventsClient,
) (ulid.I, error) {
	if a.chain.Dep != nil {
		panic("moveRepoWorkflowReleaseActivity must not have dep.")
	}

	// Workflow state.
	hasAppAccepted := false
	hasReleased := false

	apply := func(ev *pb.WorkflowEvent) {
		switch ev.Event {
		case pb.WorkflowEvent_EV_FSO_REPO_MOVE_STARTED:
			return

		case pb.WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED:
			hasAppAccepted = true
			return

		case pb.WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED:
			hasReleased = true
			return

		case pb.WorkflowEvent_EV_FSO_REPO_MOVED:
			return

		case pb.WorkflowEvent_EV_FSO_REPO_MOVE_COMMITTED:
			return

		default:
			a.lg.Warnw(
				"Ignored unknown move-repo workflow event.",
				"event", ev.Event.String(),
			)
			return
		}
	}

	onBlock := func() error {
		if !hasAppAccepted {
			return nil
		}
		if hasReleased {
			return nil
		}
		c := pb.NewReposClient(a.conn)
		if _, err := c.PostMoveRepoStaReleased(
			ctx,
			&pb.PostMoveRepoStaReleasedI{
				Repo:     repoId[:],
				Workflow: workflowId[:],
			},
			a.sysRPCCreds,
		); err != nil {
			return err
		}
		hasReleased = true
		return nil
	}

	doRetry := func(err error) (ulid.I, error) {
		_, err = a.doRetry(ctx, err)
		return ulid.Nil, err
	}

	doQuit := func() (ulid.I, error) {
		_, _ = a.doQuit()
		return ulid.Nil, nil
	}

	for !hasReleased {
		rsp, err := stream.Recv()
		switch {
		case err == io.EOF:
			if err := onBlock(); err != nil {
				return doRetry(err)
			}
			a.lg.Infow(
				"Expecting more move-repo release workflow events.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
			)
			return ulid.Nil, io.EOF
		case err != nil:
			return ulid.Nil, err
		}

		for _, ev := range rsp.Events {
			apply(ev)
		}

		if rsp.WillBlock {
			if err := onBlock(); err != nil {
				return doRetry(err)
			}
			a.lg.Infow(
				"Waiting for move-repo release workflow events.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
			)
		}
	}

	return doQuit()
}
