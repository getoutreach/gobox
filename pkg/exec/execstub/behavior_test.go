// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Behavior tests that run against the real Exec and the fakes.

package execstub

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/exec"
)

// behavior is a command that each implementation runs in the same way. The
// real Exec runs script with sh. The fakes give the canned result, or run fn
// if it is set.
type behavior struct {
	script string
	stdout string
	stderr string
	code   int
	fn     RunFunc
}

// The behaviors that the tests use.
var (
	// failing writes to both streams and exits with status 3.
	failing = behavior{script: "echo hello; echo boom >&2; exit 3", stdout: "hello\n", stderr: "boom\n", code: 3}

	// succeeding writes to stdout and exits with status 0.
	succeeding = behavior{script: "echo hello", stdout: "hello\n"}

	// catting copies stdin to stdout.
	catting = behavior{script: "cat", fn: func(_ context.Context, _ *exec.Cmd, stdin io.Reader, stdout, _ io.Writer) error {
		_, err := io.Copy(stdout, stdin)
		return err
	}}

	// sleeping runs until it is killed.
	sleeping = behavior{script: "exec sleep 60", fn: func(ctx context.Context, _ *exec.Cmd, _ io.Reader, _, _ io.Writer) error {
		<-ctx.Done()
		return nil
	}}
)

// commands returns a function for each implementation of b, by name. The
// function returns a command from that implementation for a context. Call
// each function once, because each Stubs has one stub, which it checks at
// the end of t.
func (b behavior) commands(t *testing.T) map[string]func(context.Context) *exec.Cmd {
	t.Helper()

	stubs := func(r Result) exec.Exec {
		s := NewStubs(t)
		s.Register(MatchFunc("any command", func(string, []string) bool { return true }), r)
		return s
	}

	impls := map[string]exec.Exec{"real": exec.NewRealExec()}
	if b.fn != nil {
		impls["Stubs.Func"] = stubs(Result{Func: b.fn})
	} else {
		impls["Stubs"] = stubs(Result{Stdout: b.stdout, Stderr: b.stderr, ExitCode: b.code})
		impls["StubbedCmdWithCode"] = NewStubbedCmdWithCode(b.stdout, b.stderr, b.code)
		impls["FuncExec"] = NewFuncExec(func(*exec.Cmd, string, []string) ([]byte, error) {
			if b.code == 0 {
				return []byte(b.stdout), nil
			}
			return []byte(b.stdout), NewExitError(b.code, b.stderr)
		})
	}

	commands := make(map[string]func(context.Context) *exec.Cmd, len(impls))
	for name, ex := range impls {
		commands[name] = func(ctx context.Context) *exec.Cmd {
			return ex.CommandContext(ctx, "sh", "-c", b.script)
		}
	}

	return commands
}

// exitErrorOf returns the *exec.ExitError that err is.
func exitErrorOf(t *testing.T, err error) *exec.ExitError {
	t.Helper()

	var exitErr *exec.ExitError
	assert.Assert(t, errors.As(err, &exitErr), "expected an *exec.ExitError, got %T: %v", err, err)

	return exitErr
}

// assertKilled checks that err and the state of cmd show a kill.
func assertKilled(t *testing.T, cmd *exec.Cmd, err error) {
	t.Helper()

	exitErr := exitErrorOf(t, err)
	assert.Equal(t, exitErr.Error(), "signal: killed")
	assert.Equal(t, exitErr.ExitCode(), -1)
	assert.Equal(t, cmd.ProcessState.String(), "signal: killed")
	assert.Equal(t, cmd.ProcessState.Exited(), false)
	assert.Equal(t, cmd.ProcessState.ExitCode(), -1)
}

func TestBehaviorRunWritesToWriters(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr

			exitErr := exitErrorOf(t, cmd.Run())

			assert.Equal(t, exitErr.ExitCode(), 3)
			assert.Equal(t, len(exitErr.Stderr), 0, "expected stderr off the error")
			assert.Equal(t, stdout.String(), "hello\n")
			assert.Equal(t, stderr.String(), "boom\n")
		})
	}
}

func TestBehaviorRunWithoutWriters(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			exitErr := exitErrorOf(t, cmd.Run())

			assert.Equal(t, len(exitErr.Stderr), 0, "expected stderr off the error")
		})
	}
}

func TestBehaviorOutputPutsStderrOnError(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			out, err := cmd.Output()

			exitErr := exitErrorOf(t, err)
			assert.Equal(t, string(out), "hello\n")
			assert.Equal(t, string(exitErr.Stderr), "boom\n")
			assert.Equal(t, exitErr.ExitCode(), 3)
			assert.Equal(t, exitErr.Error(), "exit status 3")
		})
	}
}

func TestBehaviorOutputWithStderrWriter(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			out, err := cmd.Output()

			assert.Equal(t, string(out), "hello\n")
			assert.Equal(t, len(exitErrorOf(t, err).Stderr), 0, "expected stderr off the error")
			assert.Equal(t, stderr.String(), "boom\n")
		})
	}
}

func TestBehaviorOutputSucceeds(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			out, err := cmd.Output()

			assert.NilError(t, err)
			assert.Equal(t, string(out), "hello\n")
			assert.Assert(t, cmd.ProcessState.Success())
		})
	}
}

