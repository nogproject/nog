package observer6

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	workflowsev "github.com/nogproject/nog/backend/internal/workflows/events"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

type moveRepoWorkflowAcquireActivity struct {
	chainedRepoActivity
	initer      Initializer
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption
}

// `moveRepoWorkflowAcquireActivity`:
//
//  - wait for `WorkflowEvent_EV_FSO_REPO_MOVE_STA_RELEASED` aka
//    `EvRepoMoveStaReleased`;
//  - wait for `WorkflowEvent_EV_FSO_REPO_MOVE_APP_ACCEPTED` aka
//    `EvRepoMoveAppAccepted`;
//  - ask an admin to move the repo in `initer.MoveRepo()`;
//  - observe the filesystem to confirm the move;
//  - commit the new location.
//
// Always restart from epoch, i.e. `return ulid.Nil, ...`, to keep logic
// simple.
//
// Retry all errors, assuming that an admin is monitoring the workflow
// execution and would fix issues right away.
func (a *moveRepoWorkflowAcquireActivity) ProcessRepoWorkflowEvents(
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
	var oldHostPath string
	var oldShadowPath string
	var newHostPath string
	otherHasReleased := false
	appHasAccepted := false
	hasMoved := false
	isTerminated := false

	apply := func(pbev *pb.WorkflowEvent) error {
		ev, err := workflowsev.ParsePbWorkflowEvent(pbev)
		if err != nil {
			return err
		}
		switch x := ev.(type) {
		case *workflowsev.EvRepoMoveStarted:
			oldHostPath = x.OldHostPath
			oldShadowPath = x.OldShadowPath
			newHostPath = x.NewHostPath
			return nil

		case *workflowsev.EvRepoMoveStaReleased:
			otherHasReleased = true
			return nil

		case *workflowsev.EvRepoMoveAppAccepted:
			appHasAccepted = true
			return nil

		case *workflowsev.EvRepoMoved:
			hasMoved = true
			return nil

		case *workflowsev.EvRepoMoveCommitted:
			isTerminated = true
			return nil

		default:
			a.lg.Warnw(
				"Ignored unknown move-repo workflow event.",
				"event", pbev.Event.String(),
			)
			return nil
		}
	}

	onBlock := func() error {
		if !otherHasReleased || !appHasAccepted {
			return nil
		}
		if hasMoved {
			return nil
		}

		newShadowPath, err := a.initer.MoveRepo(
			ctx, repoId, oldHostPath, oldShadowPath, newHostPath,
		)
		if err != nil {
			return err
		}

		c := pb.NewReposClient(a.conn)
		_, err = c.CommitMoveRepo(
			ctx,
			&pb.CommitMoveRepoI{
				Repo:          repoId[:],
				Workflow:      workflowId[:],
				NewShadowPath: newShadowPath,
			},
			a.sysRPCCreds,
		)
		if err != nil {
			return err
		}

		hasMoved = true
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

	for !isTerminated {
		rsp, err := stream.Recv()
		switch {
		case err == io.EOF:
			if err := onBlock(); err != nil {
				return doRetry(err)
			}
			a.lg.Infow(
				"Expecting more move-repo acquire workflow events.",
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
				"Waiting for move-repo acquire workflow events.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
			)
		}
	}

	return doQuit()
}
