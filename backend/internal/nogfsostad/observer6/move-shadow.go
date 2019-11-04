package observer6

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type moveShadowWorkflowActivity struct {
	chainedRepoActivity
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
}

// `moveShadowWorkflowActivity`:
//
//  - disables the repo;
//  - posts `EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED` if necessary;
//  - waits for `EV_FSO_SHADOW_REPO_MOVED`, which indicates that the repo is at
//    its new location;
//  - re-enables the repo.
//
// Always restart from epoch, i.e. `return ulid.Nil, ...`, to keep logic
// simple.
//
// Retry all errors, assuming that an admin is monitoring the workflow
// execution and would fix issues right away.
func (a *moveShadowWorkflowActivity) ProcessRepoWorkflowEvents(
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
	isInitialized := false
	hasDisabledEvent := false
	isTerminated := false

	apply := func(ev *pb.WorkflowEvent) error {
		switch ev.Event {
		case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STARTED:
			isInitialized = true
			return nil

		case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_STA_DISABLED:
			hasDisabledEvent = true
			return nil

		case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVED:
			return nil

		case pb.WorkflowEvent_EV_FSO_SHADOW_REPO_MOVE_COMMITTED:
			isTerminated = true
			return nil

		default:
			a.lg.Warnw(
				"Ignored unknown move-shadow workflow event.",
				"event", ev.Event.String(),
			)
			return nil
		}
	}

	onBlock := func() error {
		if !isInitialized {
			return nil
		}
		if hasDisabledEvent {
			return nil
		}
		c := pb.NewReposClient(a.conn)
		_, err := c.PostMoveShadowStaDisabled(
			ctx,
			&pb.PostMoveShadowStaDisabledI{
				Repo:     repoId[:],
				Workflow: workflowId[:],
			},
			a.sysRPCCreds,
		)
		return err
	}

	doRetry := func(err error) (ulid.I, error) {
		_, err = a.doRetry(ctx, err)
		return ulid.Nil, err
	}

	doQuit := func() (ulid.I, error) {
		_, _ = a.doQuit()
		return ulid.Nil, nil
	}

	for !isTerminated {
		rsp, err := stream.Recv()
		switch {
		case err == io.EOF:
			if err := onBlock(); err != nil {
				return doRetry(err)
			}
			a.lg.Infow(
				"Expecting more move-shadow workflow events.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
			)
			return ulid.Nil, io.EOF
		case err != nil:
			return ulid.Nil, err
		}

		for _, ev := range rsp.Events {
			if err := apply(ev); err != nil {
				return doRetry(err)
			}
		}

		if rsp.WillBlock {
			if err := onBlock(); err != nil {
				return doRetry(err)
			}
			a.lg.Infow(
				"Waiting for move-shadow workflow events.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
			)
		}
	}

	return doQuit()
}
