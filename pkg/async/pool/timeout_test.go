package pool_test

import (
	"context"
	"testing"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
	"github.com/getoutreach/gobox/pkg/async/pool"
	"gotest.tools/v3/assert"
)

func TestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p := pool.New(ctx,
		pool.ConstantSize(1),
	)

	startedAt := time.Now()

	scheduler := pool.WithTimeout(5*time.Millisecond, p)

	scheduler, wait := pool.WithWait(scheduler)

	var err error

	scheduler.Schedule(ctx, async.Func(func(ctx context.Context) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(10 * time.Millisecond)
		return nil
	}))
	scheduler.Schedule(ctx, async.Func(func(ctx context.Context) error {
		if ctx.Err() != nil {
			err = ctx.Err()
			return ctx.Err()
		}
		return nil
	}))

	wait()

	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Assert(t, WithinDuration(time.Now(), startedAt, 20*time.Millisecond))
}
