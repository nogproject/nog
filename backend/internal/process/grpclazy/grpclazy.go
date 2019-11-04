package grpclazy

import (
	"context"
	"io"
	"sync"
	"time"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/internal/process/grpcentities"
	"github.com/nogproject/nog/backend/internal/shorteruuid"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const ConfigRetryWatchSignalsWaitSeconds = 15
const ConfigRetryRunTasksIntervalSeconds = 20

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
	Conn                       *grpc.ClientConn
	SysRPCCreds                credentials.PerRPCCredentials
	StreamLimiter              Limiter
	StreamLimiterRegistry      Limiter
	StreamLimiterRepos         Limiter
	StreamLimiterRepoWorkflows Limiter
}

type Engine struct {
	lg          Logger
	names       *shorteruuid.Names
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption

	streamLimiter          Limiter
	streamLimiterRegistry  Limiter
	streamLimiterRepos     Limiter
	streamLimiterWorkflows Limiter

	ctx           context.Context
	mu            sync.Mutex
	wg            sync.WaitGroup
	tasksByEntity taskMap
	readyTasks    []*task
	retryTasks    []*task
	notifyRun     chan struct{}
}

type taskState int

const (
	tsUnspecified taskState = iota
	tsReady
	tsRunning
	tsSleeping
	tsRetry
)

type task struct {
	state      taskState
	signal     bool
	sleepUntil time.Time
	entityId   uuid.I
	tail       ulid.I
	act        lazyActivity
}

// `lazyActivity` is the interface to hide details how to run specific
// aggregate types.  Implementations:
//
//  - `lazyRegistryActivity` in `./registry.go`.
//  - `lazyRepoActivity` in `./repo.go`.
//  - `lazyRepoWorkflowActivity` in `./workflow.go`.
//
type lazyActivity interface {
	run(*Engine, context.Context, *task) (ulid.I, error)
}

type taskMap map[uuid.I][]*task

func NewEngine(lg Logger, cfg *EngineConfig) *Engine {
	return &Engine{
		lg:          lg,
		names:       shorteruuid.NewNogNames(),
		conn:        cfg.Conn,
		sysRPCCreds: grpc.PerRPCCredentials(cfg.SysRPCCreds),

		streamLimiter:          cfg.StreamLimiter,
		streamLimiterRegistry:  cfg.StreamLimiterRegistry,
		streamLimiterRepos:     cfg.StreamLimiterRepos,
		streamLimiterWorkflows: cfg.StreamLimiterRepoWorkflows,

		tasksByEntity: newTaskMap(),
		notifyRun:     make(chan struct{}, 1),
	}
}

func (e *Engine) SetContext(ctx context.Context) {
	e.ctx = ctx
}

func newTaskMap() taskMap {
	return make(map[uuid.I][]*task)
}

func (m taskMap) add(t *task) {
	key := t.entityId
	ts, ok := m[key]
	if !ok {
		ts = make([]*task, 0, 1)
	}
	ts = append(ts, t)
	m[key] = ts
}

func (m taskMap) del(task *task) {
	key := task.entityId
	ts := m[key]

	// If there is only one, delete the slice.
	if len(ts) == 1 && ts[0] == task {
		delete(m, key)
		return
	}

	// If there are several, update the slice.
	for i, t := range m[key] {
		if task == t {
			m[key] = append(ts[:i], ts[i+1:]...)
			return
		}
	}
}

func (e *Engine) addNewTask(entityId uuid.I, act lazyActivity) error {
	t := &task{
		entityId: entityId,
		act:      act,
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tasksByEntity.add(t)
	e.appendReadyLocked(t)
	return nil
}

// `e.mu` must be locked when calling `appendReadyLocked()`.
func (e *Engine) appendReadyLocked(t *task) {
	t.state = tsReady
	e.readyTasks = append(e.readyTasks, t)
	select {
	case e.notifyRun <- struct{}{}:
	default: // non-blocking
	}
}

func (e *Engine) Run() error {
	e.lg.Infow("Started gRPC entities lazy engine.")
	ctx := e.ctx
	defer e.wgWait()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		_ = e.runTasks(ctx)
	}()

	_ = e.watchSignalsRetry(ctx)
	<-ctx.Done()
	return ctx.Err()
}

func (e *Engine) runTasks(ctx context.Context) error {
	ticker := time.NewTicker(
		ConfigRetryRunTasksIntervalSeconds * time.Second,
	)
	defer ticker.Stop()
	for {
		ts := e.takeReadyTasks()
		for _, t := range ts {
			if err := e.startTask(ctx, t); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			e.moveRetryToReady()
		case <-e.notifyRun:
			// Continue loop.
		}
	}
}

func (e *Engine) takeReadyTasks() []*task {
	e.mu.Lock()
	defer e.mu.Unlock()
	ts := e.readyTasks
	e.readyTasks = nil
	return ts
}

