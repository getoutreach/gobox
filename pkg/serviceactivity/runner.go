// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements the service activity runner

package serviceactivity

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Runner is a service activity runner
type Runner struct {
	acts []ServiceActivity
}

// New creates a new service activity runner
func New(acts []ServiceActivity) *Runner {
	return &Runner{acts}
}

// Run starts all serviceactivities and blocks until they are done
// returning an error if any of them failed to start.
func (r *Runner) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	// run the activities in parallel
	for idx := range r.acts {
		act := r.acts[idx]
		g.Go(func() error {
			// stop the activity once it returns
			defer act.Close(ctx)

			return act.Run(ctx)
		})
	}

	return g.Wait()
}
