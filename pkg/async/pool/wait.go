// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides wait capabilities for the async pool which allow delaying processing

package pool

import (
	"context"
	"sync"

	"github.com/getoutreach/gobox/pkg/async"
)

// Wait is a scheduler that allows you to wait until all scheduled tasks are
// processed or have failed to enqueue. It can be used when you need to wait
// for all items from one batch to be processed.
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

// WithWait wraps a scheduler and returns a new scheduler and a function
// that blocks until all scheduled tasks are processed or have failed to
// enqueue.
//
// Deprecated: This library is being deprecated in favor of using
// https://pkg.go.dev/github.com/sourcegraph/conc/pool instead. Use
// (*Pool).Wait() instead. For more information, see the README:
// https://github.com/getoutreach/gobox/tree/main/pkg/async/pool/README.md
func WithWait(s Scheduler) (scheduler Scheduler, wait func()) {
	w := &Wait{Scheduler: s}
	return w, func() {
		w.Wait()
	}
}
