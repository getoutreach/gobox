// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Implements an optional, context-aware level resolver
// hook allowing client code to override log levels per log emission
// (e.g. via feature flags).

package olog

import (
	"context"
	"log/slog"
	"sync/atomic"

	charmlog "github.com/charmbracelet/log"
)

// LevelResolver optionally resolves the log level to enforce for a log
// record at emission time. It receives the request context, the
// addresses (package/module paths) associated with the emitting logger,
// and the level that would otherwise apply (from the level registry or
// the global level). Return (level, true) to override, or (_, false)
// to fall back to the default behavior.
//
// The resolver is invoked on every log emission, so implementations
// must be cheap (e.g. backed by a locally-cached feature-flag client).
type LevelResolver func(ctx context.Context, addresses []string, registryLevel slog.Level) (slog.Level, bool)

// globalLevelResolver holds the process-wide LevelResolver, if any.
var globalLevelResolver atomic.Pointer[LevelResolver]

// SetLevelResolver installs a process-wide LevelResolver consulted by
// all loggers created by this package. Pass nil to remove a previously
// installed resolver. Services that do not call this pay no overhead
// beyond a nil check.
func SetLevelResolver(r LevelResolver) {
	if r == nil {
		globalLevelResolver.Store(nil)
		return
	}
	globalLevelResolver.Store(&r)
}

// resolverHandler wraps a slog.Handler and consults the global
// LevelResolver (when set) in Enabled, using the per-record context.
type resolverHandler struct {
	inner   slog.Handler
	leveler slog.Leveler
	addrs   []string
}

// newResolverHandler wraps inner with LevelResolver support.
func newResolverHandler(inner slog.Handler, leveler slog.Leveler, addrs []string) slog.Handler {
	return &resolverHandler{inner: inner, leveler: leveler, addrs: addrs}
}

// Enabled implements slog.Handler. If a LevelResolver is installed and
// returns an override, the record is gated on the resolved level;
// otherwise the decision is delegated to the wrapped handler.
func (h *resolverHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if rp := globalLevelResolver.Load(); rp != nil {
		if resolved, ok := (*rp)(ctx, h.addrs, h.leveler.Level()); ok {
			// charmlog.Logger filters again in Handle using its own
			// level, so keep it in sync with the resolved level.
			if c, isCharm := h.inner.(*charmLevelHandler); isCharm {
				c.inner.SetLevel(charmlog.Level(resolved))
			}
			return level >= resolved
		}
	}
	return h.inner.Enabled(ctx, level)
}

// Handle implements slog.Handler.
//
//nolint:gocritic // Why: Handle's signature is fixed by the slog.Handler interface.
func (h *resolverHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.inner.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h *resolverHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &resolverHandler{inner: h.inner.WithAttrs(attrs), leveler: h.leveler, addrs: h.addrs}
}

// WithGroup implements slog.Handler.
func (h *resolverHandler) WithGroup(name string) slog.Handler {
	return &resolverHandler{inner: h.inner.WithGroup(name), leveler: h.leveler, addrs: h.addrs}
}
