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

type lazyRepoActivity struct {
	act grpcentities.RepoActivity
}

func (e *Engine) StartRepoActivity(
	repoId uuid.I, act grpcentities.RepoActivity,
) error {
	return e.addNewTask(repoId, &lazyRepoActivity{act: act})
}

func (act *lazyRepoActivity) run(
	e *Engine, ctx context.Context, t *task,
) (ulid.I, error) {
	repoId := t.entityId
	tail := t.tail
	return e.runRepo(ctx, repoId, tail, act.act)
}

func (e *Engine) runRepo(
	ctx context.Context,
	repoId uuid.I,
	tail ulid.I,
	act grpcentities.RepoActivity,
) (ulid.I, error) {
	newTail, err := e.runRepoStreamNoBlock(ctx, repoId, tail, act)
	switch {
	case err == nil:
		e.lg.Infow(
			"Completed processing repo activity.",
			"repoId", repoId.String(),
		)
	case err == context.Canceled:
		// Handle cancel silently.
	case err == io.EOF:
		if newTail != tail {
			e.lg.Infow(
				"Repo activity progressed.",
				"repoId", repoId.String(),
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
			"Will retry repo activity.",
			"err", err,
			"repoId", repoId.String(),
			"afterEvent", afterEvent,
		)
	}
	return newTail, err
}

func (e *Engine) runRepoStreamNoBlock(
	ctx context.Context,
	repoId uuid.I,
	tail ulid.I,
	act grpcentities.RepoActivity,
) (ulid.I, error) {
	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	c := pb.NewReposClient(e.conn)
	req := &pb.RepoEventsI{
		Repo:  repoId[:],
		Watch: false,
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
