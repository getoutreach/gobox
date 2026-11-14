// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Fakes of exec.Exec with canned results, a function, or a spy.

// Package execstub has fake implementations of
// [github.com/getoutreach/gobox/pkg/exec.Exec] for tests. Use it only in
// tests.
//
// Code under test takes an exec.Exec value and creates its commands with it.
// A test passes one of these fakes instead of exec.NewRealExec:
//
//   - [NewStubbedCmd] and [NewStubbedCmdWithCode] give the same result for
//     every command.
//   - [NewFuncExec] calls a function to get the result of each command.
//   - [NewStubs] matches each command against registered stubs, and fails
//     the test on a command that no stub matches and on a stub that no
//     command used. [InOrder] also checks the order of the commands.
//   - [NewSpyExec] wraps another Exec and records every command.
//
// A stub of Stubs gives canned output and an exit code, or calls a [RunFunc]
// first, for example to write a file in the folder of the command, and then
// gives the canned result. A stub with [Result.Real] runs the real command,
// so a test can let some commands run while it stubs the others. [Exact]
// matches any one argument at the position of [AnyArg].
//
// A fake command does not start a process. Start runs the fake in a new
// goroutine and Wait returns its result, with the same rules as os/exec for
// Stdout, Stderr, the pipe methods, Output, CombinedOutput, the context and
// Cancel. [exec.Cmd.Process] has a fixed Pid. [Signals] returns the signals
// that were sent to it. After [exec.Process.Kill], or after the context is
// done with the default Cancel, Wait returns an *exec.ExitError for
// "signal: killed". A fake ignores ExtraFiles, SysProcAttr and WaitDelay,
// and the Sys and SysUsage methods of its ProcessState return nil.
//
// # POSIX only
//
// The standard library cannot make an *exec.ExitError with an exit status
// without a process. So [NewExitError], the failing stubs and a killed fake
// run /bin/sh once for each exit status, and keep the result for later
// errors. This needs a POSIX system. On other systems, these return an error
// that tells why.
package execstub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"weak"

	"github.com/getoutreach/gobox/pkg/exec"
)

// errInvalidExitCode is returned by NewExitError for a code outside 1 to 255.
// Zero means success, a code above 255 does not fit in one byte, and a
// negative code is not a valid exit status.
var errInvalidExitCode = errors.New("execstub: exit code must be between 1 and 255")

// errShellSucceeded is the cause when /bin/sh exits with status 0, which
// gives no *exec.ExitError.
var errShellSucceeded = errors.New("execstub: /bin/sh exited with status 0")

// The /bin/sh scripts that make a process state. The exit code goes in the
// environment, so that the command line is constant.
const (
	exitScript = `exit "$EXECSTUB_EXIT_CODE"`
	killScript = `kill -s KILL $$`
)

// killKey is the key in states for "signal: killed". It is 0 because
// NewExitError never caches exit status 0.
const killKey = 0

// states caches one process state for each exit code, and one for killKey.
// A process state does not change, so all errors with that status share it.
var states sync.Map //nolint:gochecknoglobals // Why: a cache, so that each exit status starts only one process.

// runners maps each fake Cmd to its runner, for Signals. The key is a weak
// pointer, and a cleanup removes the entry when the Cmd is collected.
var runners sync.Map //nolint:gochecknoglobals // Why: Signals finds the runner of a Cmd, which keeps it in an unexported field.

// RunFunc does the work of a fake command.
//
// cmd is the started command, so RunFunc can read Dir, Env and the other
// fields; it must not change them. RunFunc reads the input of the command
// from stdin, and writes its output to stdout and stderr. These are never
// nil, and stdout and stderr can be the same writer.
//
// ctx is done when the process gets os.Kill, from [exec.Process.Kill] or
// from the default Cancel when the context of the command is done. A
// RunFunc that blocks must return when ctx is done. After a kill, Wait
// returns the error for "signal: killed", whatever RunFunc returns.
//
// RunFunc returns nil for success, or an error for Wait, usually from
// [NewExitError]. If the error is an *exec.ExitError with Stderr, that text
// goes to stderr, as the output of a real process does.
type RunFunc func(ctx context.Context, cmd *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) error

