// Package olog implements a logger focused on providing strongly typed
// logs focused around audit and compliance. Also included are
// utilities to change logging level dynamically (without
// re-compiling/restarting).
//
// This package does not provide the ability to ship logs to a remote
// server, instead a logging collector should be used.
package olog

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/getoutreach/gobox/pkg/callerinfo"
)

// New creates a new slog instance that can be used for logging. The
// provided logger use the global handler provided by this package. See
// the documentation on the 'handler' global for more information.
//
// The logger will be automatically associated with the module and
// package that it was instantiated in. This is done by looking at the
// call stack.
func New() *slog.Logger {
	m, err := getMetadata()
	if err != nil {
		//nolint:errorlint // Why: We can't wrap panic-d errors.
		panic(fmt.Errorf("failed to get information about what created the logger: %v", err))
	}

	handler := createHandler(globalLevelRegistry, defaultOut, &m)
	return new(globalLevelRegistry, defaultOut, &m, handler)
}

// metadata is metadata associated with every logger created by New().
// This metadata always corresponds to whatever created the logger
// through New().
type metadata struct {
	// ModuleName is the name of the module that created this logger.
	// Format: <module> (e.g., github.com/getoutreach/gobox)
	ModuleName string

	// ModuleVersion is the version of the module that created this
	// logger. See (callerInfo.CallerInfo).ModuleVersion for more
	// information.
	ModuleVersion string

	// PackageName is the name of the package that created this logger.
	// Format: <moduleName>/<package>
	PackageName string
}

// getLoggerInformation returns the moduleName, moduleVersion, and
// packageName for the caller of the New() function. This associates a
// logger with the module and package that it was instantiated in.
func getMetadata() (metadata, error) {
	var m metadata

	// Skips are the number of functions we should skip when attempting to
	// look up the caller information.
	//
	// 1: getLoggerInformation (this function)
	// 2: New (the function that called this function)
	skips := uint16(2)

	ci, err := callerinfo.GetCallerInfo(skips)
	if err != nil {
		return m, err
	}

	// We require module information, if we can't get it we should return
	// an error.
	if ci.Module == "" {
		return m, fmt.Errorf("failed to determine the current module")
	}

	return metadata{
		ModuleName:    ci.Module,
		ModuleVersion: ci.ModuleVersion,
		PackageName:   ci.Package,
	}, nil
}

// new returns a new logger with the provided levelRegistry, io.Writer,
// and metadata. This is used primarily useful for testing purposes.
//
// h is optional and only should be provided by New().
func new(lr *levelRegistry, out io.Writer, m *metadata, h slog.Handler) *slog.Logger {
	if h == nil {
		h = createHandler(lr, out, m)
	}

	return slog.New(h).
		With("module", m.ModuleName, "modulever", m.ModuleVersion)
}
