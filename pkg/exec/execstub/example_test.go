// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Examples that inject a fake Exec into code under test.

package execstub_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/getoutreach/gobox/pkg/exec"
	"github.com/getoutreach/gobox/pkg/exec/execstub"
)

// latestTag returns the latest tag of the Git repository in dir. It takes an
// exec.Exec, so that a test can fake git.
func latestTag(ctx context.Context, ex exec.Exec, dir string) (string, error) {
	cmd := ex.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git describe: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Production code passes exec.NewRealExec() to latestTag. A test passes a
// fake, and can check the command that latestTag ran.
func ExampleNewFuncExec() {
	ex := execstub.NewFuncExec(func(cmd *exec.Cmd, name string, args []string) ([]byte, error) {
		fmt.Printf("ran %s %s in %s\n", name, strings.Join(args, " "), cmd.Dir)
		return []byte("v1.2.3\n"), nil
	})

	tag, err := latestTag(context.Background(), ex, "/src/repo")
	fmt.Println(tag, err)

	// Output:
	// ran git describe --tags --abbrev=0 in /src/repo
	// v1.2.3 <nil>
}

// A failing stub gives the same *exec.ExitError as a real command.
func ExampleNewStubbedCmdWithCode() {
	ex := execstub.NewStubbedCmdWithCode("", "fatal: No names found\n", 128)

	_, err := latestTag(context.Background(), ex, "/src/repo")

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		fmt.Println(exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
	}

	// Output:
	// 128 fatal: No names found
}

// exampleT is the testing.TB of the examples. It prints each failure, and
// runs the cleanups in finish. In a test, use the *testing.T of the test.
type exampleT struct {
	testing.TB

	cleanups []func()
}

// Helper does nothing.
func (*exampleT) Helper() {}

// Errorf prints the failure.
func (*exampleT) Errorf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// Cleanup records fn, for finish.
func (e *exampleT) Cleanup(fn func()) {
	e.cleanups = append(e.cleanups, fn)
}

// finish runs the cleanups in reverse order, as testing does at the end of
// a test.
func (e *exampleT) finish() {
	for _, fn := range slices.Backward(e.cleanups) {
		fn()
	}
}

// Stubs answers each command with a registered stub. Here, the commands
// must run in the order of the stubs.
func ExampleStubs() {
	t := &exampleT{}
	dir, err := os.MkdirTemp("", "example")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(dir)

	s := execstub.NewStubs(t, execstub.InOrder())
	// AnyArg matches the output file, whatever its path is. Func writes the
	// file, and then the stub writes Stdout.
	s.Register(execstub.Exact("tool", "build", "--out", execstub.AnyArg), execstub.Result{
		Func: func(_ context.Context, cmd *exec.Cmd, _ io.Reader, _, _ io.Writer) error {
			return os.WriteFile(cmd.Args[3], []byte("artifact"), 0o600)
		},
		Stdout: "built\n",
	})
	s.Register(execstub.Exact("tool", "upload"), execstub.Result{Stderr: "denied\n", ExitCode: 1})
	// echo runs as a real process, any number of times.
	s.RegisterAnyTimes(execstub.Exact("echo", execstub.AnyArg), execstub.Result{Real: true})

	ctx := context.Background()
	artifact := filepath.Join(dir, "artifact")
	out, err := s.CommandContext(ctx, "tool", "build", "--out", artifact).Output()
	fmt.Printf("%q %v\n", out, err)
	b, err := os.ReadFile(artifact)
	fmt.Printf("%q %v\n", b, err)
	out, err = s.CommandContext(ctx, "echo", "real").Output()
	fmt.Printf("%q %v\n", out, err)
	_, err = s.CommandContext(ctx, "tool", "upload").Output()
	fmt.Println(err)
	t.finish()

	// Output:
	// "built\n" <nil>
	// "artifact" <nil>
	// "real\n" <nil>
	// exit status 1
}

// With InOrder, a command that runs before the command of an earlier stub
// fails the test. The failure names both commands.
func ExampleInOrder() {
	t := &exampleT{}
	s := execstub.NewStubs(t, execstub.InOrder())
	s.Register(execstub.Exact("git", "fetch"), execstub.Result{})
	s.Register(execstub.Exact("git", "push"), execstub.Result{})

	ctx := context.Background()
	_ = s.CommandContext(ctx, "git", "push").Run()
	_ = s.CommandContext(ctx, "git", "fetch").Run()
	t.finish()

	// Output:
	// execstub: command out of order: git push ran before git fetch
}
