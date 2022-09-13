package exec

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"

	"gotest.tools/v3/assert"
)

// Short sanity-check of OsExecutor. Not exhaustive, as the implementation is
// trivial.
func TestOsExecutor(t *testing.T) {
	t.Run("Run should work in the happy case", func(t *testing.T) {
		cmd := exec.Command("echo", "this is a test")
		assert.NilError(t, OsExecutor{}.Run(cmd))
	})

	t.Run("Run should wrap errors in ExecError", func(t *testing.T) {
		cmd := exec.Command("bash", "-c", "echo 'this is a test' >&2 && false")
		err := OsExecutor{}.Run(cmd)
		var exitErr *ExitError
		assert.Assert(t, errors.As(err, &exitErr))
		// Should have captured the error code.
		assert.Assert(t, exitErr.ExitCode > 0)
		// Should NOT have captured stderr. This only happens for Output.
		assert.Equal(t, string(exitErr.Stderr), "")
	})

	t.Run("Output should work in the happy case", func(t *testing.T) {
		cmd := exec.Command("echo", "this is a test")
		stdout, err := OsExecutor{}.Output(cmd)
		assert.NilError(t, err)
		assert.Equal(t, string(stdout), "this is a test")
	})

	t.Run("Output should wrap errors in ExecError", func(t *testing.T) {
		cmd := exec.Command("bash", "-c", "echo 'this is good' && echo 'this is bad' >&2 && false")
		stdout, err := OsExecutor{}.Output(cmd)
		var exitErr *ExitError
		assert.Assert(t, errors.As(err, &exitErr))
		assert.Assert(t, exitErr.ExitCode > 0)
		assert.Equal(t, string(stdout), "this is good")
		assert.Equal(t, string(exitErr.Stderr), "this is bad")
	})
}

func TestMockExecutor(t *testing.T) {
	t.Run("Run should return provided error", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		err := errors.New("bad things")
		executor := &MockExecutor{Error: err}
		assert.Equal(t, executor.Run(cmd), err)
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Run should write stdout and stderr", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		stdout := &bytes.Buffer{}
		cmd.Stdout = stdout
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		executor := &MockExecutor{Stdout: []byte("is stdout"), Stderr: []byte("is stderr")}
		assert.NilError(t, executor.Run(cmd), "no error expected")
		assert.Equal(t, stdout.String(), "is stdout")
		assert.Equal(t, stderr.String(), "is stderr")
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Run should error if called twice", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		executor := &MockExecutor{}
		assert.NilError(t, executor.Run(cmd), "no error expected on first call")
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
		assert.ErrorContains(t, executor.Run(cmd), "started twice")
	})

	t.Run("Start should return provided error", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		err := errors.New("bad things")
		executor := &MockExecutor{StartError: err}
		assert.Equal(t, executor.Start(cmd), err)
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Start should write stdout and stderr", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		stdout := &bytes.Buffer{}
		cmd.Stdout = stdout
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		executor := &MockExecutor{Stdout: []byte("is stdout"), Stderr: []byte("is stderr")}
		assert.NilError(t, executor.Start(cmd), "no error expected")
		assert.Equal(t, stdout.String(), "is stdout")
		assert.Equal(t, stderr.String(), "is stderr")
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Start should error if called twice", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		executor := &MockExecutor{}
		assert.NilError(t, executor.Start(cmd), "no error expected on first call")
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
		assert.ErrorContains(t, executor.Start(cmd), "started twice")
	})

	t.Run("Wait should return provided error", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		err := errors.New("bad things")
		executor := &MockExecutor{Error: err, StartedExecution: true}
		assert.Equal(t, executor.Wait(cmd), err)
		assert.Equal(t, executor.waitCalled, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Wait should write stdout and stderr", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		stdout := &bytes.Buffer{}
		cmd.Stdout = stdout
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		executor := &MockExecutor{
			Stdout:           []byte("is stdout"),
			Stderr:           []byte("is stderr"),
			StartedExecution: true,
		}
		assert.NilError(t, executor.Wait(cmd), "no error expected")
		assert.Equal(t, stdout.String(), "is stdout")
		assert.Equal(t, stderr.String(), "is stderr")
		assert.Equal(t, executor.waitCalled, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Wait should error if called twice", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		executor := &MockExecutor{StartedExecution: true}
		assert.NilError(t, executor.Wait(cmd), "no error expected on first call")
		assert.Equal(t, executor.waitCalled, true)
		assert.Equal(t, executor.Cmd, cmd)
		assert.ErrorContains(t, executor.Wait(cmd), "called twice")
	})

	t.Run("Wait should error if called before starting", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		executor := &MockExecutor{}
		assert.ErrorContains(t, executor.Wait(cmd), "called before being started")
	})

	t.Run("Start followed by Wait should only print output once", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		stdout := &bytes.Buffer{}
		cmd.Stdout = stdout
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		executor := &MockExecutor{Stdout: []byte("is stdout"), Stderr: []byte("is stderr")}
		assert.NilError(t, executor.Start(cmd))
		assert.NilError(t, executor.Wait(cmd))
		assert.Equal(t, stdout.String(), "is stdout")
		assert.Equal(t, stderr.String(), "is stderr")
	})

	t.Run("Output should return provided error", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		err := errors.New("bad things")
		executor := &MockExecutor{Error: err}
		output, gotErr := executor.Output(cmd)
		assert.Equal(t, string(output), "")
		assert.Equal(t, gotErr, err)
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Output should return stdout and write stderr", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		executor := &MockExecutor{Stdout: []byte("is stdout"), Stderr: []byte("is stderr")}
		output, gotErr := executor.Output(cmd)
		assert.Equal(t, string(output), "is stdout")
		assert.NilError(t, gotErr)
		assert.Equal(t, stderr.String(), "is stderr")
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Output should return stdout and error if both given", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		err := errors.New("bad things")
		executor := &MockExecutor{Stdout: []byte("is stdout"), Error: err}
		output, gotErr := executor.Output(cmd)
		assert.Equal(t, string(output), "is stdout")
		assert.Equal(t, gotErr, err)
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
	})

	t.Run("Output should error if called twice", func(t *testing.T) {
		cmd := exec.Command("echo", "ran")
		executor := &MockExecutor{}
		_, err := executor.Output(cmd)
		assert.NilError(t, err, "no error expected on first call")
		assert.Equal(t, executor.StartedExecution, true)
		assert.Equal(t, executor.Cmd, cmd)
		_, err = executor.Output(cmd)
		assert.ErrorContains(t, err, "started twice")
	})
}
