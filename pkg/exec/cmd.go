// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Defines Exec and Cmd, a replacement for os/exec.Cmd that tests can fake.

package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// These errors have the same text as the os/exec errors for the same cases.
var (
	errAlreadyStarted    = errors.New("exec: already started")
	errNotStarted        = errors.New("exec: not started")
	errWaitCalled        = errors.New("exec: Wait was already called")
	errStdinSet          = errors.New("exec: Stdin already set")
	errStdoutSet         = errors.New("exec: Stdout already set")
	errStderrSet         = errors.New("exec: Stderr already set")
	errStdinPipeStarted  = errors.New("exec: StdinPipe after process started")
	errStdoutPipeStarted = errors.New("exec: StdoutPipe after process started")
	errStderrPipeStarted = errors.New("exec: StderrPipe after process started")
)

// ExitError is an alias of [os/exec.ExitError]. Wait returns it when a
// command exits with a status other than 0.
type ExitError = exec.ExitError

// Error is an alias of [os/exec.Error]. It is returned when an executable
// cannot be found.
type Error = exec.Error

// These variables are the same values as in os/exec, so errors.Is works with
// errors from either package.
var (
	// ErrNotFound is [os/exec.ErrNotFound].
	ErrNotFound = exec.ErrNotFound
	// ErrDot is [os/exec.ErrDot].
	ErrDot = exec.ErrDot
	// ErrWaitDelay is [os/exec.ErrWaitDelay].
	ErrWaitDelay = exec.ErrWaitDelay
)

// LookPath calls [os/exec.LookPath].
func LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Exec creates commands. [NewRealExec] creates real commands. Package
// execstub has fakes for tests.
type Exec interface {
	// CommandContext returns a command that runs name with args, like
	// [os/exec.CommandContext]. ctx must not be nil.
	CommandContext(ctx context.Context, name string, args ...string) *Cmd
}

// realExec is the Exec that NewRealExec returns.
type realExec struct{}

// NewRealExec returns an Exec whose commands run real processes with
// os/exec.
func NewRealExec() Exec {
	return realExec{}
}

// CommandContext returns a Cmd that runs a real process with os/exec.
func (realExec) CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	r := exec.CommandContext(ctx, name, args...)
	return newCmd(ctx, &Cmd{Path: r.Path, Args: r.Args, real: r})
}

// CommandContext returns a Cmd that runs a real process. It is the same as
// NewRealExec().CommandContext. Code that tests must replace the command
// takes an [Exec] instead.
func CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	return NewRealExec().CommandContext(ctx, name, args...)
}

// Cmd is a command to run. It has the fields and methods of
// [os/exec.Cmd], and the same rules apply: set the fields before Start, and
// do not use a Cmd again after Wait. Changes to the fields after Start have
// no effect.
//
// Use an [Exec] to create a Cmd. A Cmd that is not made by an Exec or by
// [NewFakeCmd] cannot run.
type Cmd struct {
	// Path is the path of the command to run. For a real command,
	// CommandContext sets it to the result of LookPath when it finds one.
	Path string

	// Args holds the command-line arguments, with the command name as
	// Args[0].
	Args []string

	// Env is the environment of the process. If Env is nil, the process
	// gets the environment of the current process.
	Env []string

	// Dir is the working directory of the process. If Dir is empty, the
	// process runs in the current directory.
	Dir string

	// Stdin is the standard input of the process. If Stdin is nil, the
	// process reads from the null device.
	Stdin io.Reader

	// Stdout and Stderr are the standard output and standard error of the
	// process. If one is nil, that output goes to the null device. If both
	// are the same writer, at most one goroutine at a time writes to it.
	Stdout io.Writer
	Stderr io.Writer

	// ExtraFiles are more open files for a real process. A fake command
	// ignores them.
	ExtraFiles []*os.File

	// SysProcAttr holds optional, operating system-specific attributes for a
	// real process. A fake command ignores it.
	SysProcAttr *syscall.SysProcAttr

	// Process is the process, after Start.
	Process *Process

	// ProcessState holds information about the exited process. It is nil
	// until Wait or Run returns.
	ProcessState ProcessState

	// Cancel is called when the context is done before the command exits.
	// By default, it calls Process.Kill. If Cancel is nil, the command
	// continues to run. See [os/exec.Cmd.Cancel] for the full rules.
	Cancel func() error

	// WaitDelay bounds the time that Wait waits for the I/O of a real
	// process. See [os/exec.Cmd.WaitDelay]. A fake command ignores it.
	WaitDelay time.Duration

	//nolint:containedctx // Why: like os/exec.Cmd, a Cmd keeps the context of CommandContext until Wait.
	ctx context.Context

	// real is the os/exec command of a real Cmd, and nil for a fake.
	real *exec.Cmd

	// runner runs a fake Cmd, and is nil for a real one.
	runner Runner

	// fake is the running fake process, after Start.
	fake *fakeProcess

	// childIOFiles are the ends of pipes that belong to the process. They
	// are closed when the process no longer needs them.
	childIOFiles []io.Closer

	// parentIOPipes are the ends of pipes that the caller reads or writes.
	// They are closed after Wait.
	parentIOPipes []io.Closer
}

