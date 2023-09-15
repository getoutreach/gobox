// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides timeout capabilities for the async pool

package pool

import (
	"context"
	"time"

	"github.com/getoutreach/gobox/pkg/async"
)

// WithTimeout creates enqueuer that cancel enqueueing after given
// timeout
//
// Deprecated: This library is being deprecated in favor of using
// https://pkg.go.dev/github.com/sourcegraph/conc/pool instead. There is
// no equivalent to this function in the new library. For more
// information, see the README:
// https://github.com/getoutreach/gobox/tree/main/pkg/async/pool/README.md
func WithTimeout(timeout time.Duration, scheduler Scheduler) Scheduler {
	return SchedulerFunc(func(ctx context.Context, r async.Runner) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		return scheduler.Schedule(ctx, async.Func(func(context.Context) error {
			defer cancel()
			return r.Run(ctx)
		}))
	})
}
