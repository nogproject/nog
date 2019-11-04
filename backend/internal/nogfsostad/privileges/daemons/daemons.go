package daemons

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/nogproject/nog/backend/internal/udopb"
	"google.golang.org/grpc"
)

var cfgDefault = Config{
	IdleLifetime: 60 * time.Second,
	ReapInterval: 1 * time.Second,
}

var ErrShutdown = errors.New("shutdown")
var ErrDaemonNotDirectChild = errors.New(
	"user daemon is not a direct child process",
)

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Infow(msg string, kv ...interface{})
}

type Config struct {
	IdleLifetime time.Duration
	ReapInterval time.Duration
}

type Daemons struct {
	cfg        *Config
	mu         sync.Mutex
	lg         Logger
	daemons    map[Key]*Daemon
	zombies    []*Daemon
	isShutdown bool
}

type Key string

type Egg struct {
	Cmd       *exec.Cmd
	Conn      *grpc.ClientConn
	IsManaged bool
}

type Daemon struct {
	// `idle` is the Unix time when the daemon became idle, i.e. `refs`
	// became zero after the daemon was active.
	idle int64 // atomic
	// `refs` is an atomic ref count.  It is only increased while holding
	// the `Daemons.mu` lock.  But it may be decreased without holding the
	// lock.
	refs      int32 // atomic
	cmd       *exec.Cmd
	conn      *grpc.ClientConn
	isManaged bool
}

func (d *Daemon) Conn() *grpc.ClientConn { return d.conn }

func New(lg Logger) *Daemons {
	return &Daemons{
		// We can make `cfg` a parameter of `New()` if we want to allow
		// the caller to control details.
		cfg:     &cfgDefault,
		lg:      lg,
		daemons: make(map[Key]*Daemon),
	}
}

func (ds *Daemons) Run(ctx context.Context) error {
	for {
		select {
		case <-time.After(ds.cfg.ReapInterval):
			ds.reapIdle(ctx)
		case <-ctx.Done():
			ds.shutdown()
			return ctx.Err()
		}
	}
}

func (ds *Daemons) reapIdle(ctx context.Context) {
	for _, d := range ds.takeUnusedZombies() {
		// Ignore errors, because zombies already failed when they
		// became a zobie.
		ctx2, cancel2 := context.WithTimeout(ctx, 10*time.Millisecond)
		_ = d.quit(ctx2)
		cancel2()
		ds.lg.Infow("Reaped zombie.")
	}

	cutoff := time.Now().Unix() - int64(ds.cfg.IdleLifetime.Seconds())
	for _, k := range ds.getKeysIdleSince(cutoff) {
		d, ok := ds.takeIfUnused(k)
		if !ok {
			continue
		}

		ctx2, cancel2 := context.WithTimeout(ctx, 30*time.Second)
		err := d.quit(ctx2)
		cancel2()
		if err != nil {
			ds.lg.Errorw(
				"Failed to reap daemon.",
				"key", k,
				"err", err,
			)
		} else {
			ds.lg.Infow(
				"Reaped daemon.",
				"key", k,
			)
		}

		select {
		default: // non-blocking
		case <-ctx.Done():
			return
		}
	}
}

