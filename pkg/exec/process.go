// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Defines Process and ProcessState, which a fake command can also provide.

package exec

import (
	"errors"
	"os"
	"time"
)

// errNoProcess is returned by the methods of a Process that Start did not
// create.
var errNoProcess = errors.New("exec: Process was not created by Start")

// ProcessState holds information about an exited process. It has the
// methods of [*os.ProcessState], which is the type for a real command.
type ProcessState interface {
	// ExitCode returns the exit code, or -1 if the process did not exit
	// normally, for example because a signal stopped it.
	ExitCode() int
	// Exited reports whether the process exited normally.
	Exited() bool
	// Pid returns the process ID.
	Pid() int
	// String returns a description of the exit status, such as
	// "exit status 1" or "signal: killed".
	String() string
	// Success reports whether the process exited with status 0.
	Success() bool
	// Sys returns system-dependent exit information, or nil for a fake.
	Sys() any
	// SysUsage returns system-dependent resource usage, or nil for a fake.
	SysUsage() any
	// SystemTime returns the system CPU time of the process.
	SystemTime() time.Duration
	// UserTime returns the user CPU time of the process.
	UserTime() time.Duration
}

// Process is a started process. It has the Pid field and the methods of
// [*os.Process]. A real command wraps an *os.Process. A fake command sends
// signals to its [Runner].
type Process struct {
	// Pid is the process ID.
	Pid int

	// impl does the work of the methods.
	impl processImpl
}

// processImpl is the real or fake process behind a Process.
type processImpl interface {
	signal(sig os.Signal) error
	wait() (ProcessState, error)
	release() error
}

// Kill stops the process at once. See [os.Process.Kill].
func (p *Process) Kill() error {
	return p.Signal(os.Kill)
}

// Signal sends a signal to the process. After Wait, it returns
// [os.ErrProcessDone]. See [os.Process.Signal].
func (p *Process) Signal(sig os.Signal) error {
	if p == nil || p.impl == nil {
		return errNoProcess
	}
	return p.impl.signal(sig)
}

// Wait waits for the process to exit. Most code calls [Cmd.Wait] instead,
// which also waits for the I/O to complete. See [os.Process.Wait].
func (p *Process) Wait() (ProcessState, error) {
	if p == nil || p.impl == nil {
		return nil, errNoProcess
	}
	return p.impl.wait()
}

// Release releases the resources of the process. Call it only if you do not
// call Wait. See [os.Process.Release].
func (p *Process) Release() error {
	if p == nil || p.impl == nil {
		return errNoProcess
	}
	if err := p.impl.release(); err != nil {
		return err
	}
	p.Pid = -1 // As os.Process.Release does.

	return nil
}

// osProcess is the processImpl of a real command.
type osProcess struct {
	p *os.Process
}

// signal sends sig to the process.
func (o osProcess) signal(sig os.Signal) error {
	return o.p.Signal(sig)
}

// wait waits for the process, and does not return a typed nil state.
func (o osProcess) wait() (ProcessState, error) {
	state, err := o.p.Wait()
	if state == nil {
		return nil, err
	}
	return state, err
}

// release releases the process.
func (o osProcess) release() error {
	return o.p.Release()
}
