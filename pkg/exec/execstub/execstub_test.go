// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for the canned, function and spy fakes.

package execstub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/exec"
)

// runMethods returns the three ways to run a command, each reduced to its
// error.
func runMethods() map[string]func(*exec.Cmd) error {
	return map[string]func(*exec.Cmd) error{
		"Run":            func(c *exec.Cmd) error { return c.Run() },
		"Output":         func(c *exec.Cmd) error { _, err := c.Output(); return err },
		"CombinedOutput": func(c *exec.Cmd) error { _, err := c.CombinedOutput(); return err },
	}
}

// TestStubbedCmd checks that the stub returns its canned output whatever the
// command is, and that Dir has no effect on it.
func TestStubbedCmd(t *testing.T) {
	t.Parallel()

	stub := NewStubbedCmd("stdout content", "")

	for name, run := range map[string]func(*exec.Cmd) ([]byte, error){
		"Output":         (*exec.Cmd).Output,
		"CombinedOutput": (*exec.Cmd).CombinedOutput,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := stub.CommandContext(t.Context(), "anything", "at", "all")
			cmd.Dir = "/some/dir"

			out, err := run(cmd)
			assert.NilError(t, err)
			assert.Equal(t, string(out), "stdout content")
		})
	}

	assert.NilError(t, stub.CommandContext(t.Context(), "anything").Run())
}

// TestStubbedCmdError checks that a non-empty stderr becomes an
// *exec.ExitError, which callers detect with errors.As.
func TestStubbedCmdError(t *testing.T) {
	t.Parallel()

	_, err := NewStubbedCmd("", "boom").CommandContext(t.Context(), "anything").Output()

	assert.Equal(t, string(exitErrorOf(t, err).Stderr), "boom")
}

// TestSpyExec checks that the spy records every command, and returns a
// command from the wrapped Exec.
func TestSpyExec(t *testing.T) {
	t.Parallel()

	spy := NewSpyExec(NewStubbedCmd("stubbed output", ""))

	first := spy.CommandContext(t.Context(), "gh", "pr", "list")
	second := spy.CommandContext(t.Context(), "git", "status")

	assert.Equal(t, len(spy.Calls), 2)
	assert.Equal(t, spy.Calls[0].Name, "gh")
	assert.DeepEqual(t, spy.Calls[0].Args, []string{"pr", "list"})
	assert.Equal(t, spy.Calls[0].Cmd, first)
	assert.Equal(t, spy.Calls[1].Name, "git")
	assert.DeepEqual(t, spy.Calls[1].Args, []string{"status"})
	assert.Equal(t, spy.Calls[1].Cmd, second)

	out, err := first.Output()
	assert.NilError(t, err)
	assert.Equal(t, string(out), "stubbed output")
}

// TestFuncExec checks that a FuncExec can answer by name and args across
// calls, and can see the Dir that the caller set.
func TestFuncExec(t *testing.T) {
	t.Parallel()

	var calls []string
	ex := NewFuncExec(func(cmd *exec.Cmd, name string, args []string) ([]byte, error) {
		calls = append(calls, name+" dir="+cmd.Dir)
		if name == "fails" {
			return nil, NewExitError(1, "boom")
		}
		return []byte(name + ":" + args[0]), nil
	})

	first := ex.CommandContext(t.Context(), "ok", "one")
	first.Dir = "/first"
	out, err := first.Output()
	assert.NilError(t, err)
	assert.Equal(t, string(out), "ok:one")

	second := ex.CommandContext(t.Context(), "fails", "two")
	second.Dir = "/second"
	_, err = second.Output()
	assert.Equal(t, string(exitErrorOf(t, err).Stderr), "boom")

	assert.DeepEqual(t, calls, []string{"ok dir=/first", "fails dir=/second"})
}

// TestWrappedExitErrorPassesThroughUnchanged checks the limit that
// NewFuncExec documents: an error that wraps an *exec.ExitError is returned
// as it is, with the stderr text still on the inner error.
func TestWrappedExitErrorPassesThroughUnchanged(t *testing.T) {
	t.Parallel()

	inner := &exec.ExitError{Stderr: []byte("boom\n")}
	wrapped := fmt.Errorf("wrapped: %w", inner)
	ex := NewFuncExec(func(*exec.Cmd, string, []string) ([]byte, error) { return nil, wrapped })

	for name, run := range runMethods() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := ex.CommandContext(t.Context(), "anything")
			err := run(cmd)

			assert.Equal(t, err, wrapped)
			assert.Equal(t, string(inner.Stderr), "boom\n")
			assert.Assert(t, cmd.ProcessState.Success(), "expected exit status 0 for an error that is not a bare *exec.ExitError")
		})
	}
}

