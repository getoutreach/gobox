// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Implements a hook based pattern for automatically
// adding data to logs before the record is written.

// Package olog/hooks implements a lightweight hooks interface for
// creating olog/slog compliant loggers. This package builds around
// the log handler and logger already built and used by the olog
// package, but wraps the underlying handler such that it can accept
// hook functions for augmenting the final log record.
package hooks

import (
	"context"
	"log/slog"

	"github.com/getoutreach/gobox/pkg/olog"
)

// Logger creates a new slog instance that can be used for logging. The
// provided logger uses a handler which wraps the global handler provided
// by the olog pkg, allowing hooks to be provided by the caller in order
// to automatically augment the attributes on the log record before it
// writes. See the [documentation](../README.md) on the olog pkg for more
// information.
//
// All hooks provided will executed in the order in which they are provided
// and will overwrite and attributes written by the previous hook when a
// duplicate key is provided.
func Logger(hooks ...LogHookFunc) *slog.Logger {
	defaultHandler := olog.New().Handler()
	hookedHandler := &handler{Handler: defaultHandler, hooks: hooks}
	return olog.NewWithHandler(hookedHandler)
}

// LogHookFunc defines a function which can be called prior to a log being
// emitted, allowing the caller to augment the attributes on a log by
// returning a slice of slog.Attr which will appended to the record. The
// caller may also return an error, which will be handled by the underlying
// log handler (slog.TextHandler or slog.JSONHandler).
type LogHookFunc func(context.Context, slog.Record) ([]slog.Attr, error)

type handler struct {
	hooks []LogHookFunc
	slog.Handler
}

// Handle performs the required Handle operation of the log handler interface,
// calling any provided hooks before calling the underlying embedded handler.
func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	for _, hook := range h.hooks {
		attrs, err := hook(ctx, r)
		if err != nil {
			return err
		}

		r.AddAttrs(attrs...)
	}

	return h.Handler.Handle(ctx, r)
}
