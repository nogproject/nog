package grpclazy

import (
	"context"
	"fmt"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
)

const NsFsoRegistry = "fsoreg"

type lazyRegistryActivity struct {
	registry string
	act      grpcentities.RegistryActivity
}

func (e *Engine) StartRegistryActivity(
	registry string, act grpcentities.RegistryActivity,
) error {
	registryId := e.names.UUID(NsFsoRegistry, registry)
	return e.addNewTask(registryId, &lazyRegistryActivity{
		registry: registry,
		act:      act,
	})
}

func (act *lazyRegistryActivity) run(
	e *Engine, ctx context.Context, t *task,
) (ulid.I, error) {
	registry := act.registry
	tail := t.tail
	return e.runRegistry(ctx, registry, tail, act.act)
}

func (e *Engine) runRegistry(
	ctx context.Context,
	registry string,
	tail ulid.I,
	act grpcentities.RegistryActivity,
) (ulid.I, error) {
	newTail, err := e.runRegistryStreamNoBlock(ctx, registry, tail, act)
	switch {
	case err == nil:
		e.lg.Infow(
			"Completed processing registry activity.",
			"registry", registry,
		)
	case err == context.Canceled:
		// Handle cancel silently.
	case err == io.EOF:
		if newTail != tail {
			e.lg.Infow(
				"Registry activity progressed.",
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
			"Will retry registry activity.",
			"err", err,
			"registry", registry,
			"afterEvent", afterEvent,
		)
	}
	return newTail, err
}

func (e *Engine) runRegistryStreamNoBlock(
	ctx context.Context,
	registry string,
	tail ulid.I,
	act grpcentities.RegistryActivity,
) (ulid.I, error) {
	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewRegistryClient(e.conn)
	req := &pb.RegistryEventsI{
		Registry: registry,
		Watch:    false,
	}
	if tail != ulid.Nil {
		req.After = tail[:]
	}
	stream, err := c.Events(ctx2, req, e.sysRPCCreds)
	if err != nil {
		return tail, err
	}

	return act.ProcessRegistryEvents(ctx2, registry, tail, stream)
}
