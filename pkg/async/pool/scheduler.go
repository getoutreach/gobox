// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides a scheduler for the async pool

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
	//
	// Deprecated: This library is being deprecated in favor of using
	// https://pkg.go.dev/github.com/sourcegraph/conc/pool instead.
	// Replaces calls to Schedule with (*Pool).Go().  For more information,
	// see the README:
	// https://github.com/getoutreach/gobox/tree/main/pkg/async/pool/README.md
	Schedule(ctx context.Context, r async.Runner) error
}
