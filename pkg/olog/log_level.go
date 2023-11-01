// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains a dynamic log level implementation that
// implements the slog.Leveler interface.

package olog

import (
	"log/slog"
	"sync/atomic"
)

// level is a global log-level. It can be updated using SetLevel, if
// not set then the logging level will be retrieved from
// configuration.
//
// Defaults to "0" which is the info level.
var level atomic.Int64

// _ ensures that leveler implements slog.Leveler.
var _ slog.Leveler = &leveler{}

// leveler is a slog.Leveler implementation that returns the current
// logging level. For more information, see the Level method.
type leveler struct {
	// levelRegistry is the registry that should be used to look up
	// log-level overrides.
	levelRegistry *levelRegistry

	// addrs are addresses that this logger uses for determining what the
	// logging level of this logger should be.
	//
	// Note: The more addresses that this logger is associated with, the
	// more expensive it is to determine the logging level. So, it is
	// recommended that only one to two address be used per logger.
	addrs []string
}

// Level returns a logging level. This is used by the package-level
// handler to determine which logs should be printed. The level is
// determined using the following logic:
//
//   - If the global level is set, then that level is returned.
//   - Otherwise, the level is retrieved from configuration using the
//     config package. The configuration is automatically reloaded when
//     changes are detected.
func (l *leveler) Level() slog.Level {
	addrLevel := l.levelRegistry.Get(l.addrs...)
	if addrLevel != nil {
		return *addrLevel
	}

	return slog.Level(level.Load())
}

// newLeveler creates a new leveler with the provided addresses.
func newLeveler(lr *levelRegistry, addrs []string) slog.Leveler {
	return &leveler{lr, addrs}
}

// SetGlobalLevel sets the global logging level used by all loggers by
// default that do not have a level set in the level registry. This
// impacts loggers that have previously been created as well as loggers
// that will be created in the future.
func SetGlobalLevel(l slog.Level) {
	level.Store(int64(l))
}
