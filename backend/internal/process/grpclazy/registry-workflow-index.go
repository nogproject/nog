package grpclazy

import (
	"context"
	"fmt"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

const NsFsoRegistryEphemeralWorkflows = "fsoregephwfl"

type lazyRegistryWorkflowIndexActivity struct {
	registry string
	act      grpcentities.RegistryWorkflowIndexActivity
}

func (e *Engine) StartRegistryWorkflowIndexActivity(
	registry string, act grpcentities.RegistryWorkflowIndexActivity,
) error {
	registryId := e.names.UUID(NsFsoRegistryEphemeralWorkflows, registry)
	return e.addNewTask(registryId, &lazyRegistryWorkflowIndexActivity{
		registry: registry,
		act:      act,
	})
}

func (act *lazyRegistryWorkflowIndexActivity) run(
	e *Engine, ctx context.Context, t *task,
) (ulid.I, error) {
	registry := act.registry
	tail := t.tail
	return e.runRegistryWorkflowIndexActivity(ctx, registry, tail, act.act)
}

func (e *Engine) runRegistryWorkflowIndexActivity(
	ctx context.Context,
	registry string,
	tail ulid.I,
	act grpcentities.RegistryWorkflowIndexActivity,
) (ulid.I, error) {
	newTail, err := e.runRegistryWorkflowIndexActivityStreamNoBlock(
		ctx, registry, tail, act,
	)
	switch {
	case err == nil:
		e.lg.Infow(
			"Completed processing "+
				"registry workflow index activity.",
			"registry", registry,
		)
	case err == context.Canceled:
		// Handle cancel silently.
	case err == io.EOF:
		if newTail != tail {
			e.lg.Infow(
				"Registry workflow index activity progressed.",
				"registry", registry,
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
			"Will retry registry workflow index activity.",
			"err", err,
			"registry", registry,
			"afterEvent", afterEvent,
		)
	}
	return newTail, err
}

func (e *Engine) runRegistryWorkflowIndexActivityStreamNoBlock(
	ctx context.Context,
	registry string,
	tail ulid.I,
	act grpcentities.RegistryWorkflowIndexActivity,
) (ulid.I, error) {
	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewEphemeralRegistryClient(e.conn)
	req := &pb.RegistryWorkflowIndexEventsI{
		Registry: registry,
		Watch:    false,
	}
	if tail != ulid.Nil {
		req.After = tail[:]
	}
	stream, err := c.RegistryWorkflowIndexEvents(ctx2, req, e.sysRPCCreds)
	if err != nil {
		return tail, err
	}

	return act.ProcessRegistryWorkflowIndexEvents(
		ctx2, registry, tail, stream,
	)
}
