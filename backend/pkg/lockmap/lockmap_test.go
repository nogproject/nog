package lockmap_test

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/nogproject/nog/backend/pkg/lockmap"
)

const maxSleep = 1 * time.Millisecond

func HammerLock(l *lockmap.L, loops int) {
	const key = "foo"
	for i := 0; i < loops; i++ {
		if l.Lock(context.Background(), key) != nil {
			panic("lock failed")
		}
		r := rand.Int63n(int64(maxSleep / time.Nanosecond))
		time.Sleep(time.Duration(r) * time.Nanosecond)
		l.Unlock(key)
	}
}

func TestLock(t *testing.T) {
	var l lockmap.L

	n := lockmap.MaxWaiters + 1
	loops := 10000 / n
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			HammerLock(&l, loops)
		}()
	}
	wg.Wait()
}

func TestLockTimeout(t *testing.T) {
	var l lockmap.L

	ctx := context.Background()
	lock := func(k string) bool {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()
		return l.Lock(ctx, k) == nil
	}

	tries := []bool{}
	tries = append(tries, lock("foo"))
	tries = append(tries, lock("bar"))
	tries = append(tries, lock("foo"))
	tries = append(tries, lock("bar"))
	l.Unlock("foo")
	l.Unlock("bar")
	tries = append(tries, lock("foo"))
	tries = append(tries, lock("bar"))

	want := []bool{true, true, false, false, true, true}
	for i := range tries {
		if tries[i] != want[i] {
			t.Errorf(
				"tries[%d]: got %t, want %t",
				i, tries[i], want[i],
			)
		}
	}
}
