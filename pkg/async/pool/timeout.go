package pool

import (
	"context"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
)

// WithTimeout creates enqueuer that cancel enqueueing after given timeout
func WithTimeout(timeout time.Duration, scheduler Scheduler) Scheduler {
	return SchedulerFunc(func(ctx context.Context, r async.Runner) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		return scheduler.Schedule(ctx, async.Func(func(context.Context) error {
			defer cancel()
			return r.Run(ctx)
		}))
	})
}
