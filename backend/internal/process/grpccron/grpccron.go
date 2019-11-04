package grpccron

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
}

type Limiter interface {
	Acquire(ctx context.Context, n int64) error
	Release(n int64)
}

type EngineConfig struct {
	Conn          *grpc.ClientConn
	SysRPCCreds   credentials.PerRPCCredentials
	StreamLimiter Limiter
	CronInterval  time.Duration
}

type Engine struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption

	interval      time.Duration
	streamLimiter Limiter

	ctx   context.Context
	mu    sync.Mutex
	wg    sync.WaitGroup
	repos map[*repoActivity]struct{}
}

type repoActivity struct {
	repoId uuid.I
	tail   ulid.I
	act    grpcentities.RepoActivity
	done   chan bool
}

func NewCronEngine(lg Logger, cfg *EngineConfig) *Engine {
	return &Engine{
		lg:            lg,
		conn:          cfg.Conn,
		sysRPCCreds:   grpc.PerRPCCredentials(cfg.SysRPCCreds),
		interval:      cfg.CronInterval,
		streamLimiter: cfg.StreamLimiter,
		repos:         make(map[*repoActivity]struct{}),
	}
}

func (e *Engine) SetContext(ctx context.Context) {
	e.ctx = ctx
}

func (e *Engine) Run() error {
	e.lg.Infow(
		"Started gRPC entities cron engine.",
		"interval", e.interval,
	)
	ticker := time.NewTicker(e.interval)
	for {
		select {
		case <-e.ctx.Done():
			e.wg.Wait()
			return e.ctx.Err()
		case <-ticker.C: // run loop body.
		}
		if err := e.runRepos(e.ctx); err != nil {
			e.wg.Wait()
			return err
		}
	}
}

func (e *Engine) StartRepoActivity(
	repoId uuid.I, act grpcentities.RepoActivity,
) error {
	e.addRepo(&repoActivity{
		repoId: repoId,
		act:    act,
	})
	return nil
}

func (e *Engine) addRepo(r *repoActivity) {
	e.mu.Lock()
	e.repos[r] = struct{}{}
	e.mu.Unlock()
}

func (e *Engine) deleteRepo(r *repoActivity) {
	e.mu.Lock()
	delete(e.repos, r)
	e.mu.Unlock()
}

func (e *Engine) getRepos() []*repoActivity {
	e.mu.Lock()
	defer e.mu.Unlock()
	rs := make([]*repoActivity, 0, len(e.repos))
	for r, _ := range e.repos {
		rs = append(rs, r)
	}
	return rs
}

func (e *Engine) runRepos(ctx context.Context) error {
	repos := e.getRepos()
	for _, r := range repos {
		if err := e.scheduleRepo(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) scheduleRepo(ctx context.Context, r *repoActivity) error {
	// If there is a goroutine, check whether it has completed.
	if r.done != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case quit := <-r.done:
			if quit {
				e.deleteRepo(r)
				return nil
			}
			r.done = nil
		default: // Do not wait for goroutine.
			return nil
		}
	}

	limiter := e.streamLimiter
	if limiter != nil {
		if err := limiter.Acquire(ctx, 1); err != nil {
			return err
		}
	}

	r.done = make(chan bool, 1)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		if limiter != nil {
			defer limiter.Release(1)
		}
		_ = e.runRepo(ctx, r)
	}()

	return nil
}

func (e *Engine) runRepo(ctx context.Context, r *repoActivity) error {
	newTail, err := e.runRepoStreamNoBlock(ctx, r.repoId, r.tail, r.act)
	switch {
	case err == nil:
		e.lg.Infow(
			"Completed processing repo activity.",
			"repoId", r.repoId.String(),
		)
		r.done <- true
		close(r.done)
		return nil

	case err == context.Canceled:
		r.done <- false
		close(r.done)
		return err

	case err == io.EOF:
		if newTail != r.tail {
			r.tail = newTail
			e.lg.Infow(
				"Repo activity progressed.",
				"repoId", r.repoId.String(),
				"vid", r.tail.String(),
			)
		}
		r.done <- false
		close(r.done)
		return nil

	default:
		afterEvent := "Epoch"
		if newTail != ulid.Nil {
			afterEvent = fmt.Sprintf("%v", newTail)
		}
		e.lg.Errorw(
			"Will retry running repo during next cron interval.",
			"err", err,
			"repoId", r.repoId.String(),
			"afterEvent", afterEvent,
			"cronInterval", e.interval,
		)
		r.tail = newTail
		r.done <- false
		close(r.done)
		return nil
	}
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
