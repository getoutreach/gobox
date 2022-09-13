package exec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
)

// ExitError represents an error executing a command. This is used to wrap
// exec.ExitError in a mockable way.
type ExitError struct {
	// ExitCode is the exit code from the underlying process.
	ExitCode int
	// Stderr holds a subset of the standard error output from the
	// Command.Output method if standard error was not otherwise being
	// collected.
	Stderr []byte
	// errFunc is the function to generate an error string for the wrapped
	// error.
	errFunc func() string
}

// Error returns the error string from this ExitError.
func (e *ExitError) Error() string {
	return e.errFunc()
}

// wrapOsError wraps the given error in ExitError if it is an exec.ExitError;
// else, it returns the given error unchanged.
func wrapOsError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ExitError{
			ExitCode: exitErr.ExitCode(),
			Stderr:   exitErr.Stderr,
			errFunc:  exitErr.Error,
		}
	}
	return err
}

// CommandExecutor is an interface responsible for executing commands.
type CommandExecutor interface {
	// Run starts the specified command and waits for it to complete.
	Run(cmd *exec.Cmd) error
	// Start starts the specified command but does not wait for it to complete.
	Start(cmd *exec.Cmd) error
	// Wait waits for the command to exit.
	Wait(cmd *exec.Cmd) error
	// Output runs the command and returns its standard output.
	Output(cmd *exec.Cmd) ([]byte, error)
	// CombinedOutput runs the command and returns its combined standard output
	// and standard error.
	CombinedOutput(cmd *exec.Cmd) ([]byte, error)
}

// TODO(jkinkead): Add {Stdin,Stdout,Stderr}Pipe methods?

// OsExecutor is an executor that delegates to os/exec.
type OsExecutor struct{}

// Run starts the specified command and waits for it to complete.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command starts but does not complete successfully, the error is of
// type *ExitError. Other error types may be returned for other situations.
//
// If the calling goroutine has locked the operating system thread
// with runtime.LockOSThread and modified any inheritable OS-level
// thread state (for example, Linux or Plan 9 name spaces), the new
// process will inherit the caller's thread state.
func (OsExecutor) Run(cmd *exec.Cmd) error {
	return wrapOsError(cmd.Run())
}

// Start starts the specified command but does not wait for it to complete.
//
// If Start returns successfully, the c.Process field will be set.
//
// The Wait method will return the exit code and release associated resources
// once the command exits.
func (OsExecutor) Start(cmd *exec.Cmd) error {
	return wrapOsError(cmd.Start())
}

// Wait waits for the command to exit and waits for any copying to
// stdin or copying from stdout or stderr to complete.
//
// The command must have been started by Start.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command fails to run or doesn't complete successfully, the
// error is of type *ExitError. Other error types may be
// returned for I/O problems.
//
// If any of c.Stdin, c.Stdout or c.Stderr are not an *os.File, Wait also waits
// for the respective I/O loop copying to or from the process to complete.
//
// Wait releases any resources associated with the Cmd.
func (OsExecutor) Wait(cmd *exec.Cmd) error {
	return wrapOsError(cmd.Wait())
}

// Output runs the command and returns its standard output.
// Any returned error will usually be of type *ExitError.
// If c.Stderr was nil, Output populates ExitError.Stderr.
func (OsExecutor) Output(cmd *exec.Cmd) ([]byte, error) {
	output, err := cmd.Output()
	return output, wrapOsError(err)
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (OsExecutor) CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	output, err := cmd.CombinedOutput()
	return output, wrapOsError(err)
}

// MockExecutor is an executor that will return its internal data instead of
// running a command.
type MockExecutor struct {
	// Stdout is the value to return for stdout from fake execution.
	Stdout []byte
	// Stderr is the value to return for stderr from fake execution.
	Stderr []byte
	// Error, if set, is the error to return from the fake execution for Run,
	// Wait, Output, and CombinedOutput operations.
	// Like with the exec package, an error may be returned alongside output.
	Error error
	// StartError, if set, is the error to return from Start operations. When
	// set with a nil Error, this can be used to mock executions that fail to
	// start.
	// Like with the exec package, an error may be returned alongside output.
	StartError error

	// Cmd is the last command passed in for execution.
	Cmd *exec.Cmd
	// StartedExecution is set to true after execution has been started with a
	// Run-like method. Clients should set to true if calling only Wait in a
	// unit test.
	StartedExecution bool

	// wroteOutput is set to true after Stdout and Stderr are written. This
	// prevents double-writes in the case of Start and Wait both being called on
	// an executor.
	wroteOutput bool
	// waitCalled is set to true after Wait is called. This is used to provide
	// the same errors as os.Exec (it's an error to Wait twice).
	waitCalled bool
}

// writeOutput writes the mock's Stdout and Stderr values to the given command's
// streams the first time it is called on an executor.
func (m *MockExecutor) writeOutput(cmd *exec.Cmd) error {
	if !m.wroteOutput {
		if cmd.Stdout != nil {
			if _, err := cmd.Stdout.Write(m.Stdout); err != nil {
				return err
			}
		}
		if cmd.Stderr != nil {
			if _, err := cmd.Stderr.Write(m.Stderr); err != nil {
				return err
			}
		}
		m.wroteOutput = true
	}
	return nil
}

// recordExecution notes that this mock has been run and sets the executor's Cmd
// field to the given value. Returns an error if this has already been run,
// mimicking os/exec.
func (m *MockExecutor) recordExecution(cmd *exec.Cmd) error {
	if m.StartedExecution {
		return errors.New("execution started twice")
	}
	m.StartedExecution = true
	m.Cmd = cmd
	return nil
}