// TestNewExitErrorMatchesARealExit checks that a stubbed failure reports what
// a real one does: the exit code and the "exit status N" text.
func TestNewExitErrorMatchesARealExit(t *testing.T) {
	t.Parallel()

	for _, code := range []int{1, 2, 255} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()

			script := "exit " + strconv.Itoa(code)
			want := exitErrorOf(t, exec.NewRealExec().CommandContext(t.Context(), "sh", "-c", script).Run())
			got := exitErrorOf(t, NewExitError(code, "boom"))

			assert.Equal(t, got.ExitCode(), code)
			assert.Equal(t, got.Error(), want.Error())
			assert.Equal(t, string(got.Stderr), "boom")
		})
	}
}

// TestNewExitErrorReturnsNewErrors checks that the errors for one code do
// not share Stderr, although they share the cached process state.
func TestNewExitErrorReturnsNewErrors(t *testing.T) {
	t.Parallel()

	first := exitErrorOf(t, NewExitError(4, "first"))
	second := exitErrorOf(t, NewExitError(4, "second"))

	assert.Assert(t, first != second)
	assert.Equal(t, string(first.Stderr), "first")
	assert.Equal(t, string(second.Stderr), "second")
}

// TestNewExitErrorRejectsImpossibleCodes checks that a code that no process
// can exit with fails, instead of giving an error with no exit status.
func TestNewExitErrorRejectsImpossibleCodes(t *testing.T) {
	t.Parallel()

	for _, code := range []int{-1, 0, 256} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, NewExitError(code, "boom"), errInvalidExitCode)
		})
	}
}

// TestNewExitErrorDoesNotNeedPATH checks that a test which empties PATH
// still gets an exit status, because the stub runs /bin/sh by its path.
func TestNewExitErrorDoesNotNeedPATH(t *testing.T) {
	t.Setenv("PATH", "")

	assert.Equal(t, exitErrorOf(t, NewExitError(7, "")).ExitCode(), 7)
}

// TestStubbedFailureExitsWithItsCode checks that a stubbed failure carries a
// real exit status, whichever method runs it.
func TestStubbedFailureExitsWithItsCode(t *testing.T) {
	t.Parallel()

	stubs := map[string]struct {
		stub exec.Exec
		code int
	}{
		"NewStubbedCmd":         {NewStubbedCmd("", "boom\n"), 1},
		"NewStubbedCmdWithCode": {NewStubbedCmdWithCode("", "", 2), 2},
	}

	for stubName, tc := range stubs {
		t.Run(stubName, func(t *testing.T) {
			t.Parallel()

			for runName, run := range runMethods() {
				t.Run(runName, func(t *testing.T) {
					t.Parallel()

					exitErr := exitErrorOf(t, run(tc.stub.CommandContext(t.Context(), "anything")))

					assert.Equal(t, exitErr.ExitCode(), tc.code)
					assert.Equal(t, exitErr.Error(), fmt.Sprintf("exit status %d", tc.code))
				})
			}
		})
	}
}

// TestStubbedCmdWithCodeEdges checks the two cases that a failing stub does
// not cover: code 0 succeeds, and an impossible code fails every command.
func TestStubbedCmdWithCodeEdges(t *testing.T) {
	t.Parallel()

	t.Run("code 0 succeeds and ignores stderr", func(t *testing.T) {
		t.Parallel()

		out, err := NewStubbedCmdWithCode("hello\n", "boom\n", 0).CommandContext(t.Context(), "anything").CombinedOutput()

		assert.NilError(t, err)
		assert.Equal(t, string(out), "hello\n")
	})

	t.Run("an impossible code fails every command", func(t *testing.T) {
		t.Parallel()

		_, err := NewStubbedCmdWithCode("", "", 256).CommandContext(t.Context(), "anything").Output()

		assert.ErrorIs(t, err, errInvalidExitCode)
	})
}

// TestOutputKeepsASharedErrorIntact checks that Output does not change an
// error that a FuncExec returns for many commands.
func TestOutputKeepsASharedErrorIntact(t *testing.T) {
	t.Parallel()

	shared := exitErrorOf(t, NewExitError(1, "boom\n"))
	ex := NewFuncExec(func(*exec.Cmd, string, []string) ([]byte, error) { return nil, shared })

	withWriter := ex.CommandContext(t.Context(), "anything")
	withWriter.Stderr = &bytes.Buffer{}
	_, err := withWriter.Output()
	assert.Equal(t, len(exitErrorOf(t, err).Stderr), 0)

	_, err = ex.CommandContext(t.Context(), "anything").Output()

	assert.Equal(t, string(exitErrorOf(t, err).Stderr), "boom\n")
	assert.Equal(t, string(shared.Stderr), "boom\n")
}

