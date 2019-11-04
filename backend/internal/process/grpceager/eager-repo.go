package grpceager

import (
	"context"
	"fmt"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// See comments in `./registry.go`.
func (e *Engine) StartRepoActivity(
	repoId uuid.I, act grpcentities.RepoActivity,
) error {
	if err := e.wgAdd(); err != nil {
		return err
	}

	go func() {
		defer e.wg.Done()
		_ = e.runRepoActivityRetry(e.ctx, repoId, act)
	}()

	return nil
}

// See comments in `./registry.go`.
func (e *Engine) runRepoActivityRetry(
	ctx context.Context,
	repoId uuid.I,
	act grpcentities.RepoActivity,
) error {
	tail := ulid.Nil
	for {
		newTail, err := e.runRepoActivity(ctx, repoId, tail, act)
		if err == nil {
			e.lg.Infow(
				"Completed processing repo activity.",
				"repoId", repoId.String(),
			)
			return nil
		}
		if err == context.Canceled {
			return err
		}
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.Canceled {
				return context.Canceled
			}
		}
		tail = newTail

		wait := 20 * time.Second
		afterEvent := "Epoch"
		if tail != ulid.Nil {
			afterEvent = fmt.Sprintf("%v", tail)
		}
		e.lg.Errorw(
			"Will retry watch repo.",
			"err", err,
			"repoId", repoId.String(),
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

func (e *Engine) runRepoActivity(
	ctx context.Context,
	repoId uuid.I,
	tail ulid.I,
	act grpcentities.RepoActivity,
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

	c := pb.NewReposClient(e.conn)
	req := &pb.RepoEventsI{
		Repo:  repoId[:],
		Watch: true,
	}
	if tail != ulid.Nil {
		req.After = tail[:]
	}
	stream, err := c.Events(ctx2, req, e.sysRPCCreds)
	if err != nil {
		return tail, err
	}

	return act.ProcessRepoEvents(ctx2, repoId, tail, stream)
}
