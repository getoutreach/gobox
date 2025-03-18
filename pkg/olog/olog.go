// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Implements the public API for the olog package.

// Package olog implements a lightweight logging library built around
// the slog package. It aims to never mask the core slog.Logger type by
// default. Provided is a global system for controlling logging levels
// based on the package and module that a logger was created in, with a
// system to update the logging level at runtime.
//
// This package does not provide the ability to ship logs to a remote
// server, instead a logging collector should be used.
package olog

import (
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/getoutreach/gobox/pkg/callerinfo"
)

var outputLock = new(sync.RWMutex)

// New creates a new slog instance that can be used for logging. The
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
func New() *slog.Logger {
	m, err := getMetadata()
	if err != nil {
		// We panic here because we don't have an error return signature.
		// The main reason for not having an error return signature is that
		// we do not want the logger to be able to be used without the
		// required metadata (otherwise log levels, attribution, etc. would
		// not work). If we return an error, a caller would likely just
		// return err to terminate their program anyways, and worst case
		// they would continue without a valid logger which is also not a
		// good scenario.
		//
		//nolint:errorlint // Why: We can't wrap panic-d errors.
		panic(fmt.Errorf("failed to get information about what created the logger: %v", err))
	}

	handler := createHandler(globalLevelRegistry, &m)
	return NewWithHandler(handler)
}

// metadata is metadata associated with every logger created by New().
// This metadata always corresponds to whatever created the logger
// through New().
type metadata struct {
	// ModulePath is the path of the module that created this logger.
	// Format: <module> (e.g., github.com/getoutreach/gobox)
	ModulePath string

	// ModuleVersion is the version of the module that created this
	// logger. See (callerInfo.CallerInfo).ModuleVersion for more
	// information.
	ModuleVersion string

	// PackagePath is the path of the package that created this logger.
	// Format: <moduleName>/<package>
	PackagePath string
}

// getMetadata returns the moduleName, moduleVersion, and packageName
// for the caller of the New() function. This associates a logger with
// the module and package that it was instantiated in.
func getMetadata() (metadata, error) {
	var m metadata

	// Skips are the number of functions we should skip when attempting to
	// look up the caller information.
	//
	// 1: getMetadata (this function)
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
		ModulePath:    ci.Module,
		ModuleVersion: ci.ModuleVersion,
		PackagePath:   ci.Package,
	}, nil
}

// NewWithHandler returns a new slog.Logger with the provided handler.
//
// Note: A logger created with this function will not be controlled by
// the global log level and will not have any of the features provided
// by this package. This is primarily meant to be used only by tests or
// other special cases.
func NewWithHandler(h slog.Handler) *slog.Logger {
	return slog.New(h)
}

// SetOutput sets the global logger output to desired writer.
// The function uses a mutex to ensure that setting the output writer is thread-safe.
func SetOutput(w io.Writer) {
	outputLock.Lock()
	defer outputLock.Unlock()
	defaultOut = w
}

// NewWithHooks returns a new slog.Logger, allowing hooks to be provided
// by the caller in order to automatically augment the attributes on the
// log record before it writes.
//
// All hooks provided will be executed in the order in which they are provided
// and will overwrite any attributes written by the previous hook when a
// duplicate key is provided.
func NewWithHooks(hooks ...LogHookFunc) *slog.Logger {
	defaultHandler := New().Handler()
	hookedHandler := &hookHandler{Handler: defaultHandler, hooks: hooks}
	return slog.New(hookedHandler)
}