// NewExitError returns the error that a real command returns when it exits
// with code after it writes stderr. ExitCode returns code, Error returns
// "exit status N", and Stderr holds stderr.
//
// The standard library cannot make the process state of the error, so the
// first call for each code runs `/bin/sh -c "exit N"` and keeps the result.
// This needs a POSIX system. If code is outside 1 to 255, or /bin/sh cannot
// run, NewExitError returns an error that wraps the cause.
func NewExitError(code int, stderr string) error {
	if code < 1 || code > 255 {
		return fmt.Errorf("%w: got %d", errInvalidExitCode, code)
	}

	state, err := shellState(code)
	if err != nil {
		return err
	}

	return &exec.ExitError{ProcessState: state, Stderr: []byte(stderr)}
}

// NewStubbedCmd returns a fake whose commands all write stdout. A non-empty
// stderr makes every command fail with exit status 1 and an *exec.ExitError
// for that text. An empty stderr makes every command succeed. To fail with
// another exit status, or with no stderr text, use NewStubbedCmdWithCode.
func NewStubbedCmd(stdout, stderr string) exec.Exec {
	code := 0
	if stderr != "" {
		code = 1
	}

	return NewStubbedCmdWithCode(stdout, stderr, code)
}

// NewStubbedCmdWithCode returns a fake whose commands all write stdout and
// exit with code. A code from 1 to 255 makes every command fail with an
// *exec.ExitError for that status and stderr, as NewExitError does. Code 0
// makes every command succeed and ignores stderr. Any other code makes every
// command return the error from NewExitError. The error comes from Wait, not
// from this function.
func NewStubbedCmdWithCode(stdout, stderr string, code int) exec.Exec {
	if code == 0 {
		stderr = ""
	}
	fn := canned(stdout, stderr, code)

	return funcExec(func(string, []string) RunFunc { return fn })
}

// NewFuncExec returns a fake whose commands call fn to get their result. fn
// gets the started command, so it can read Dir, Env and Stdin, and the name
// and args from CommandContext. fn returns the standard output of the
// command and, on failure, an error such as an *exec.ExitError from
// NewExitError, whose Stderr holds the standard error. A wrapped
// *exec.ExitError is returned as it is, and its Stderr is not written.
func NewFuncExec(fn func(cmd *exec.Cmd, name string, args []string) ([]byte, error)) exec.Exec {
	return funcExec(func(name string, args []string) RunFunc {
		return func(_ context.Context, cmd *exec.Cmd, _ io.Reader, stdout, _ io.Writer) error {
			out, err := fn(cmd, name, args)
			write(stdout, out)
			return err
		}
	})
}

// Signals returns the signals that were sent to the process of cmd, in
// order. This includes os.Kill from [exec.Process.Kill] and from the default
// Cancel. It returns nil if cmd is not a fake from this package.
func Signals(cmd *exec.Cmd) []os.Signal {
	r, ok := runners.Load(weak.Make(cmd))
	if !ok {
		return nil
	}

	return r.(*runner).recordedSignals()
}

// Call is a command that a SpyExec created.
type Call struct {
	// Name is the name given to CommandContext.
	Name string
	// Args are the arguments given to CommandContext.
	Args []string
	// Cmd is the command that the wrapped Exec returned.
	Cmd *exec.Cmd
}

// SpyExec wraps another Exec and records every command created through it,
// so a test can check the commands that code created and still get working
// commands from the wrapped Exec.
type SpyExec struct {
	// exec creates the commands.
	exec exec.Exec

	// mu guards Calls while commands are created.
	mu sync.Mutex
	// Calls holds every command created through the SpyExec, in order.
	// Read it after the code under test has returned.
	Calls []Call
}

// NewSpyExec returns a SpyExec that wraps ex.
func NewSpyExec(ex exec.Exec) *SpyExec {
	return &SpyExec{exec: ex}
}

// CommandContext records name and args, then returns the command from the
// wrapped Exec.
func (s *SpyExec) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	// A test failure from wrapped Stubs then skips this frame too.
	if st, ok := s.exec.(*Stubs); ok {
		st.tb.Helper()
	}

	cmd := s.exec.CommandContext(ctx, name, args...)

	s.mu.Lock()
	s.Calls = append(s.Calls, Call{Name: name, Args: append([]string(nil), args...), Cmd: cmd})
	s.mu.Unlock()

	return cmd
}

// funcExec is an exec.Exec that gets a RunFunc for each command.
type funcExec func(name string, args []string) RunFunc

// CommandContext returns a fake command that runs the RunFunc for name and
// args.
func (f funcExec) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return newCmd(ctx, f(name, args), name, args)
}