// NewFakeCmd returns a Cmd that runs r instead of a process. It sets Path to
// name, Args to name and args, and Cancel to call Process.Kill, as
// CommandContext does. ctx must not be nil.
//
// Packages such as execstub use it to build fakes. Start runs r in a new
// goroutine and sets Process to a [Process] with a fixed Pid that no real
// process can have. Wait returns the error from r. The ProcessState
// shows the exit status of an [*ExitError] from r, or exit status 0 for nil
// or for an error of another type.
func NewFakeCmd(ctx context.Context, r Runner, name string, args ...string) *Cmd {
	return newCmd(ctx, &Cmd{Path: name, Args: append([]string{name}, args...), runner: r})
}

// Run starts the command and waits for it to exit. See [os/exec.Cmd.Run].
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Start starts the command, but does not wait for it to exit. See
// [os/exec.Cmd.Start].
func (c *Cmd) Start() error {
	if c.Process != nil {
		return errAlreadyStarted
	}

	var err error
	if c.real != nil {
		err = c.startReal()
	} else {
		err = c.startFake()
	}

	if err != nil {
		closeAll(c.parentIOPipes)
		c.parentIOPipes = nil
	}
	// A fake takes the child ends, and closes them when it exits.
	closeAll(c.childIOFiles)
	c.childIOFiles = nil

	return err
}

// Wait waits for the command to exit, and for the copy to and from its
// standard input, output and error to complete. It sets ProcessState. See
// [os/exec.Cmd.Wait].
func (c *Cmd) Wait() error {
	if c.Process == nil {
		return errNotStarted
	}
	if c.ProcessState != nil {
		return errWaitCalled
	}

	var err error
	if c.real != nil {
		err = c.real.Wait()
		// Do not store a typed nil, so that ProcessState != nil stays false.
		if state := c.real.ProcessState; state != nil {
			c.ProcessState = state
		}
	} else {
		err = c.waitFake()
	}

	closeAll(c.parentIOPipes)
	c.parentIOPipes = nil

	return err
}

// Output runs the command and returns its standard output. If Stderr is nil
// and the command fails with an [*ExitError], the error holds the start and
// end of the standard error. See [os/exec.Cmd.Output].
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errStdoutSet
	}
	var stdout bytes.Buffer
	c.Stdout = &stdout

	var stderr *prefixSuffixSaver
	if c.Stderr == nil {
		stderr = &prefixSuffixSaver{N: 32 << 10}
		c.Stderr = stderr
	}

	err := c.Run()
	if stderr != nil {
		if ee, ok := err.(*ExitError); ok { //nolint:errorlint // Why: Run returns a bare *ExitError, as in os/exec.
			ee.Stderr = stderr.Bytes()
		}
	}

	return stdout.Bytes(), err
}

