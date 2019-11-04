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

type lazyRegistryWorkflowActivity struct {
	registry string
	act      grpcentities.RegistryWorkflowActivity
}

func (e *Engine) StartRegistryWorkflowActivity(
	registry string,
	workflowId uuid.I,
	act grpcentities.RegistryWorkflowActivity,
) error {
	return e.addNewTask(
		workflowId,
		&lazyRegistryWorkflowActivity{
			registry: registry,
			act:      act,
		},
	)
}

func (act *lazyRegistryWorkflowActivity) run(
	e *Engine, ctx context.Context, t *task,
) (ulid.I, error) {
	registry := act.registry
	workflowId := t.entityId
	tail := t.tail
	return e.runRegistryWorkflow(ctx, registry, workflowId, tail, act.act)
}

func (e *Engine) runRegistryWorkflow(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	act grpcentities.RegistryWorkflowActivity,
) (ulid.I, error) {
	newTail, err := e.runRegistryWorkflowStreamNoBlock(
		ctx, registry, workflowId, tail, act,
	)
	switch {
	case err == nil:
		e.lg.Infow(
			"Completed processing registry workflow activity.",
			"registry", registry,
			"workflowId", workflowId.String(),
		)
	case err == context.Canceled:
		// Handle cancel silently.
	case err == io.EOF:
		if newTail != tail {
			e.lg.Infow(
				"Repo workflow activity progressed.",
				"registry", registry,
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
			"Will retry registry workflow activity.",
			"err", err,
			"registry", registry,
			"workflowId", workflowId.String(),
			"afterEvent", afterEvent,
		)
	}
	return newTail, err
}

func (e *Engine) runRegistryWorkflowStreamNoBlock(
	ctx context.Context,
	registry string,
	workflowId uuid.I,
	tail ulid.I,
	act grpcentities.RegistryWorkflowActivity,
) (ulid.I, error) {
	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewEphemeralRegistryClient(e.conn)
	req := &pb.RegistryWorkflowEventsI{
		Registry: registry,
		Workflow: workflowId[:],
		Watch:    false,
	}
	if tail != ulid.Nil {
		req.After = tail[:]
	}
	stream, err := c.RegistryWorkflowEvents(ctx2, req, e.sysRPCCreds)
	if err != nil {
		return tail, err
	}

	return act.ProcessRegistryWorkflowEvents(
		ctx2, registry, workflowId, tail, stream,
	)
}
