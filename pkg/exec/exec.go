// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides helpers for os/exec

// Package exec wraps os/exec, so that code can run external commands and
// tests can fake them. It also has helpers for os/exec, such as
// [ResolveExecutable].
//
// # Injection point
//
// A package-level function cannot be faked. Code that runs a command it
// wants to test takes an [Exec] value and calls its CommandContext method.
// Production code passes [NewRealExec]. Tests pass a fake from package
// github.com/getoutreach/gobox/pkg/exec/execstub. The package-level
// [CommandContext] function is a shortcut for NewRealExec().CommandContext,
// and a test cannot replace it.
//
// # Name clash with os/exec
//
// This package has the same name as os/exec on purpose. [Cmd] has the fields
// and methods of [os/exec.Cmd], and this package has aliases for
// [ExitError], [Error], [ErrNotFound], [ErrDot], [ErrWaitDelay] and
// [LookPath]. A file that adopts this package usually only replaces its
// os/exec import. A file that needs both packages must import one of them
// with another name:
//
//	import (
//		osexec "os/exec"
//
//		"github.com/getoutreach/gobox/pkg/exec"
//	)
//
// # Differences from os/exec
//
// [Cmd.Process] is a [*Process], not an [*os.Process], and
// [Cmd.ProcessState] is the [ProcessState] interface, not an
// [*os.ProcessState]. The Pid field and the methods are the same, but code
// that passes them to a function that takes the os types does not compile.
// Cmd has no Err field, and this package has no Command function without a
// context.
//
// Errors from the command, such as an [*ExitError], are returned as os/exec
// returns them, without added context, so that type assertions on them
// continue to work.
//
// # Fakes
//
// [NewFakeCmd] makes a Cmd that runs a [Runner] in a goroutine instead of a
// process. Package execstub uses it. Start, Wait, the pipe methods, Output,
// CombinedOutput, the context and Cancel work as they do for a real command.
// A fake command ignores ExtraFiles, SysProcAttr and WaitDelay, and its
// [ProcessState] returns nil from Sys and SysUsage.
package exec

import (
	"os/exec"
	"path/filepath"
)

// ResolveExecutable find the absolute path to a given binary.
// This is meant to be used with os.Args[0].
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

// ResolveExecuable is a wrapper for ResolveExecutable.
//
// Deprecated: use ResolveExecutable.
func ResolveExecuable(path string) (string, error) {
	return ResolveExecutable(path)
}
