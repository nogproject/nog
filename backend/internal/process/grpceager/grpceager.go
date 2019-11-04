package grpceager

import (
	"context"
	"sync"

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
}

// `Engine` keeps entity activities alive until completion.  It
// immediately restarts an activity if its event stream gets interrupted.
type Engine struct {
	lg          Logger
	conn        *grpc.ClientConn
	sysRPCCreds grpc.CallOption

	streamLimiter Limiter

	mu  sync.Mutex
	wg  sync.WaitGroup
	ctx context.Context
}

func NewEngine(lg Logger, cfg *EngineConfig) *Engine {
	return &Engine{
		lg:            lg,
		conn:          cfg.Conn,
		sysRPCCreds:   grpc.PerRPCCredentials(cfg.SysRPCCreds),
		streamLimiter: cfg.StreamLimiter,
	}
}

func (e *Engine) SetContext(ctx context.Context) {
	e.ctx = ctx
}

func (e *Engine) Run() error {
	e.lg.Infow("Started gRPC entities eager engine.")
	<-e.ctx.Done()
	e.wgWait()
	return e.ctx.Err()
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
