package grpceager

import (
	"context"
	"fmt"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (e *Engine) StartRegistryActivity(
	registry string, act grpcentities.RegistryActivity,
) error {
	if err := e.wgAdd(); err != nil {
		return err
	}

	// We could call `e.streamLimiter.Acquire()` here, before spawning the
	// goroutine, to allow the limiter to block and, thus, defer creating
	// new goroutines.  We would then have to always proceed to the
	// corresponding `Release()` in the goroutine to avoid leaking limiter
	// tokens.
	//
	// We do not use `streamLimiter` here to keep it simple.  We use
	// `streamLimiter` only close to where we create a stream.  The number
	// of goroutines would have to be limited on a higher level by the
	// caller of `StartRepoActivity()`, if desired.

	go func() {
		defer e.wg.Done()
		_ = e.runRegistryActivityRetry(e.ctx, registry, act)
	}()

	return nil
}

func (e *Engine) runRegistryActivityRetry(
	ctx context.Context,
	registry string,
	act grpcentities.RegistryActivity,
) error {
	tail := ulid.Nil
	for {
		newTail, err := e.runRegistryActivity(ctx, registry, tail, act)
		// Nil indicates that the activity has completed and need not
		// be restarted.
		if err == nil {
			e.lg.Infow(
				"Completed processing registry activity.",
				"registry", registry,
			)
			return nil
		}
		// If the error is due to context cancel, which indicates
		// engine shutdown, return early without logging.
		if err == context.Canceled {
			return err
		}
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.Canceled {
				return context.Canceled
			}
		}
		// The `RegistryActivity` return value `newTail` indicates
		// which events will be loaded during restart.  A sensible
		// strategy is:
		//
		//  - If an error occurs during initial loading,
		//    `RegistryActivity` will return `ulid.Nil` to restart
		//    loading with all events.
		//  - If an error occurs during continuous processing,
		//    `RegistryActivity` will return the last processed event
		//    in order to restart processing with the next event.
		//
		tail = newTail

		wait := 20 * time.Second
		afterEvent := "Epoch"
		if tail != ulid.Nil {
			afterEvent = fmt.Sprintf("%v", tail)
		}
		e.lg.Errorw(
			"Will retry watch registry.",
			"err", err,
			"registry", registry,
			"afterEvent", afterEvent,
			"retryIn", wait,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (e *Engine) runRegistryActivity(
	ctx context.Context,
	registry string,
	tail ulid.I,
	act grpcentities.RegistryActivity,
) (ulid.I, error) {
	limiter := e.streamLimiter
	if limiter != nil {
		if err := limiter.Acquire(ctx, 1); err != nil {
			return tail, err
		}
		defer limiter.Release(1)
	}

	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewRegistryClient(e.conn)
	req := &pb.RegistryEventsI{
		Registry: registry,
		Watch:    true,
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
