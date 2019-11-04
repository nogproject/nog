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
func (e *Engine) StartRepoWorkflowActivity(
	repoId uuid.I,
	workflowId uuid.I,
	act grpcentities.RepoWorkflowActivity,
) error {
	if err := e.wgAdd(); err != nil {
		return err
	}

	go func() {
		defer e.wg.Done()
		_ = e.runRepoWorkflowActivityRetry(
			e.ctx, repoId, workflowId, act,
		)
	}()

	return nil
}

// See comments in `./registry.go`.
func (e *Engine) runRepoWorkflowActivityRetry(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	act grpcentities.RepoWorkflowActivity,
) error {
	tail := ulid.Nil
	for {
		newTail, err := e.runRepoWorkflowActivity(
			ctx, repoId, workflowId, tail, act,
		)
		if err == nil {
			e.lg.Infow(
				"Completed processing repo workflow activity.",
				"repoId", repoId.String(),
				"workflowId", workflowId.String(),
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
			"Will retry watch repo workflow.",
			"err", err,
			"repoId", repoId.String(),
			"workflowId", workflowId.String(),
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

func (e *Engine) runRepoWorkflowActivity(
	ctx context.Context,
	repoId uuid.I,
	workflowId uuid.I,
	tail ulid.I,
	act grpcentities.RepoWorkflowActivity,
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
	req := &pb.RepoWorkflowEventsI{
		Repo:     repoId[:],
		Workflow: workflowId[:],
		Watch:    true,
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