func TestBehaviorOutputRejectsStdout(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			cmd.Stdout = &bytes.Buffer{}

			_, err := cmd.Output()

			assert.Error(t, err, "exec: Stdout already set")
		})
	}
}

func TestBehaviorCombinedOutput(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			out, err := cmd.CombinedOutput()

			assert.Equal(t, string(out), "hello\nboom\n")
			assert.Equal(t, len(exitErrorOf(t, err).Stderr), 0, "expected stderr off the error")
		})
	}
}

func TestBehaviorCombinedOutputRejectsStderr(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			cmd.Stderr = &bytes.Buffer{}

			_, err := cmd.CombinedOutput()

			assert.Error(t, err, "exec: Stderr already set")
		})
	}
}

func TestBehaviorStartWait(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			assert.Assert(t, cmd.Process == nil)

			assert.NilError(t, cmd.Start())
			assert.Assert(t, cmd.Process != nil)
			assert.Assert(t, cmd.ProcessState == nil, "expected no ProcessState before Wait, got %#v", cmd.ProcessState)

			assert.Equal(t, exitErrorOf(t, cmd.Wait()).ExitCode(), 3)
			state := cmd.ProcessState
			assert.Assert(t, state != nil)
			assert.Equal(t, state.ExitCode(), 3)
			assert.Equal(t, state.Exited(), true)
			assert.Equal(t, state.Success(), false)
			assert.Equal(t, state.String(), "exit status 3")
			assert.Equal(t, state.Pid(), cmd.Process.Pid)
		})
	}
}

func TestBehaviorStartTwice(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			assert.NilError(t, cmd.Start())
			assert.Error(t, cmd.Start(), "exec: already started")
			assert.NilError(t, cmd.Wait())
		})
	}
}

func TestBehaviorWaitWithoutStart(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			assert.Error(t, cmd.Wait(), "exec: not started")
		})
	}
}

func TestBehaviorWaitTwice(t *testing.T) {
	t.Parallel()

	for name, command := range failing.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			assert.NilError(t, cmd.Start())
			exitErrorOf(t, cmd.Wait())

			assert.Error(t, cmd.Wait(), "exec: Wait was already called")
		})
	}
}

func TestBehaviorOutputPipes(t *testing.T) {
	t.Parallel()

	pipes := map[string]struct {
		pipe func(*exec.Cmd) (io.ReadCloser, error)
		want string
	}{
		"StdoutPipe": {(*exec.Cmd).StdoutPipe, "hello\n"},
		"StderrPipe": {(*exec.Cmd).StderrPipe, "boom\n"},
	}

	for pipeName, tc := range pipes {
		for name, command := range failing.commands(t) {
			t.Run(pipeName+"/"+name, func(t *testing.T) {
				t.Parallel()

				cmd := command(t.Context())
				pipe, err := tc.pipe(cmd)
				assert.NilError(t, err)
				assert.NilError(t, cmd.Start())

				out, err := io.ReadAll(pipe)
				assert.NilError(t, err)

				assert.Equal(t, exitErrorOf(t, cmd.Wait()).ExitCode(), 3)
				assert.Equal(t, string(out), tc.want)
			})
		}
	}
}

func TestBehaviorStdinPipe(t *testing.T) {
	t.Parallel()

	for name, command := range catting.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			stdin, err := cmd.StdinPipe()
			assert.NilError(t, err)
			assert.NilError(t, cmd.Start())

			_, err = io.WriteString(stdin, "from the pipe")
			assert.NilError(t, err)
			assert.NilError(t, stdin.Close())

			assert.NilError(t, cmd.Wait())
			assert.Equal(t, stdout.String(), "from the pipe")
		})
	}
}

func TestBehaviorStdinReader(t *testing.T) {
	t.Parallel()

	for name, command := range catting.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			cmd.Stdin = strings.NewReader("from a reader")

			out, err := cmd.Output()

			assert.NilError(t, err)
			assert.Equal(t, string(out), "from a reader")
		})
	}
}

func TestBehaviorPipeAfterStart(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			assert.NilError(t, cmd.Start())

			_, err := cmd.StdoutPipe()
			assert.Error(t, err, "exec: StdoutPipe after process started")
			_, err = cmd.StderrPipe()
			assert.Error(t, err, "exec: StderrPipe after process started")
			_, err = cmd.StdinPipe()
			assert.Error(t, err, "exec: StdinPipe after process started")

			assert.NilError(t, cmd.Wait())
		})
	}
}

func TestBehaviorContextDoneBeforeStart(t *testing.T) {
	t.Parallel()

	for name, command := range succeeding.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			cmd := command(ctx)

			assert.ErrorIs(t, cmd.Start(), context.Canceled)
			assert.Assert(t, cmd.Process == nil)
		})
	}
}

func TestBehaviorContextCanceledWhileRunning(t *testing.T) {
	t.Parallel()

	for name, command := range sleeping.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cmd := command(ctx)

			assert.NilError(t, cmd.Start())
			cancel()

			assertKilled(t, cmd, cmd.Wait())
		})
	}
}

func TestBehaviorKill(t *testing.T) {
	t.Parallel()

	for name, command := range sleeping.commands(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := command(t.Context())

			assert.NilError(t, cmd.Start())
			assert.NilError(t, cmd.Process.Kill())

			assertKilled(t, cmd, cmd.Wait())
			assert.ErrorIs(t, cmd.Process.Kill(), os.ErrProcessDone)
		})
	}
}