func (ds *Daemons) getKeysIdleSince(cutoff int64) []Key {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	keys := make([]Key, 0, len(ds.daemons))
	for k, d := range ds.daemons {
		if atomic.LoadInt32(&d.refs) > 0 {
			continue
		}
		if atomic.LoadInt64(&d.idle) > cutoff {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (ds *Daemons) shutdown() {
	// After `setIsShutdown()`, no other goroutine uses `ds.daemons`.
	ds.setIsShutdown()

	// Block until all daemons stopped.  `main()` has a separate timeout
	// that will force a shutdown if this function does not return.
	ctx := context.Background()

	var wg sync.WaitGroup
	for {
		// Ignore errors, because zombies already failed when they
		// became a zobie.
		usedZombies := make([]*Daemon, 0, len(ds.zombies))
		for _, d := range ds.zombies {
			if atomic.LoadInt32(&d.refs) > 0 {
				usedZombies = append(usedZombies, d)
				continue
			}
			wg.Add(1)
			go func(d *Daemon) {
				_ = d.quit(ctx)
				wg.Done()
			}(d)
		}
		ds.zombies = usedZombies

		// Report errors, because daemons should exit cleanly.
		for k, d := range ds.daemons {
			if atomic.LoadInt32(&d.refs) > 0 {
				continue
			}
			delete(ds.daemons, k)
			wg.Add(1)
			go func(d *Daemon) {
				if err := d.quit(ctx); err != nil {
					ds.lg.Errorw(
						"Failed to shutdown daemon.",
						"err", err,
					)
				}
				wg.Done()
			}(d)
		}

		if len(ds.zombies) == 0 && len(ds.daemons) == 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	wg.Wait()
}

func (d *Daemon) ref() int32 {
	return atomic.AddInt32(&d.refs, 1)
}

func (d *Daemon) unref() int32 {
	return atomic.AddInt32(&d.refs, -1)
}

func (d *Daemon) setIdle() {
	atomic.StoreInt64(&d.idle, time.Now().Unix())
}

func (d *Daemon) Release() {
	if d.unref() == 0 {
		d.setIdle()
	}
}

func (d *Daemon) quit(ctx context.Context) error {
	if d.refs != 0 {
		panic("refs != 0")
	}
	// `quit()` must be called only once.  Even if it returns an error, it
	// tries to complete in the background to avoid zombies.
	if d.cmd == nil && d.conn == nil {
		panic("already closed")
	}

	errCh := make(chan error, 1)
	if d.isManaged {
		go terminateDaemon(d.conn, d.cmd, errCh)
	} else {
		errCh <- d.conn.Close()
	}
	d.conn = nil
	d.cmd = nil
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func terminateDaemon(
	conn *grpc.ClientConn, cmd *exec.Cmd, errCh chan<- error,
) {
	// `daemonClose()` runs in the background without timeout.
	ctx := context.Background()
	// We should perhaps analyze the error and retry `Terminate()` if the
	// error is temporary.
	c := pb.NewUdoDaemonClient(conn)
	_, err := c.Terminate(ctx, &pb.TerminateI{})
	if err2 := conn.Close(); err == nil {
		err = err2
	}
	if cmd != nil {
		if err2 := cmd.Wait(); err == nil {
			err = err2
		}
	}
	errCh <- err
}

func (ds *Daemons) setIsShutdown() {
	ds.mu.Lock()
	ds.isShutdown = true
	ds.mu.Unlock()
}

// `takeIfUnused()` removes and returns the daemon for `key` if its ref count
// is 0.
func (ds *Daemons) takeIfUnused(key Key) (*Daemon, bool) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.isShutdown {
		return nil, false
	}
	d, ok := ds.daemons[key]
	if !ok {
		return nil, false
	}
	if atomic.LoadInt32(&d.refs) > 0 {
		return nil, false
	}
	delete(ds.daemons, key)
	return d, true
}

func (ds *Daemons) takeUnusedZombies() []*Daemon {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.isShutdown {
		return nil
	}

	used := make([]*Daemon, 0, len(ds.zombies))
	unused := make([]*Daemon, 0, len(ds.zombies))
	for _, d := range ds.zombies {
		if atomic.LoadInt32(&d.refs) > 0 {
			used = append(used, d)
		} else {
			unused = append(unused, d)
		}
	}

	ds.zombies = used
	return unused
}

func (ds *Daemons) get(key Key) (*Daemon, bool) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.isShutdown {
		return nil, false
	}
	d, ok := ds.daemons[key]
	if ok {
		_ = d.ref()
	}
	return d, ok
}

func (ds *Daemons) setdefault(key Key, d2 *Daemon) *Daemon {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.isShutdown {
		return nil
	}
	d, ok := ds.daemons[key]
	if ok {
		_ = d.ref()
		return d
	}
	_ = d2.ref()
	ds.daemons[key] = d2
	return d2
}

// `setZombie()` atomically checks whether the daemon is still active and if
// so, moves it to the zombie list.
func (ds *Daemons) setZombie(key Key, d *Daemon) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.isShutdown {
		return
	}
	dActive, ok := ds.daemons[key]
	if !ok {
		return
	}
	if d != dActive {
		return
	}
	delete(ds.daemons, key)
	ds.zombies = append(ds.zombies, d)
}

func (ds *Daemons) Start(
	ctx context.Context,
	key Key,
	start func(context.Context) (*Egg, error),
) (*Daemon, error) {
	d, ok := ds.get(key)
	if ok {
		// If the daemon is healthy, return it.  Otherwise, change it
		// to a zombie and start a new daemon.
		err := d.check(ctx)
		if err == nil {
			return d, nil
		}
		ds.setZombie(key, d)
		d.unref()
		d = nil
	}

	egg, err := start(ctx)
	if err != nil {
		return nil, err
	}
	d2 := &Daemon{
		cmd:       egg.Cmd,
		conn:      egg.Conn,
		isManaged: egg.IsManaged,
	}
	defer func() {
		if d2 == nil {
			return
		}
		// If `d2` has not been stored in `ds.daemons`, terminate it.
		ctx2, cancel2 := context.WithTimeout(ctx, 50*time.Millisecond)
		_ = d2.quit(ctx2)
		cancel2()
	}()
	if err := d2.check(ctx); err != nil {
		return nil, err
	}

	d = ds.setdefault(key, d2)
	if d == d2 {
		// If `d2` has been stored, do not defer terminate it.
		d2 = nil
	}
	if d == nil {
		return nil, ErrShutdown
	}

	return d, nil
}

func (d *Daemon) check(ctx context.Context) error {
	c := pb.NewUdoDaemonClient(d.conn)
	pingO, err := c.Ping(ctx, &pb.PingI{})
	if err != nil {
		return err
	}

	// If the daemon runs as a command, it should be a direct child, so
	// that `cmd.Wait()` will work in `terminateDaemon()`.
	if d.cmd != nil && int(pingO.Ppid) != os.Getpid() {
		return ErrDaemonNotDirectChild
	}

	return nil
}