func (e *Engine) moveRetryToReady() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.retryTasks == nil {
		return
	}

	now := time.Now()
	ts := e.retryTasks
	ready := make([]*task, 0, len(ts))
	later := make([]*task, 0, len(ts))
	for _, t := range ts {
		if now.After(t.sleepUntil) {
			t.state = tsReady
			ready = append(ready, t)
		} else {
			later = append(later, t)
		}
	}
	e.readyTasks = append(e.readyTasks, ready...)
	e.retryTasks = later
	e.lg.Infow(
		"Activated tasks for retry.",
		"n", len(ready),
	)
}

func (e *Engine) startTask(ctx context.Context, t *task) error {
	limiter := e.whichLimiter(t)
	if limiter != nil {
		if err := limiter.Acquire(ctx, 1); err != nil {
			return err
		}
	}

	if err := e.wgAdd(); err != nil {
		return err
	}
	go func() {
		defer e.wg.Done()
		if limiter != nil {
			defer limiter.Release(1)
		}
		e.runTask(ctx, t)
	}()

	return nil
}

func (e *Engine) whichLimiter(t *task) Limiter {
	switch t.act.(type) {
	case *lazyRegistryActivity:
		if e.streamLimiterRegistry != nil {
			return e.streamLimiterRegistry
		}
	case *lazyRepoActivity:
		if e.streamLimiterRepos != nil {
			return e.streamLimiterRepos
		}
	case *lazyRepoWorkflowActivity:
		if e.streamLimiterWorkflows != nil {
			return e.streamLimiterWorkflows
		}
	}
	return e.streamLimiter
}

func (e *Engine) watchSignalsRetry(ctx context.Context) error {
	for {
		err := e.watchSignals(ctx)
		// If the server context canceled, quit without logging,
		// because the error is very likely `context.Canceled` or
		// `grpc/codes.Canceled`.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default: // non-blocking
		}

		wait := ConfigRetryWatchSignalsWaitSeconds * time.Second
		e.lg.Errorw(
			"Will retry watch aggregates.",
			"err", err,
			"retryIn", wait,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (e *Engine) watchSignals(ctx context.Context) error {
	// `cancel2()` ends the stream when returning before the stream's EOF.
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	// Request signals for aggregate types that the engine runs.
	c := pb.NewLiveBroadcastClient(e.conn)
	req := &pb.AggregateSignalsI{
		SelectAggregates: []pb.AggregateSignalsI_AggregateSelector{
			pb.AggregateSignalsI_AS_REGISTRY,
			pb.AggregateSignalsI_AS_REPO,
			pb.AggregateSignalsI_AS_WORKFLOW,
			pb.AggregateSignalsI_AS_EPHEMERAL_WORKFLOW,
		},
	}
	stream, err := c.AggregateSignals(ctx2, req, e.sysRPCCreds)
	if err != nil {
		return err
	}

	// After the stream has been established, signal all tasks to run.
	// This ensures that tasks run at least once for new events.
	e.signalAllTasks()

	for {
		rsp, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, sig := range rsp.Signals {
			id, err := uuid.FromBytes(sig.EntityId)
			if err != nil {
				return err
			}
			e.signalTasksOfEntity(id)
		}
	}
}

func (e *Engine) signalAllTasks() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ts := range e.tasksByEntity {
		e.signalTasksLocked(ts)
	}
}

func (e *Engine) signalTasksOfEntity(id uuid.I) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ts, ok := e.tasksByEntity[id]
	if !ok {
		return
	}
	e.signalTasksLocked(ts)
}

// `e.mu` must be locked when calling `signalTasksLocked()`.
func (e *Engine) signalTasksLocked(ts []*task) {
	for _, t := range ts {
		switch t.state {
		case tsReady:
			// Already scheduled for running -> do nothing.
		case tsRunning:
			// Set signal to tell `runTask()` to put the task back
			// in the ready queue after the current run completes.
			t.signal = true
		case tsSleeping:
			e.appendReadyLocked(t)
		case tsRetry:
			// Will run again during retry -> do nothing.
		default:
			panic("invalid task state")
		}
	}
}

func (e *Engine) runTask(ctx context.Context, t *task) {
	e.mu.Lock()
	t.state = tsRunning
	e.mu.Unlock()

	newTail, err := t.act.run(e, ctx, t)

	e.mu.Lock()
	defer e.mu.Unlock()
	t.tail = newTail
	switch {
	case err == nil: // Quit activity.
		e.tasksByEntity.del(t)
		return

	case err == io.EOF: // Rerun when there are new events.
		if t.signal {
			t.signal = false
			e.appendReadyLocked(t)
			return
		}
		t.state = tsSleeping
		return

	default: // Schedule for retry.
		t.signal = false
		t.state = tsRetry
		if errAfter, ok := err.(*grpcentities.SilentRetryAfter); ok {
			t.sleepUntil = errAfter.After
		}
		e.retryTasks = append(e.retryTasks, t)
		return
	}
}

func (e *Engine) wgWait() {
	e.mu.Lock()
	e.wg.Wait()
	e.mu.Unlock()
}

func (e *Engine) wgAdd() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	select {
	case <-e.ctx.Done():
		return e.ctx.Err()
	default: // non-blocking
		e.wg.Add(1)
		return nil
	}
}
