// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains log handler wrapper allowing hook funcs.

package olog

import (
	"context"
	"log/slog"
)

// LogHookFunc defines a function which can be called prior to a log being
// emitted, allowing the caller to augment the attributes on a log by
// returning a slice of slog.Attr which will appended to the record. The
// caller may also return an error, which will be handled by the underlying
// log handler (slog.TextHandler or slog.JSONHandler).
// nolint:gocritic // Why: this is the signature require by the slog handler interface
type LogHookFunc func(context.Context, slog.Record) ([]slog.Attr, error)

type hookHandler struct {
	hooks []LogHookFunc
	slog.Handler
}

// Handle performs the required Handle operation of the log handler interface,
// calling any provided hooks before calling the underlying embedded handler.
// nolint:gocritic // Why: this is the signature require by the slog handler interface
func (h *hookHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, hook := range h.hooks {
		attrs, err := hook(ctx, r)
		if err != nil {
			return err
		}

		r.AddAttrs(attrs...)
	}

	return h.Handler.Handle(ctx, r)
}
