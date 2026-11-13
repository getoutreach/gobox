// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for the real Cmd. The execstub package tests the
// behavior that the real Cmd and the fakes share.

package exec_test

import (
	"context"
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/exec"
)

// TestRealCmdAppliesFields checks that the fields that a caller sets after
// CommandContext reach the process, as they do on an os/exec.Cmd.
func TestRealCmdAppliesFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	resolvedDir, err := filepath.EvalSymlinks(dir)
	assert.NilError(t, err)

	cmd := exec.CommandContext(t.Context(), "sh", "-c", "not run")
	cmd.Args = []string{"sh", "-c", `pwd -P; echo "env=$GREETING"; cat`}
	cmd.Dir = dir
	cmd.Env = []string{"GREETING=hello"}
	cmd.Stdin = strings.NewReader("from stdin")

	out, err := cmd.Output()
	assert.NilError(t, err)

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	assert.DeepEqual(t, lines, []string{resolvedDir, "env=hello", "from stdin"})
}

// TestRealProcessAndState checks that Process and ProcessState show the real
// process, and that ProcessState is a nil interface before Wait.
func TestRealProcessAndState(t *testing.T) {
	t.Parallel()

	cmd := exec.NewRealExec().CommandContext(t.Context(), "sh", "-c", "exit 0")
	assert.NilError(t, cmd.Start())
	assert.Assert(t, cmd.Process.Pid > 0)
	assert.Assert(t, cmd.ProcessState == nil)

	assert.NilError(t, cmd.Wait())

	state, ok := cmd.ProcessState.(*os.ProcessState)
	assert.Assert(t, ok, "expected an *os.ProcessState, got %T", cmd.ProcessState)
	assert.Equal(t, state.Pid(), cmd.Process.Pid)
	assert.Assert(t, state.Sys() != nil)
}

// TestRealCancelCanUseProcess checks that a Cancel function can use
// cmd.Process, even though os/exec calls it from another goroutine.
func TestRealCancelCanUseProcess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, "sh", "-c", "exec sleep 60")
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	assert.NilError(t, cmd.Start())

	cancel()

	err := cmd.Wait()
	var exitErr *exec.ExitError
	assert.Assert(t, errors.As(err, &exitErr), "got %v", err)
	assert.Equal(t, exitErr.String(), "signal: interrupt")
}

// TestRealOutputKeepsStderrLikeOSExec checks that Output keeps the start and
// end of a long standard error on the *ExitError, as os/exec does.
func TestRealOutputKeepsStderrLikeOSExec(t *testing.T) {
	t.Parallel()

	const script = "i=0; while [ $i -lt 10000 ]; do echo line $i >&2; i=$((i+1)); done; exit 1"

	_, wantErr := osexec.CommandContext(t.Context(), "sh", "-c", script).Output()
	_, gotErr := exec.CommandContext(t.Context(), "sh", "-c", script).Output()

	var want, got *exec.ExitError
	assert.Assert(t, errors.As(wantErr, &want))
	assert.Assert(t, errors.As(gotErr, &got))
	assert.Assert(t, strings.Contains(string(got.Stderr), "... omitting "))
	assert.Equal(t, string(got.Stderr), string(want.Stderr))
}

// TestNotFound checks that a missing command fails at Start with an error
// that matches ErrNotFound and is an *Error.
func TestNotFound(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "gobox-no-such-command")
	err := cmd.Start()

	assert.ErrorIs(t, err, exec.ErrNotFound)
	var execErr *exec.Error
	assert.Assert(t, errors.As(err, &execErr))
	assert.Assert(t, cmd.Process == nil)
	assert.Equal(t, cmd.String(), "gobox-no-such-command")
}

// TestLookPath checks the LookPath alias.
func TestLookPath(t *testing.T) {
	t.Parallel()

	want, wantErr := osexec.LookPath("sh")
	got, err := exec.LookPath("sh")

	assert.NilError(t, wantErr)
	assert.NilError(t, err)
	assert.Equal(t, got, want)
}

// TestStringAndEnviron checks String and Environ.
func TestStringAndEnviron(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "sh", "-c", "true")
	assert.Equal(t, cmd.String(), cmd.Path+" -c true")

	cmd.Env = []string{"A=1", "B=2", "A=3"}
	assert.DeepEqual(t, cmd.Environ(), []string{"B=2", "A=3"})
}

// TestProcessNotStarted checks that the methods of a Process that Start did
// not create return an error.
func TestProcessNotStarted(t *testing.T) {
	t.Parallel()

	var nilProcess *exec.Process
	for _, p := range []*exec.Process{nilProcess, {Pid: 1}} {
		assert.ErrorContains(t, p.Kill(), "not created by Start")
		assert.ErrorContains(t, p.Release(), "not created by Start")
		_, err := p.Wait()
		assert.ErrorContains(t, err, "not created by Start")
	}
}
