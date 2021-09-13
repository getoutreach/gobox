package pool

import (
	"context"
	"sync"

	"github.com/getoutreach/gobox/pkg/async"
)

// Wait is a scheduler that allow you to wait until all scheduled tasks are processed or failed to enqueue.
// It can be used when you need to wait for all items from one batch are processed
type Wait struct {
	Scheduler Scheduler
	sync.WaitGroup
}

func (w *Wait) Schedule(ctx context.Context, r async.Runner) error {
	w.Add(1)
	return w.Scheduler.Schedule(ctx, async.Func(func(ctx context.Context) error {
		defer w.Done()
		return r.Run(ctx)
	}))
}

func WithWait(s Scheduler) (scheduler Scheduler, wait func()) {
	w := &Wait{Scheduler: s}
	return w, func() {
		w.Wait()
	}
}
