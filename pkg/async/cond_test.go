package async

import (
	"context"
	"fmt"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

// ExampleCond_WaitForCondition demonstrates how to use a condition variable to wait for a condition to be met.
//
// In this example, we have a queue of integers that we can obtain in specific batch sizes (5 in this case), and
// we want to wait until the queue has room for the entire batch before enqueuing.
//
// We use a condition variable to wait until the queue has room for the next batch,
// use the cond's broadcast method any time elements are pulled from the queue.
//
// It stops after the consumer has consumed 29 values.
func ExampleCond_WaitForCondition() {
	// Create a context with a timeout
	var ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	var (
		// Create a new condition variable; the zero value is ready to use.
		// Cond protects and synchronizes goroutines that need to respond to changes in the queue's state.
		cond Cond
		// state represents the external state we are synchronizing on
		queue = make([]int, 0, 10)
		// counter is used to generate unique values for the queue
		// it is also protected by cond
		counter int
		// consumed is used to track how many values we have consumed
		// it is also protected by cond
		consumed int
		// we're going to run multiple goroutines, Group will keep track of them for us.
		group errgroup.Group
	)

	// this goroutine is the producer, it will enqueue values into the queue when there is capacity
	group.Go(func() (err error) {
		pprof.Do(ctx, pprof.Labels("cond", "produce"), func(ctx context.Context) {
			for ctx.Err() == nil {
				// the enqueing goroutine
				unlock, waiterr := cond.WaitForCondition(ctx, func() bool {
					// our condition is that the queue, has capacity for the entire next batch
					return len(queue)+2 <= cap(queue)
				})

				if err != nil {
					// this means the context timed out before the condition was met
					unlock() // safe to call regardless of error.
					err = waiterr
					return
				}

				// enqueue 5 values. this is threadsafe because we are protected by the condition's lock
				for i := 0; i < 5 && ctx.Err() == nil; i++ {
					counter++
					queue = append(queue, counter)
				}

				unlock() // safe to call regardless of error.
			}
			err = ctx.Err()
		})
		return err
	})

	// this goroutine is the consumer; it will dequeue values from the queue when it is full
	group.Go(func() (err error) {
		pprof.Do(ctx, pprof.Labels("cond", "consume"), func(ctx context.Context) {
			for ctx.Err() == nil {
				unlock, waiterr := cond.WaitForCondition(ctx, func() bool {
					// our condition is that the queue has values to dequeue
					return len(queue) > 0
				})
				if waiterr != nil {
					err = waiterr
					unlock() // safe to call regardless of error.
					return
				}
				if consumed >= 29 {
					cancel()
					return
				}

				consumed++
				queue = append(make([]int, 0, 10),
					queue[1:]...) // we have to append/make because otherwise the cap decreases by 1 each time we do this.
				unlock()
				Sleep(ctx, 10*time.Millisecond)
			}
			err = ctx.Err()
		})

		return err
	})

	err := group.Wait() // wait for all goroutines to exit
	fmt.Println(err, consumed)

	// Output:
	// context canceled 29
}

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
	t.Run("returns immediately, without error if the lock is free and the condition is met", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		unlock, err := cond.WaitForCondition(ctx, func() bool {
			return true
		})
		assert.Nil(t, err)
		assert.False(t, cond.Mu.TryLock()) // it was able to lock m

		// the returned function unlocks Mu
		unlock()
		assert.True(t, cond.Mu.TryLock())
	})

	t.Run("blocks until lock is free if condition is met", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cond.Mu.Lock() // lock it so the condition isn't evaluated until we unlock it
		waitedForUnlock := atomic.Bool{}
		go func() {
			time.Sleep(100 * time.Millisecond)
			cond.Mu.Unlock()
			waitedForUnlock.Store(true)
			cond.Broadcast()
		}()
		unlock, err := cond.WaitForCondition(ctx, func() bool {
			return true
		})
		assert.True(t, waitedForUnlock.Load())

		assert.Nil(t, err)
		assert.False(t, cond.Mu.TryLock()) // it is locked
		// the returned function unlocks Mu
		unlock()
		assert.True(t, cond.Mu.TryLock())
	})

	t.Run("blocks until condition is met", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var i = atomic.Int32{}
		unlock, err := cond.WaitForCondition(ctx, func() bool {
			go func() {
				i.Add(1)
				cond.Broadcast() // the condition has changed
			}()
			return i.Load() > 5
		})

		assert.Nil(t, err)
		assert.False(t, cond.Mu.TryLock()) // it is locked
		// the returned function unlocks Mu
		unlock()
		assert.True(t, cond.Mu.TryLock())
	})

	t.Run("respects context expiry; even if locked", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cond.Mu.Lock()
		go func() {
			time.Sleep(50 * time.Millisecond) // just a breath so the other goroutine goes first
			cancel()
		}()

		fn, err := cond.WaitForCondition(ctx, func() bool {
			return true
		})
		defer fn()
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("respects context expiry; if lock is free", func(t *testing.T) {
		cond := Cond{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(50 * time.Millisecond) // just a breath so the other goroutine goes first
			cancel()
		}()

		fn, err := cond.WaitForCondition(ctx, func() bool {
			return false
		})
		defer fn()
		assert.Equal(t, context.Canceled, err)
	})
}

func BenchmarkCond(b *testing.B) {
	wait := time.Millisecond * 10
	b.Run("one broadcasts; one wait", func(b *testing.B) {
		b.ReportAllocs()
		var cond Cond
		start := time.Now()

		for i := 0; i < b.N; i++ {
			var g sync.WaitGroup
			var ctx, cancel = context.WithCancel(context.Background())
			g.Add(2)
			go func() {
				time.Sleep(wait) // just a breath so the other goroutine goes first
				cond.Broadcast()
				g.Done()
			}()
			go func() {
				err := cond.Wait(ctx)
				cancel()
				g.Done()
				assert.Nil(b, err)
			}()
			g.Wait()
			cancel()
		}

		correctedDuration := time.Since(start) - wait*time.Duration(b.N)
		b.ReportMetric(float64(correctedDuration.Milliseconds())/float64(b.N), "ms_corrected/op")
	})

	b.Run("one broadcasts; 10 waiters", func(b *testing.B) {
		b.ReportAllocs()
		start := time.Now()
		var cond Cond
		for i := 0; i < b.N; i++ {
			g, ctx := errgroup.WithContext(context.Background())
			ctx, cancel := context.WithCancel(ctx)

			for i := 0; i < 10; i++ {
				g.Go(func() error {
					err := cond.Wait(ctx)
					return err
				})
			}

			go func() {
				time.Sleep(wait) // just a breath so the other goroutine goes first
				cond.Broadcast()
			}()
			err := g.Wait()
			assert.Nil(b, err)
			cancel()
		}
		correctedDuration := time.Since(start) - wait*time.Duration(b.N)
		b.ReportMetric(float64(correctedDuration.Milliseconds())/float64(b.N), "ms_corrected/op")
	})
}
