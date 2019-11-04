package observer6

import (
	"context"
	"io"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

type chainedRepoActivity struct {
	lg         Logger
	chain      DepChainNode
	repoId     uuid.I
	errHandler repoErrorHandler
}

func (a *chainedRepoActivity) depWait(ctx context.Context) error {
	if a.chain.Dep == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.chain.Dep: // proceed
		a.chain.Dep = nil
		return nil
	}
}

func (a *chainedRepoActivity) doContinue() (bool, error) {
	return false, nil
}

func (a *chainedRepoActivity) doQuit() (bool, error) {
	if a.chain.Dep != nil {
		panic("doQuit() called before depWait()")
	}
	if a.chain.Done != nil {
		close(a.chain.Done)
	}
	return true, nil
}

func (a *chainedRepoActivity) doDepWaitQuit(
	ctx context.Context,
) (bool, error) {
	// `doQuit()` closes `a.chain.Done` to unblock the downstream
	// activity, which must happen after the upstream activity has
	// completed.  Ensure that this activity has waited for upstream.
	if err := a.depWait(ctx); err != nil {
		return a.doRetry(ctx, err)
	}
	return a.doQuit()
}

func (a *chainedRepoActivity) doRetry(
	ctx context.Context, err error,
) (bool, error) {
	return false, err
}

func (a *chainedRepoActivity) doHandleErrorContinueOrRetry(
	ctx context.Context, err error,
) (bool, error) {
	if a.errHandler == nil {
		return a.doRetry(ctx, err)
	}

	ok, err2 := a.errHandler.handleRepoError(ctx, a.repoId, err)
	if err2 != nil {
		a.lg.Warnw(
			"Error handler failed.",
			"module", "nogfsostad",
			"repoId", a.repoId.String(),
			"err", err2,
		)
		// Retry unhandled error.
		return a.doRetry(ctx, err)
	}
	if ok {
		return a.doContinue()
	}

	// Retry unhandled error.
	return a.doRetry(ctx, err)
}

func (a *chainedRepoActivity) doHandleErrorQuitOrRetry(
	ctx context.Context, err error,
) (bool, error) {
	if a.errHandler == nil {
		return a.doRetry(ctx, err)
	}

	ok, err2 := a.errHandler.handleRepoError(ctx, a.repoId, err)
	if err2 != nil {
		a.lg.Warnw(
			"Error handler failed.",
			"module", "nogfsostad",
			"repoId", a.repoId.String(),
			"err", err2,
		)
		// Retry unhandled error.
		return a.doRetry(ctx, err)
	}
	if ok {
		return a.doDepWaitQuit(ctx)
	}

	// Retry unhandled error.
	return a.doRetry(ctx, err)
}

type repoStreamEventLoader interface {
	loadRepoEvent(vid ulid.I, ev *pb.RepoEvent) error
}

func loadRepoStreamEventsNoBlock(
	repo repoStreamEventLoader,
	stream pb.Repos_EventsClient,
) error {
	for {
		rsp, err := stream.Recv()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		}

		for _, ev := range rsp.Events {
			vid, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				return err
			}
			if err := repo.loadRepoEvent(vid, ev); err != nil {
				return err
			}
		}

		if rsp.WillBlock {
			return nil
		}
	}
}

type repoStreamEventWatcher interface {
	watchRepoEvent(
		ctx context.Context,
		vid ulid.I,
		ev *pb.RepoEvent,
	) (done bool, err error)
}

func watchRepoStreamEvents(
	a repoStreamEventWatcher,
	ctx context.Context,
	tail ulid.I,
	stream pb.Repos_EventsClient,
) (ulid.I, error) {
	for {
		rsp, err := stream.Recv()
		if err != nil {
			return tail, err
		}

		for _, ev := range rsp.Events {
			vid, err := ulid.ParseBytes(ev.Id)
			if err != nil {
				return tail, err
			}

			done, err := a.watchRepoEvent(ctx, vid, ev)
			switch {
			case err != nil:
				return tail, err
			case done:
				return tail, nil
			}

			tail = vid
		}
	}
}
