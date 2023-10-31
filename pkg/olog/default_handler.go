// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains logic for determining which handler should be
// used by default.

package olog

import (
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	"golang.org/x/term"
)

var (
	// defaultHandler is the global handler used by all loggers returned
	// by this package. For details on how this is set, see
	// `determineDefaultHandler`.
	defaultHandler atomic.Int32

	// defaultOut is the default output for the default handler. This is
	// set to `os.Stderr` by default.
	defaultOut = os.Stderr
)

// DefaultHandlerType denotes which handler should be used by default.
// This is calculated via the `setDefaultHandler` function on package
// init.
type DefaultHandlerType int

const (
	JSONHandler DefaultHandlerType = iota
	TextHandler
)

// determineDefaultHandler sets the default handler based on the current
// environment. If the `os.Stderr` is a TTY, then the default handler
// is a `slog.TextHandler`. Otherwise, the default handler is the
// `slog.JSONHandler`.
func determineDefaultHandler() {
	if term.IsTerminal(int(os.Stderr.Fd())) {
		defaultHandler.Store(int32(TextHandler))
	} else {
		defaultHandler.Store(int32(JSONHandler))
	}
}

// init sets the default handler. See `determineDefaultHandler` for more
// information.
func init() {
	determineDefaultHandler()
}

// createHandler creates a new handler for usage with a slog.Logger. The
// handler used is determined based on the current defaultHandler. The
// handler is configured to add source information to all logs as well
// as determine the log level with the `leveler` implementation provided
// by this package. The `leveler` implementation is configured to use
// the provided moduleName and packageName as addresses for determining
// the log level.
//
// `lr` should be `globalLevelRegistry` unless you are testing.
// `out` should be defaultOut unless you are testing.
func createHandler(lr *levelRegistry, out io.Writer, moduleName, packageName string) slog.Handler {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level: newLeveler(lr, []string{
			// Order is important here, the first address that
			// matches will be used. So, we start with the most granular
			// address, the package name.
			packageName,
			moduleName,
		}),
	}
	switch DefaultHandlerType(defaultHandler.Load()) {
	case JSONHandler:
		return slog.NewJSONHandler(out, opts)
	case TextHandler:
		return slog.NewTextHandler(out, opts)
	default:
		panic("unknown default handler")
	}
}
