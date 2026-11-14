// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Runs a fake Cmd with a Runner instead of a process.

package exec

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// fakePid is the Pid of every fake process. It is larger than the largest
// Pid on Linux (2^22) and on macOS, and it is odd, so it is not a Windows
// process ID.
const fakePid = 1<<22 + 1

// These errors have the same text as the os/exec errors for the same cases.
var (
	errNoCommand  = errors.New("exec: no command")
	errNilContext = errors.New("exec: nil Context")
)

// Runner runs a fake command instead of a process. [NewFakeCmd] makes a Cmd
// from it. Package execstub implements Runner; most code does not use it.
type Runner interface {
	// Run runs the command and returns when the command exits. Start calls
	// Run once, in a new goroutine.
	//
	// c is the started Cmd; Run must not change it. stdin, stdout and
	// stderr are never nil. They come from the Stdin, Stdout and Stderr
	// fields and the pipes of c. stdout and stderr can be the same writer,
	// so Run must not write to both at the same time. Run must not use them
	// after it returns.
	//
	// Run returns nil if the command succeeds. Otherwise it returns the
	// error for Wait to return, which is usually an [*ExitError].
	Run(c *Cmd, stdin io.Reader, stdout, stderr io.Writer) error

	// Signal sends sig to the command. [Process.Signal] and [Process.Kill]
	// call it, from any goroutine, after Start and until Wait returns. This
	// includes the default Cancel, which sends [os.Kill]. A Runner that
	// stops on [os.Kill] should make Run return an *ExitError for
	// "signal: killed", as a real process does.
	Signal(sig os.Signal) error
}

// fakeProcess is the processImpl of a fake command.
type fakeProcess struct {
	runner Runner

	// done is closed when Run returns.
	done chan struct{}

	// err is the result of Run. Read it only after done is closed.
	err error

	// ctxResult receives the result of watchCtx, or is nil if there is no
	// watchCtx goroutine.
	ctxResult chan error

	// mu guards waited.
	mu sync.Mutex

	// waited is true after a wait returned.
	waited bool
}

// signal sends sig to the runner, or returns os.ErrProcessDone after a wait.
func (p *fakeProcess) signal(sig os.Signal) error {
	p.mu.Lock()
	waited := p.waited
	p.mu.Unlock()

	if waited {
		return os.ErrProcessDone
	}
	return p.runner.Signal(sig)
}

// wait waits for Run to return.
func (p *fakeProcess) wait() (ProcessState, error) {
	return p.finish(), nil
}

// finish waits for Run to return, marks the process as waited for, and
// returns its state.
func (p *fakeProcess) finish() ProcessState {
	<-p.done

	p.mu.Lock()
	p.waited = true
	p.mu.Unlock()

	return newFakeState(p.err)
}

// release does nothing, because a fake has no resources to release.
func (*fakeProcess) release() error {
	return nil
}

// startFake starts c.runner in a new goroutine, with the same checks as
// os/exec.Cmd.Start.
func (c *Cmd) startFake() error {
	if c.ctx == nil {
		return errNilContext
	}
	if c.Path == "" {
		return errNoCommand
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}

	stdin := c.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	stdout, stderr := c.Stdout, c.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	p := &fakeProcess{runner: c.runner, done: make(chan struct{})}
	c.fake = p
	c.Process = &Process{Pid: fakePid, impl: p}

	// The fake takes the child ends of the pipes, and closes them when it
	// exits, as the child ends of a real process close when it exits.
	childIOFiles := c.childIOFiles
	c.childIOFiles = nil
	go func() {
		p.err = c.runner.Run(c, stdin, stdout, stderr)
		closeAll(childIOFiles)
		close(p.done)
	}()

	if c.Cancel != nil && c.ctx.Done() != nil {
		p.ctxResult = make(chan error)
		go watchCtx(c.ctx, c.Cancel, p.ctxResult)
	}

	return nil
}

// waitFake waits for the fake process and combines its result with the
// result of watchCtx, as os/exec.Cmd.Wait does.
func (c *Cmd) waitFake() error {
	c.ProcessState = c.fake.finish()

	err := c.fake.err
	if c.fake.ctxResult != nil {
		// Like os/exec, prefer the error of the process over the context.
		if ctxErr := <-c.fake.ctxResult; err == nil {
			err = ctxErr
		}
	}

	return err
}

// fakeState is the ProcessState of a fake command.
type fakeState struct {
	code   int
	exited bool
	desc   string
}

// newFakeState returns the state for the result of Run: the status of a bare
// *ExitError, or exit status 0 for nil or another error.
func newFakeState(err error) fakeState {
	ee, ok := err.(*ExitError) //nolint:errorlint // Why: os/exec returns a bare *ExitError, so wrapping is not modeled.
	if !ok || ee.ProcessState == nil {
		return fakeState{exited: true, desc: "exit status 0"}
	}
	return fakeState{code: ee.ExitCode(), exited: ee.Exited(), desc: ee.String()}
}

// ExitCode returns the exit code, or -1 if a signal stopped the process.
func (s fakeState) ExitCode() int { return s.code }

// Exited reports whether the process exited normally.
func (s fakeState) Exited() bool { return s.exited }

// Pid returns the fixed Pid of fake processes.
func (fakeState) Pid() int { return fakePid }

// String returns the exit status, such as "exit status 1".
func (s fakeState) String() string { return s.desc }

// Success reports whether the process exited with status 0.
func (s fakeState) Success() bool { return s.code == 0 }

// Sys returns nil, because a fake has no system-dependent exit information.
func (fakeState) Sys() any { return nil }

// SysUsage returns nil, because a fake has no resource usage.
func (fakeState) SysUsage() any { return nil }

// SystemTime returns 0.
func (fakeState) SystemTime() time.Duration { return 0 }

// UserTime returns 0.
func (fakeState) UserTime() time.Duration { return 0 }