// TestFakeProcess checks what the Process and ProcessState of a fake report.
func TestFakeProcess(t *testing.T) {
	t.Parallel()

	cmd := NewStubbedCmd("", "").CommandContext(t.Context(), "anything")
	assert.NilError(t, cmd.Run())

	assert.Equal(t, cmd.Process.Pid, 1<<22+1)
	assert.Equal(t, cmd.ProcessState.Pid(), cmd.Process.Pid)
	assert.Equal(t, cmd.ProcessState.String(), "exit status 0")
	assert.Assert(t, cmd.ProcessState.Sys() == nil)
	assert.Assert(t, cmd.ProcessState.SysUsage() == nil)
	assert.Equal(t, cmd.ProcessState.UserTime(), cmd.ProcessState.SystemTime())
	assert.NilError(t, cmd.Process.Release())
}

// TestSignals checks that a fake records every signal, and that only os.Kill
// stops it.
func TestSignals(t *testing.T) {
	t.Parallel()

	cmd := newCmd(t.Context(), sleeping.fn, "sleep", nil)
	assert.NilError(t, cmd.Start())

	assert.NilError(t, cmd.Process.Signal(os.Interrupt))
	assert.NilError(t, cmd.Process.Kill())
	assertKilled(t, cmd, cmd.Wait())

	assert.DeepEqual(t, Signals(cmd), []os.Signal{os.Interrupt, os.Kill})
	assert.Assert(t, Signals(exec.NewRealExec().CommandContext(t.Context(), "true")) == nil)
}

// TestKillAfterExit checks that a kill after the fake returned, but before
// Wait, does not change the result, as for a real process that exited.
func TestKillAfterExit(t *testing.T) {
	t.Parallel()

	cmd := NewStubbedCmdWithCode("hello\n", "", 0).CommandContext(t.Context(), "anything")
	pipe, err := cmd.StdoutPipe()
	assert.NilError(t, err)
	assert.NilError(t, cmd.Start())
	// The fake closes the pipe after it returns, so EOF means it returned.
	_, err = io.ReadAll(pipe)
	assert.NilError(t, err)

	assert.NilError(t, cmd.Process.Kill())

	assert.NilError(t, cmd.Wait())
}

// TestContextDoneAfterExit checks the os/exec rule for a context that is
// done after the command exited but before Wait: Cancel runs, and Wait
// returns the context error because the command succeeded.
func TestContextDoneAfterExit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cmd := NewStubbedCmdWithCode("hello\n", "", 0).CommandContext(ctx, "anything")
	canceled := make(chan struct{})
	cmd.Cancel = func() error {
		defer close(canceled)
		return cmd.Process.Kill()
	}
	pipe, err := cmd.StdoutPipe()
	assert.NilError(t, err)
	assert.NilError(t, cmd.Start())
	_, err = io.ReadAll(pipe)
	assert.NilError(t, err)

	cancel()
	// Wait for Cancel, which os/exec calls from another goroutine.
	<-canceled

	assert.ErrorIs(t, cmd.Wait(), context.Canceled)
	assert.Assert(t, cmd.ProcessState.Success())
}

// waitFor returns a RunFunc that succeeds when release is closed, and
// ignores kills.
func waitFor(release <-chan struct{}) RunFunc {
	return func(context.Context, *exec.Cmd, io.Reader, io.Writer, io.Writer) error {
		<-release
		return nil
	}
}

// TestCancelNilIgnoresContext checks that a nil Cancel lets the fake run
// after the context is done, as os/exec does.
func TestCancelNilIgnoresContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	release := make(chan struct{})
	cmd := newCmd(ctx, waitFor(release), "wait", nil)
	cmd.Cancel = nil
	assert.NilError(t, cmd.Start())

	cancel()
	close(release)

	assert.NilError(t, cmd.Wait())
	assert.DeepEqual(t, Signals(cmd), []os.Signal(nil))
}

// errCancel is the error of a Cancel function that fails.
var errCancel = errors.New("cancel failed")

// TestCancelError checks that an error from Cancel is returned by Wait, with
// the os/exec prefix.
func TestCancelError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	release := make(chan struct{})
	cmd := newCmd(ctx, waitFor(release), "wait", nil)
	cmd.Cancel = func() error {
		close(release)
		return errCancel
	}
	assert.NilError(t, cmd.Start())

	cancel()

	err := cmd.Wait()
	assert.ErrorIs(t, err, errCancel)
	assert.Error(t, err, "exec: canceling Cmd: cancel failed")
}