// runner is the exec.Runner of every fake in this package.
type runner struct {
	fn RunFunc

	// mu guards the fields below.
	mu sync.Mutex

	// signals are the signals sent to the process, in order.
	signals []os.Signal

	// killed is true after os.Kill.
	killed bool

	// cancel stops the ctx of fn. It is nil until Run starts.
	cancel context.CancelFunc
}

// Run calls fn, and then returns the error for Wait.
func (r *runner) Run(cmd *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.mu.Lock()
	r.cancel = cancel
	if r.killed {
		cancel()
	}
	r.mu.Unlock()

	err := r.fn(ctx, cmd, stdin, stdout, stderr)

	r.mu.Lock()
	killed := r.killed
	r.mu.Unlock()
	if killed {
		return killedError()
	}

	return deliverStderr(err, stderr)
}

// Signal records sig. For os.Kill, it also stops the ctx of fn.
func (r *runner) Signal(sig os.Signal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.signals = append(r.signals, sig)
	if sig == os.Kill {
		r.killed = true
		if r.cancel != nil {
			r.cancel()
		}
	}

	return nil
}

// recordedSignals returns a copy of the signals.
func (r *runner) recordedSignals() []os.Signal {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]os.Signal(nil), r.signals...)
}

// newCmd returns a fake command that runs fn, and records it for Signals.
func newCmd(ctx context.Context, fn RunFunc, name string, args []string) *exec.Cmd {
	r := &runner{fn: fn}
	cmd := exec.NewCmd(ctx, r, name, args...)

	key := weak.Make(cmd)
	runners.Store(key, r)
	runtime.AddCleanup(cmd, func(k weak.Pointer[exec.Cmd]) { runners.Delete(k) }, key)

	return cmd
}

// canned returns a RunFunc that writes stdout and exits with code. For code
// 0, it writes stderr and succeeds. For another code, it returns the error
// from NewExitError, which carries stderr.
func canned(stdout, stderr string, code int) RunFunc {
	return func(_ context.Context, _ *exec.Cmd, _ io.Reader, stdoutW, stderrW io.Writer) error {
		write(stdoutW, []byte(stdout))
		if code == 0 {
			write(stderrW, []byte(stderr))
			return nil
		}

		//nolint:contextcheck // Why: a kill cancels ctx, but the exit status does not depend on it.
		return NewExitError(code, stderr)
	}
}

// deliverStderr writes the Stderr of a bare *exec.ExitError to w, and
// returns a copy of the error without it, as a real process writes its
// standard error to the writer and not to the error. The copy matters,
// because a test can return the same error for many commands, and Output
// sets Stderr on the error that it gets. Other errors are returned as they
// are, because os/exec returns a bare *exec.ExitError.
func deliverStderr(err error, w io.Writer) error {
	ee, ok := err.(*exec.ExitError) //nolint:errorlint // Why: os/exec returns a bare *ExitError, so wrapping is not modeled.
	if !ok {
		return err
	}

	write(w, ee.Stderr)
	errCopy := *ee
	errCopy.Stderr = nil

	return &errCopy
}

// write writes b to w, as a real process writes its output. A real process
// does not report write errors to the caller, so write ignores them.
func write(w io.Writer, b []byte) {
	if len(b) > 0 {
		w.Write(b) //nolint:errcheck // Why: as for a real process, write errors are not reported.
	}
}

// killedError returns a new *exec.ExitError for "signal: killed".
func killedError() error {
	state, err := shellState(killKey)
	if err != nil {
		return err
	}

	return &exec.ExitError{ProcessState: state}
}

// shellState returns the process state of /bin/sh for key, which is an exit
// code or killKey. The first call for each key runs /bin/sh.
func shellState(key int) (*os.ProcessState, error) {
	if state, ok := states.Load(key); ok {
		return state.(*os.ProcessState), nil
	}

	script := exitScript
	if key == killKey {
		script = killScript
	}
	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", script)
	cmd.Env = []string{"EXECSTUB_EXIT_CODE=" + strconv.Itoa(key)}

	err := cmd.Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		if err == nil {
			err = errShellSucceeded
		}
		return nil, fmt.Errorf("execstub: run /bin/sh to get the process state for %q: %w", script, err)
	}
	state, _ := states.LoadOrStore(key, ee.ProcessState)

	return state.(*os.ProcessState), nil
}
