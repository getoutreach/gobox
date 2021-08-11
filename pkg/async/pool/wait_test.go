package pool_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/async/pool"
	"gotest.tools/v3/assert"
)

func TestWithWait(t *testing.T) {
	var (
		counter int32 = 0
	)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	p := pool.New(ctx,
		pool.ConstantSize(2),
	)
	defer p.Close()

	scheduler, wait := pool.WithWait(p)

	for i := 0; i < 2; i++ {
		func() {
			scheduler.Schedule(ctx, async.Func(func(ctx context.Context) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				atomic.AddInt32(&counter, 1)
				return nil
			}))
		}()
	}
	wait()
	assert.Equal(t, int32(2), counter)
}
