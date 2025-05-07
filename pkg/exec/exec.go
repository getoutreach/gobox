// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides helpers for os/exec

// Package exec implements os/exec stdlib helpers
package exec

import (
	"os/exec"
	"path/filepath"
)

// ResolveExecutable find the absolute path to a given binary.
// This is meant to be used with os.Args[0]
func ResolveExecutable(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// if we're not a path, e.g. devenv then look it up
	// in PATH
	if dir, _ := filepath.Split(path); dir == "" {
		return exec.LookPath(path)
	}

	// otherwise we should just return the absolute path (resolve it)
	return filepath.Abs(path)
}

// ResolveExecuable is a wrapper for ResolveExecutable. This is the
// original function which was misspelled. At some point in the future,
// this will be deprecated and then removed.
func ResolveExecuable(path string) (string, error) {
	return ResolveExecutable(path)
}
