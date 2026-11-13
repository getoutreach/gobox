// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for the matcher-based Stubs.

package execstub

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sync"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/pkg/exec"
)

// fakeTB records the failures and cleanups of the Stubs under test, so a
// test can check them without failing itself.
type fakeTB struct {
	testing.TB

	mu       sync.Mutex
	failures []string
	cleanups []func()
}

// Helper does nothing.
func (*fakeTB) Helper() {}

// Errorf records a failure.
func (f *fakeTB) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failures = append(f.failures, fmt.Sprintf(format, args...))
}

// Cleanup records fn, for runCleanups.
func (f *fakeTB) Cleanup(fn func()) {
	f.cleanups = append(f.cleanups, fn)
}

// runCleanups calls the cleanup functions in reverse order, as testing does,
// and returns the failures.
func (f *fakeTB) runCleanups() []string {
	for _, fn := range slices.Backward(f.cleanups) {
		fn()
	}
	return f.failures
}

func TestStubsExact(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(Exact("gh", "pr", "list", "--state", "open"), Result{Stdout: "[]"})

	out, err := s.CommandContext(t.Context(), "gh", "pr", "list", "--state", "open").Output()

	assert.NilError(t, err)
	assert.Equal(t, string(out), "[]")
}

func TestStubsRegexp(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(Regexp(regexp.MustCompile(`^git (fetch|pull)\b`)), Result{Stderr: "denied\n", ExitCode: 128})

	_, err := s.CommandContext(t.Context(), "git", "pull", "origin").Output()

	exitErr := exitErrorOf(t, err)
	assert.Equal(t, exitErr.ExitCode(), 128)
	assert.Equal(t, string(exitErr.Stderr), "denied\n")
}

func TestStubsMatchFuncAndFunc(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(
		MatchFunc("kubectl with a context", func(name string, args []string) bool {
			return name == "kubectl" && slices.Contains(args, "--context")
		}),
		Result{Func: func(_ context.Context, cmd *exec.Cmd, _ io.Reader, stdout, _ io.Writer) error {
			_, err := io.WriteString(stdout, "dir="+cmd.Dir)
			return err
		}},
	)

	cmd := s.CommandContext(t.Context(), "kubectl", "--context", "dev", "get", "pods")
	cmd.Dir = "/work"
	out, err := cmd.Output()

	assert.NilError(t, err)
	assert.Equal(t, string(out), "dir=/work")
}

func TestStubsAnswerOnceInOrder(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(Exact("git", "push"), Result{Stderr: "rejected\n", ExitCode: 1})
	s.Register(Exact("git", "push"), Result{Stdout: "pushed\n"})

	_, err := s.CommandContext(t.Context(), "git", "push").Output()
	assert.Equal(t, exitErrorOf(t, err).ExitCode(), 1)

	out, err := s.CommandContext(t.Context(), "git", "push").Output()
	assert.NilError(t, err)
	assert.Equal(t, string(out), "pushed\n")
}

func TestStubsUnexpectedCommandFailsTest(t *testing.T) {
	t.Parallel()

	tb := &fakeTB{TB: t}
	s := NewStubs(tb)
	s.Register(Exact("gh", "pr", "list"), Result{})

	_, err := s.CommandContext(t.Context(), "gh", "pr", "list").Output()
	assert.NilError(t, err)

	// A second call has no stub left, and other args never match.
	for _, args := range [][]string{{"pr", "list"}, {"pr", "view"}} {
		_, err = s.CommandContext(t.Context(), "gh", args...).Output()
		assert.ErrorIs(t, err, errUnexpectedCommand)
	}

	assert.DeepEqual(t, tb.runCleanups(), []string{
		"execstub: unexpected command: gh pr list",
		"execstub: unexpected command: gh pr view",
	})
}

func TestStubsUnusedStubFailsTest(t *testing.T) {
	t.Parallel()

	tb := &fakeTB{TB: t}
	s := NewStubs(tb)
	s.Register(Exact("gh", "pr", "list"), Result{})
	s.Register(Regexp(regexp.MustCompile(`^gh pr merge`)), Result{})
	s.Register(MatchFunc("any gh command", func(name string, _ []string) bool { return name == "gh" }), Result{})

	assert.NilError(t, s.CommandContext(t.Context(), "gh", "pr", "list").Run())

	assert.DeepEqual(t, tb.runCleanups(), []string{
		"execstub: stub never used: regexp ^gh pr merge",
		"execstub: stub never used: any gh command",
	})
}

func TestStubsConcurrentCommands(t *testing.T) {
	t.Parallel()

	const n = 8
	s := NewStubs(t)
	for range n {
		s.Register(Exact("echo", "hi"), Result{Stdout: "hi\n"})
	}

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			out, err := s.CommandContext(t.Context(), "echo", "hi").Output()
			assert.Check(t, err)
			assert.Check(t, string(out) == "hi\n")
		})
	}
	wg.Wait()
}
