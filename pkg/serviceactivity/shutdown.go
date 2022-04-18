// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements a shutdown service for gracefully
// shutting down the application.

package serviceactivity

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/getoutreach/gobox/pkg/orerr"
)

var _ ServiceActivity = (*shutdownService)(nil)

// ShutdownService implements the ServiceActivity interface for handling graceful
// shutdowns.
type shutdownService struct {
	done chan struct{}
}

// NewShutdownService creates a new shutdown service
func NewShutdownService() *shutdownService { //nolint:revive
	return &shutdownService{
		done: make(chan struct{}),
	}
}

// Run helps implement the ServiceActivity for its pointer receiver, ShutdownService.
// This function listens for interrupt signals and handles gracefully shutting down
// the entire application.
func (s *shutdownService) Run(ctx context.Context) error {
	// listen for interrupts and gracefully shutdown server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case out := <-c:
		// Allow interrupt signals to be caught again in worse-case scenario
		// situations when the service hangs during a graceful shutdown.
		signal.Reset(os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		return orerr.ShutdownError{
			Err: fmt.Errorf("shutting down due to interrupt: %v", out),
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return nil
	}
}

// Close stops the shutdown service
func (s *shutdownService) Close(_ context.Context) error {
	close(s.done)
	return nil
}
