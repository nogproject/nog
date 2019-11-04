package grpclazy

import (
	"context"
	"fmt"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type lazyRepoWorkflowActivity struct {
	repoId uuid.I
	act    grpcentities.RepoWorkflowActivity
}

func (e *Engine) StartRepoWorkflowActivity(
	repoId uuid.I,
	workflowId uuid.I,
	act grpcentities.RepoWorkflowActivity,
) error {
	return e.addNewTask(
		workflowId,
		&lazyRepoWorkflowActivity{
			repoId: repoId,
			act:    act,
		},
	)
}

func (act *lazyRepoWorkflowActivity) run(
	e *Engine, ctx context.Context, t *task,
) (ulid.I, error) {
	repoId := act.repoId
	workflowId := t.entityId
	tail := t.tail
	return e.runRepoWorkflow(ctx, repoId, workflowId, tail, act.act)
}

func (e *Engine) runRepoWorkflow(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	tail ulid.I,
	act grpcentities.RepoWorkflowActivity,
) (ulid.I, error) {
	newTail, err := e.runRepoWorkflowStreamNoBlock(
		ctx, repoId, workflowId, tail, act,
	)
	switch {
	case err == nil:
		e.lg.Infow(
			"Completed processing repo workflow activity.",
			"repoId", repoId.String(),
			"workflowId", workflowId.String(),
		)
	case err == context.Canceled:
		// Handle cancel silently.
	case err == io.EOF:
		if newTail != tail {
			e.lg.Infow(
				"Repo workflow activity progressed.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
				"vid", newTail.String(),
			)
		}
	case grpcentities.IsSilentRetry(err):
		// Handle silently.
	case grpcentities.IsSilentRetryAfter(err):
		// Handle silently.
	default:
		afterEvent := "Epoch"
		if newTail != ulid.Nil {
			afterEvent = fmt.Sprintf("%v", newTail)
		}
		e.lg.Errorw(
			"Will retry repo workflow activity.",
			"err", err,
			"repoId", repoId.String(),
			"workflowId", workflowId.String(),
			"afterEvent", afterEvent,
		)
	}
	return newTail, err
}

func (e *Engine) runRepoWorkflowStreamNoBlock(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	tail ulid.I,
	act grpcentities.RepoWorkflowActivity,
) (ulid.I, error) {
	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewReposClient(e.conn)
	req := &pb.RepoWorkflowEventsI{
		Repo:     repoId[:],
		Workflow: workflowId[:],
		Watch:    false,
	}
	if tail != ulid.Nil {
		req.After = tail[:]
	}
	stream, err := c.WorkflowEvents(ctx2, req, e.sysRPCCreds)
	if err != nil {
		return tail, err
	}

	return act.ProcessRepoWorkflowEvents(
		ctx2, repoId, workflowId, tail, stream,
	)
}
