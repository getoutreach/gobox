// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for the matcher-based Stubs.

package execstub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

func TestStubsInOrder(t *testing.T) {
	t.Parallel()

	tb := &fakeTB{TB: t}
	s := NewStubs(tb, InOrder())
	s.Register(Exact("gh", "pr", "list"), Result{})
	s.Register(Exact("gh", "pr", "merge"), Result{})
	s.Register(Exact("gh", "pr", "view"), Result{})
	s.RegisterAnyTimes(Exact("git", "status"), Result{})

	// A stub from RegisterAnyTimes is not in the order.
	assert.NilError(t, s.CommandContext(t.Context(), "git", "status").Run())
	assert.NilError(t, s.CommandContext(t.Context(), "gh", "pr", "list").Run())
	// The stub answers a command out of order, but the test fails.
	assert.NilError(t, s.CommandContext(t.Context(), "gh", "pr", "view").Run())
	assert.NilError(t, s.CommandContext(t.Context(), "gh", "pr", "merge").Run())

	assert.DeepEqual(t, tb.runCleanups(), []string{
		"execstub: command out of order: gh pr view ran before gh pr merge",
	})
}

func TestStubsAnyOrderByDefault(t *testing.T) {
	t.Parallel()

	tb := &fakeTB{TB: t}
	s := NewStubs(tb)
	s.Register(Exact("gh", "pr", "list"), Result{})
	s.Register(Exact("gh", "pr", "merge"), Result{})

	assert.NilError(t, s.CommandContext(t.Context(), "gh", "pr", "merge").Run())
	assert.NilError(t, s.CommandContext(t.Context(), "gh", "pr", "list").Run())

	assert.Equal(t, len(tb.runCleanups()), 0)
}

func TestStubsAnyArg(t *testing.T) {
	t.Parallel()

	tb := &fakeTB{TB: t}
	s := NewStubs(tb)
	s.Register(Exact("npm", "whoami", "--userconfig", AnyArg), Result{Stdout: "acme\n"})
	s.Register(Exact("npm", "view", AnyArg, "version"), Result{})

	out, err := s.CommandContext(t.Context(), "npm", "whoami", "--userconfig", "/tmp/npmrc-123").Output()
	assert.NilError(t, err)
	assert.Equal(t, string(out), "acme\n")

	// AnyArg matches one argument: not zero, and not two.
	for _, args := range [][]string{{"whoami", "--userconfig"}, {"whoami", "--userconfig", "a", "b"}} {
		assert.ErrorIs(t, s.CommandContext(t.Context(), "npm", args...).Run(), errUnexpectedCommand)
	}

	assert.DeepEqual(t, tb.runCleanups(), []string{
		"execstub: unexpected command: npm whoami --userconfig",
		"execstub: unexpected command: npm whoami --userconfig a b",
		"execstub: stub never used: npm view <any> version",
	})
}

func TestStubsFuncThenResult(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// writeFile writes the file "out" in the folder of the command.
	writeFile := func(_ context.Context, cmd *exec.Cmd, _ io.Reader, stdout, _ io.Writer) error {
		_, err := io.WriteString(stdout, "from Func\n")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(cmd.Dir, "out"), []byte("written"), 0o600)
	}
	s := NewStubs(t)
	s.Register(Exact("build"), Result{Func: writeFile, Stdout: "built\n", Stderr: "warning\n", ExitCode: 2})

	cmd := s.CommandContext(t.Context(), "build")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()

	assert.Equal(t, exitErrorOf(t, err).ExitCode(), 2)
	assert.Equal(t, string(out), "from Func\nbuilt\n")
	assert.Equal(t, stderr.String(), "warning\n")
	b, err := os.ReadFile(filepath.Join(dir, "out"))
	assert.NilError(t, err)
	assert.Equal(t, string(b), "written")
}

// errFunc is the error of a failing Func.
var errFunc = errors.New("func failed")

func TestStubsFuncErrorSkipsResult(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(Exact("build"), Result{
		Func: func(context.Context, *exec.Cmd, io.Reader, io.Writer, io.Writer) error {
			return errFunc
		},
		Stdout: "built\n", Stderr: "warning\n",
	})

	out, err := s.CommandContext(t.Context(), "build").CombinedOutput()

	assert.ErrorIs(t, err, errFunc)
	assert.Equal(t, len(out), 0)
}

func TestStubsFuncKilledSkipsResult(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(Exact("sleep"), Result{
		Func: func(ctx context.Context, _ *exec.Cmd, _ io.Reader, _, _ io.Writer) error {
			<-ctx.Done()
			return nil
		},
		Stdout: "too late\n",
	})

	cmd := s.CommandContext(t.Context(), "sleep")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	assert.NilError(t, cmd.Start())
	assert.NilError(t, cmd.Process.Kill())

	assertKilled(t, cmd, cmd.Wait())
	assert.Equal(t, stdout.Len(), 0)
}

func TestStubsReal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s := NewStubs(t)
	s.Register(Exact("sh", "-c", AnyArg), Result{Real: true})
	s.Register(Exact("git", "push"), Result{Stdout: "stubbed\n"})

	cmd := s.CommandContext(t.Context(), "sh", "-c", `pwd -P; echo "$GREETING"; cat; echo err >&2`)
	cmd.Dir = dir
	cmd.Env = []string{"GREETING=hello"}
	cmd.Stdin = strings.NewReader("from stdin\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	assert.NilError(t, cmd.Run())
	assert.Assert(t, Signals(cmd) == nil, "a real command has no recorded signals")
	realDir, err := filepath.EvalSymlinks(dir)
	assert.NilError(t, err)
	assert.Equal(t, stdout.String(), realDir+"\nhello\nfrom stdin\n")
	assert.Equal(t, stderr.String(), "err\n")

	out, err := s.CommandContext(t.Context(), "git", "push").Output()
	assert.NilError(t, err)
	assert.Equal(t, string(out), "stubbed\n")
}

func TestStubsRealKeepsContext(t *testing.T) {
	t.Parallel()

	s := NewStubs(t)
	s.Register(Exact("sleep", "60"), Result{Real: true})

	ctx, cancel := context.WithCancel(t.Context())
	cmd := s.CommandContext(ctx, "sleep", "60")
	assert.NilError(t, cmd.Start())
	cancel()

	assertKilled(t, cmd, cmd.Wait())
}

func TestStubsRegisterAnyTimes(t *testing.T) {
	t.Parallel()

	tb := &fakeTB{TB: t}
	s := NewStubs(tb)
	s.RegisterAnyTimes(Regexp(regexp.MustCompile(`^git `)), Result{Stdout: "any\n"})
	s.Register(Exact("git", "push"), Result{Stdout: "once\n"})
	s.RegisterAnyTimes(Exact("unused"), Result{})

	// A stub from Register answers first, also when it was registered
	// later. Then the stub from RegisterAnyTimes answers each command.
	const n = 3
	outs := make([]string, 0, n)
	for range n {
		out, err := s.CommandContext(t.Context(), "git", "push").Output()
		assert.NilError(t, err)
		outs = append(outs, string(out))
	}
	assert.DeepEqual(t, outs, []string{"once\n", "any\n", "any\n"})

	// A stub from RegisterAnyTimes that answers no command does not fail
	// the test.
	assert.Equal(t, len(tb.runCleanups()), 0)
}