// Run returns the executor's Error field, writes Stdout and Stderr to their
// respective writers on `cmd`, and sets Cmd on the executor.
func (m *MockExecutor) Run(cmd *exec.Cmd) error {
	if err := m.recordExecution(cmd); err != nil {
		return err
	}

	if err := m.writeOutput(cmd); err != nil {
		return err
	}

	return m.Error
}

// Start returns the executor's StartError field, writes Stdout and Stderr to
// their respective writers on `cmd`, and sets Cmd on the executor.
func (m *MockExecutor) Start(cmd *exec.Cmd) error {
	if err := m.recordExecution(cmd); err != nil {
		return err
	}

	if err := m.writeOutput(cmd); err != nil {
		return err
	}

	return m.StartError
}

// Run returns the executor's Error field; writes Stdout and Stderr to their
// respective writers on `cmd`, if they haven't been written yet; and sets Cmd
// on the executor.
func (m *MockExecutor) Wait(cmd *exec.Cmd) error {
	if !m.StartedExecution {
		return errors.New("Wait called before being started")
	}
	if m.waitCalled {
		return errors.New("Wait called twice")
	}

	m.waitCalled = true
	m.Cmd = cmd

	if err := m.writeOutput(cmd); err != nil {
		return err
	}

	return m.Error
}

// Output returns the executor's Stdout and Error fields and sets Cmd on the
// executor.
func (m *MockExecutor) Output(cmd *exec.Cmd) ([]byte, error) {
	// Mimic os/exec implementation.
	if cmd.Stdout != nil {
		return nil, errors.New("can't call Output with Stdout set")
	}
	if err := m.recordExecution(cmd); err != nil {
		return nil, err
	}

	// This is how Output works in os/exec: Stderr is written to (if set), but
	// not Stdout.
	if m.Stderr != nil {
		if _, err := cmd.Stderr.Write(m.Stderr); err != nil {
			return nil, err
		}
	}

	if m.Stdout != nil {
		return m.Stdout, m.Error
	}

	return []byte{}, m.Error
}

// CombinedOutput returns the executor's combined Stdout + Stderr fields, and
// the Error field; writes Stdout and Stderr to their respective writers on
// `cmd`; and sets Cmd on the executor. Stdout will always precede Stderr in the
// returned value.
func (m *MockExecutor) CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	// Mimic os/exec.
	if cmd.Stdout != nil {
		return nil, errors.New("can't call CombinedOutput with Stdout set")
	}
	if cmd.Stderr != nil {
		return nil, errors.New("can't call CombinedOutput with Stderr set")
	}

	if err := m.recordExecution(cmd); err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if _, err := buf.Write(m.Stdout); err != nil {
		return buf.Bytes(), err
	}
	if _, err := buf.Write(m.Stderr); err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), m.Error
}

// Ensure MockExecutor implements CommandExecutor.
var _ CommandExecutor = &MockExecutor{}

// defaultExecutor is the default executor to use when running commands. This
// can be overridden out during testing with the SetExecutors function. Note
// that this line also ensures OsExecutor implements CommandExecutor.
var defaultExecutor CommandExecutor = OsExecutor{}

// getDefaultExecutor is the default executor function to use to fetch the
// executor to run.
func getDefaultExecutor() CommandExecutor {
	return defaultExecutor
}

// getExecutor is the function to use to look up executors for commands when
// they are run.
var getExecutor = getDefaultExecutor

// SetTestExecutors sets the executor(s) for a single test run. This will call
// the executors in the given order as CommandContext is called, and will panic
// if CommandContext is called more times than executors were provided.
// This restores the prior executor in a Cleaup job on `t`.
func SetExecutors(t *testing.T, first CommandExecutor, rest ...CommandExecutor) {
	index := 0
	allExecutors := make([]CommandExecutor, 1+len(rest))
	allExecutors[0] = first
	allExecutors = append(allExecutors, rest...)
	prevGetExecutor := getExecutor
	getExecutor = func() CommandExecutor {
		nextExecutor := allExecutors[index]
		index++
		return nextExecutor
	}
	t.Cleanup(func() { getExecutor = prevGetExecutor })
}

// Command is a wrapper around exec.Cmd that incorporates an executor to run it.
// Clients should create commands to execute using CommandContext, then run with
// the desired Command method.
type Command struct {
	cmd      *exec.Cmd
	executor CommandExecutor
}

// CommandContext returns the Cmd struct to execute the named program with
// the given arguments using the given context using the current executor.
//
// See the os/exec package for more details.
func CommandContext(ctx context.Context, name string, args ...string) *Command {
	cmd := exec.CommandContext(ctx, name, args...)
	return &Command{cmd: cmd, executor: getExecutor()}
}

// SetExecutor sets the executor that will be used when a command is run.
func (c *Command) SetExecutor(executor CommandExecutor) {
	c.executor = executor
}

// Run starts the command and waits for it to complete.
func (c *Command) Run() error {
	return c.executor.Run(c.cmd)
}

// Start starts the command but does not wait for it to complete.
func (c *Command) Start() error {
	return c.executor.Start(c.cmd)
}

// Wait waits for the command to exit.
func (c *Command) Wait() error {
	return c.executor.Wait(c.cmd)
}

// Output runs the command and returns its standard output.
func (c *Command) Output() ([]byte, error) {
	return c.executor.Output(c.cmd)
}

// CombinedOutput runs the command and returns its combined standard output
// and standard error.
func (c *Command) CombinedOutput() ([]byte, error) {
	return c.executor.CombinedOutput(c.cmd)
}
