package pool_test

import (
	"context"
	"fmt"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/async/pool"
)

func ExamplePool() {
	var (
		concurrency = 5
		items       = 10
		sleepFor    = 5 * time.Millisecond
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Spawn pool of workers
	p := pool.New(ctx,
		pool.ConstantSize(concurrency),
		pool.ResizeEvery(5*time.Minute),
		pool.BufferLength(256),
		pool.WaitWhenFull,
	)
	defer p.Close()
	// Wrap it with timeout for schedule
	scheduler := pool.WithTimeout(5*time.Millisecond, p)

	// Lets wait for all scheduled items from this point
	scheduler, wait := pool.WithWait(scheduler)

	output := make(chan string, items)
	now := time.Now()

	for i := 0; i < items; i++ {
		func(i int) {
			// All input and output is captured by closure
			scheduler.Schedule(ctx, async.Func(func(ctx context.Context) error {
				// It is very important to check the context error:
				// - Given context might be done
				// - Underlying buffered channel is full
				// - Pool is in shutdown phase
				if ctx.Err() != nil {
					return ctx.Err()
				}
				time.Sleep(sleepFor)
				batchN := (time.Since(now) / (sleepFor))
				output <- fmt.Sprintf("task_%d_%d", batchN, i)
				// returned error is logged but not returned by Schedule function
				return nil
			}))
		}(i)
	}
	wait()
	close(output)
	for s := range output {
		fmt.Println(s)
	}
	// Not using unordered output since it not deterministic
	// task_1_3
	// task_1_4
	// task_1_0
	// task_1_1
	// task_1_2
	// task_2_6
	// task_2_9
	// task_2_5
	// task_2_7
	// task_2_8
}
