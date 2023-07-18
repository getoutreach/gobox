package async

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

func TestCond(t *testing.T) {
	t.Run("broadcast wakes up waiter", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		go func() {
			time.Sleep(50 * time.Millisecond) // just a breath so the other goroutine goes first
			cond.Broadcast()
		}()

		err := cond.Wait(ctx)
		assert.Nil(t, err)
	})

	t.Run("can safely interleave broadcasts", func(t *testing.T) {
		cond := Cond{}
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		for j := 0; j < 10; j++ {
			start := make(chan struct{})
			g := errgroup.Group{}
			g.Go(func() error {
				return cond.Wait(ctx)
			})
			for i := 0; i < 10; i++ {
				g.Go(func() error {
					<-start
					cond.Broadcast()
					return nil
				})
			}
			g.Go(func() error {
				time.Sleep(10 * time.Millisecond) // just a breath so the other goroutine goes first
				close(start)
				return nil
			})
			err := g.Wait()
			assert.Nil(t, err)
		}
	})

	t.Run("broadcast wakes all waiters", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		g := errgroup.Group{}
		// start everyone waiting
		for i := 0; i < 10; i++ {
			g.Go(func() error {
				return cond.Wait(ctx)
			})
		}

		// wake em all up
		g.Go(func() error {
			time.Sleep(10 * time.Millisecond) // just a breath so the other goroutine goes first
			cond.Broadcast()
			return nil
		})

		err := g.Wait()
		assert.Nil(t, err)
	})

	t.Run("waiter exits on context cancel", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(50 * time.Millisecond) // just a breath so the other goroutine goes first
			cancel()
		}()

		err := cond.Wait(ctx)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestCond_WaitForCondition(t *testing.T) {
	cond := Cond{}
	t.Run("returns immediately, without error if the lock is free and the condition is met", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mu := &sync.Mutex{}
		unlock, err := cond.WaitForCondition(ctx, mu, func() bool {
			return true
		})
		assert.Nil(t, err)
		assert.False(t, mu.TryLock()) // it was able to lock m

		// the returned function unlocks mu
		unlock()
		assert.True(t, mu.TryLock())
	})

	t.Run("blocks until lock is free if condition is met", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mu := &sync.Mutex{}
		mu.Lock() // lock it so the condition isn't evaluated until we unlock it
		waitedForUnlock := atomic.Bool{}
		go func() {
			time.Sleep(100 * time.Millisecond)
			mu.Unlock()
			waitedForUnlock.Store(true)
		}()
		unlock, err := cond.WaitForCondition(ctx, mu, func() bool {
			return true
		})
		assert.True(t, waitedForUnlock.Load())

		assert.Nil(t, err)
		assert.False(t, mu.TryLock()) // it is locked
		// the returned function unlocks mu
		unlock()
		assert.True(t, mu.TryLock())
	})

	t.Run("blocks until condition is met", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mu := &sync.Mutex{}

		var i = atomic.Int32{}
		unlock, err := cond.WaitForCondition(ctx, mu, func() bool {
			go func() {
				i.Add(1)
				cond.Broadcast()
			}()
			return i.Load() > 5
		})

		assert.Nil(t, err)
		assert.False(t, mu.TryLock()) // it is locked
		// the returned function unlocks mu
		unlock()
		assert.True(t, mu.TryLock())
	})

	t.Run("respects context expiry; even if locked", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mu := &sync.Mutex{}
		mu.Lock()

		go func() {
			time.Sleep(50 * time.Millisecond) // just a breath so the other goroutine goes first
			cancel()
		}()

		fn, err := cond.WaitForCondition(ctx, mu, func() bool {
			return true
		})
		defer fn()
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("respects context expiry; if lock is free", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mu := &sync.Mutex{}

		go func() {
			time.Sleep(50 * time.Millisecond) // just a breath so the other goroutine goes first
			cancel()
		}()

		fn, err := cond.WaitForCondition(ctx, mu, func() bool {
			return false
		})
		defer fn()
		assert.Equal(t, context.Canceled, err)
	})
}