// CombinedOutput runs the command and returns its standard output and
// standard error together. See [os/exec.Cmd.CombinedOutput].
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errStdoutSet
	}
	if c.Stderr != nil {
		return nil, errStderrSet
	}

	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()

	return b.Bytes(), err
}

// StdinPipe returns a pipe to the standard input of the command. Wait
// closes it after the command exits. See [os/exec.Cmd.StdinPipe].
func (c *Cmd) StdinPipe() (io.WriteCloser, error) {
	if c.Stdin != nil {
		return nil, errStdinSet
	}
	if c.Process != nil {
		return nil, errStdinPipeStarted
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdin = pr
	c.trackPipe(pr, pw)

	return pw, nil
}

// StdoutPipe returns a pipe from the standard output of the command. Wait
// closes it after the command exits, so read all of it before you call
// Wait. See [os/exec.Cmd.StdoutPipe].
func (c *Cmd) StdoutPipe() (io.ReadCloser, error) {
	if c.Stdout != nil {
		return nil, errStdoutSet
	}
	if c.Process != nil {
		return nil, errStdoutPipeStarted
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdout = pw
	c.trackPipe(pw, pr)

	return pr, nil
}

// StderrPipe returns a pipe from the standard error of the command. Wait
// closes it after the command exits, so read all of it before you call
// Wait. See [os/exec.Cmd.StderrPipe].
func (c *Cmd) StderrPipe() (io.ReadCloser, error) {
	if c.Stderr != nil {
		return nil, errStderrSet
	}
	if c.Process != nil {
		return nil, errStderrPipeStarted
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stderr = pw
	c.trackPipe(pw, pr)

	return pr, nil
}

// Environ returns a copy of the environment that the command would run with
// as it is now configured. See [os/exec.Cmd.Environ].
func (c *Cmd) Environ() []string {
	return (&exec.Cmd{Env: c.Env, Dir: c.Dir, SysProcAttr: c.SysProcAttr}).Environ()
}

// String returns the command line, for logs and errors: Path followed by the
// arguments after Args[0].
func (c *Cmd) String() string {
	if len(c.Args) < 2 {
		return c.Path
	}
	return c.Path + " " + strings.Join(c.Args[1:], " ")
}

// newCmd sets the context and the default Cancel of c, and returns c.
func newCmd(ctx context.Context, c *Cmd) *Cmd {
	c.ctx = ctx
	c.Cancel = func() error { return c.Process.Kill() }
	return c
}

// startReal copies the fields to the os/exec command and starts it.
func (c *Cmd) startReal() error {
	r := c.real
	r.Path, r.Args, r.Env, r.Dir = c.Path, c.Args, c.Env, c.Dir
	r.Stdin, r.Stdout, r.Stderr = c.Stdin, c.Stdout, c.Stderr
	r.ExtraFiles, r.SysProcAttr, r.WaitDelay = c.ExtraFiles, c.SysProcAttr, c.WaitDelay

	// os/exec can call Cancel from its own goroutine before c.Process is
	// set. Make Cancel wait for it, so that Cancel can use c.Process.
	started := make(chan struct{})
	r.Cancel = nil
	if cancel := c.Cancel; cancel != nil {
		r.Cancel = func() error {
			<-started
			return cancel()
		}
	}

	if err := r.Start(); err != nil {
		return err
	}
	c.Process = &Process{Pid: r.Process.Pid, impl: osProcess{r.Process}}
	close(started)

	return nil
}

// trackPipe records the ends of a new pipe, so that Start and Wait close
// them at the same times as os/exec does.
func (c *Cmd) trackPipe(child, parent *os.File) {
	c.childIOFiles = append(c.childIOFiles, child)
	c.parentIOPipes = append(c.parentIOPipes, parent)
}

// closeAll closes each of cs and ignores the errors, as os/exec does.
func closeAll(cs []io.Closer) {
	for _, c := range cs {
		c.Close()
	}
}
