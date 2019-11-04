// Package `flock` wraps syscall `flock(2)`.
package flock

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

var ErrNoLock = errors.New("did not acquire lock")

type Flock struct {
	fp *os.File
}

func Open(path string) (*Flock, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Flock{fp}, nil
}

func (lk *Flock) Close() {
	_ = lk.fp.Close()
}

func (lk *Flock) TryLock(ctx context.Context, retryDelay time.Duration) error {
	for {
		err := lk.sysTryLock()
		switch err {
		case nil:
			return nil
		case ErrNoLock: // retry
		default:
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
			// retry
		}
	}
}

func (lk *Flock) Unlock() error {
	return lk.sysUnlock()
}

func (lk *Flock) sysTryLock() error {
	fd := int(lk.fp.Fd())
	err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	switch err {
	case nil:
		return nil
	case syscall.EWOULDBLOCK:
		return ErrNoLock
	default:
		return err
	}
}

func (lk *Flock) sysUnlock() error {
	fd := int(lk.fp.Fd())
	return syscall.Flock(fd, syscall.LOCK_UN)
}
