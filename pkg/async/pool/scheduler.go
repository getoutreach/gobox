package pool

import (
	"context"

	"github.com/getoutreach/gobox/pkg/async"
)

type SchedulerFunc func(ctx context.Context, r async.Runner) error

func (sf SchedulerFunc) Schedule(ctx context.Context, r async.Runner) error {
	return sf(ctx, r)
}

type Scheduler interface {
	// Schedule task for processing in the pool
	Schedule(ctx context.Context, r async.Runner) error
}
