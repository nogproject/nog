package lockmap

import (
	"context"
	"errors"
	"sync"
)

const MaxWaiters = 100

var ErrTooManyWaiters = errors.New("too many waiters")

type waiter struct {
	alive bool
	ready chan<- struct{}
}

type L struct {
	mu sync.Mutex
	// - not in map: unlocked.
	// - `nil` in map: locked without waiters.
	// - chan in map: locked with waiters.
	locks map[string]chan *waiter
}

func (l *L) Lock(ctx context.Context, key string) error {
	l.mu.Lock()

	if l.locks == nil {
		l.locks = make(map[string]chan *waiter)
	}

	waiters, ok := l.locks[key]
	if !ok {
		// Take the lock.
		l.locks[key] = nil
		l.mu.Unlock()
		return nil
	}

	if waiters == nil {
		// First waiter, init the queue.
		waiters = make(chan *waiter, MaxWaiters)
		l.locks[key] = waiters
	}

	ready := make(chan struct{})
	w := &waiter{alive: true, ready: ready}
	select {
	default: // non-blocking
		l.mu.Unlock()
		return ErrTooManyWaiters
	case waiters <- w:
	}

	l.mu.Unlock()

	select {
	case <-ctx.Done():
		err := ctx.Err()
		l.mu.Lock()
		select {
		case <-ready:
			// Got the lock after cancel.  Pretend that we did not
			// notice the cancel to avoid complicated coordination
			// with Unlock().
			err = nil
		default:
			w.alive = false
		}
		l.mu.Unlock()
		return err

	case <-ready:
		return nil
	}
}

func (l *L) Unlock(key string) {
	l.mu.Lock()

	waiters, ok := l.locks[key]
	if !ok {
		l.mu.Unlock()
		panic("bad unlock")
	}

	if waiters == nil {
		// No waiters.
		delete(l.locks, key)
		l.mu.Unlock()
		return
	}

	for {
		select {
		case w := <-waiters:
			if w.alive {
				close(w.ready)
				l.mu.Unlock()
				return
			}
		default: // no waiters
			close(waiters)
			delete(l.locks, key)
			l.mu.Unlock()
			return
		}
	}
}
