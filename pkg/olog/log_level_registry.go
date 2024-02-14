// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Implements a global registry of logger handlers allowing
// dynamic configuration of the handlers used by any logger created by
// this package via `New`.

package olog

import (
	"log/slog"
	"sync"
)

// globalLevelRegistry is the global registry used to store all handlers.
var globalLevelRegistry = newRegistry()

// levelRegistry contains levelers for all
type levelRegistry struct {
	// mu is a mutex used to protect the registry.
	mu sync.RWMutex

	// ByAddress contains a map of all logger addresses to their
	// corresponding log-level overrides. The `addressedLeveler` function
	// reads from this to determine the log-level for a logger via the
	// `levelRegistry` global.
	ByAddress map[string]slog.Level
}

// newRegistry create a fully initialized registry.
func newRegistry() *levelRegistry {
	return &levelRegistry{
		ByAddress: make(map[string]slog.Level),
	}
}

// Set sets the log-level for the provided addresses.
func (lr *levelRegistry) Set(level slog.Level, address ...string) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	for _, addr := range address {
		lr.ByAddress[addr] = level
	}
}

// Get returns a log-level if any of the provided addresses have been
// registered in the current registry. If none have been set, nil is
// returned.
//
// Addresses are searched in order, so the first address that is found
// is returned.
func (lr *levelRegistry) Get(address ...string) *slog.Level {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	for _, addr := range address {
		if level, ok := lr.ByAddress[addr]; ok {
			return &level
		}
	}

	return nil
}
