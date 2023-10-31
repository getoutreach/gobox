// Package olog implements a logger focused on providing strongly typed
// logs focused around audit and compliance. Also included are
// utilities to change logging level dynamically (without
// re-compiling/restarting).
//
// This package does not provide the ability to ship logs to a remote
// server, instead a logging collector should be used.
package olog

import (
	"io"
	"log/slog"
)

// New creates a new slog instance that can be used for logging. The
// provided logger use the global handler provided by this package. See
// the documentation on the 'handler' global for more information.
//
// The logger will be automatically associated with the module and
// package that it was instantiated in. This is done by looking at the
// call stack.
func New() *slog.Logger {
	// Look up the call stack to find the module and package.
	// Create a new handler and store it in the map.
	handler := createHandler(globalLevelRegistry, defaultOut, "moduleNameGoesHere", "packageNameGoesHere")
	return slog.New(handler)
}

// newTestLogger creates a new logger for testing purposes. This logger
// can be provided with a custom module and package name as well as it's
// own level registry.
func newTestLogger(lr *levelRegistry, out io.Writer, moduleName string, packageName string) *slog.Logger {
	// Create a new handler and store it in the map.
	handler := createHandler(lr, out, moduleName, packageName)
	return slog.New(handler)
}
