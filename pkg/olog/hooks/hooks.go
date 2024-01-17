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
// provided logger use the global handler provided by this package. See
// the documentation on the 'handler' global for more information.
//
// The logger will be automatically associated with the module and
// package that it was instantiated in. This is done by looking at the
// call stack.
//
// Note: As mentioned above, this logger is associated with the module
// and package that created it. So, if you pass this logger to another
// module or package, the association will NOT be changed. This
// includes the caller metadata added to every log line as well as
// log-level management. If a type has a common logging format that the
// other module or package should use, then a slog.LogValuer should be
// implemented on that type instead of passing a logger around. If
// trying to set attributes the be logged by default, this is not
// supported without retaining the original association.
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
