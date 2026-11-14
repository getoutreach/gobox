// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests that the test failures of Stubs point to the line of
// the test, not to the execstub package.

package execstub

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// failureChildEnv selects the case of TestFailureLocationChild.
const failureChildEnv = "EXECSTUB_FAILURE_CHILD"

// printWant prints the file and line after the line of its caller, which
// is where the child test expects its failure.
func printWant() {
	_, file, line, _ := runtime.Caller(1)
	fmt.Printf("want: %s:%d\n", filepath.Base(file), line+1)
}

// TestFailureLocationChild fails on purpose, in a child process of
// TestFailureLocation.
func TestFailureLocationChild(t *testing.T) {
	ctx := context.Background()
	switch os.Getenv(failureChildEnv) {
	case "":
		t.Skip("runs only as a child of TestFailureLocation")
	case "unexpected":
		s := NewStubs(t)
		printWant()
		s.CommandContext(ctx, "gh", "pr", "list")
	case "spy":
		spy := NewSpyExec(NewStubs(t))
		printWant()
		spy.CommandContext(ctx, "gh", "pr", "list")
	case "order":
		s := NewStubs(t, InOrder())
		s.Register(Exact("first"), Result{})
		s.Register(Exact("second"), Result{})
		printWant()
		s.CommandContext(ctx, "second")
		s.CommandContext(ctx, "first")
	case "unused":
		printWant()
		s := NewStubs(t)
		s.Register(Exact("gh"), Result{})
	}
}

// TestFailureLocation checks that each failure of Stubs, also through a
// SpyExec and at cleanup, reports the line of the test.
func TestFailureLocation(t *testing.T) {
	t.Parallel()

	want := regexp.MustCompile(`(?m)^want: (\S+)$`)
	for _, tc := range []string{"unexpected", "spy", "order", "unused"} {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			cmd := osexec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestFailureLocationChild$")
			cmd.Env = append(os.Environ(), failureChildEnv+"="+tc)
			out, err := cmd.CombinedOutput()
			assert.Assert(t, err != nil, "the child test must fail:\n%s", out)

			m := want.FindSubmatch(out)
			assert.Assert(t, m != nil, "no want line:\n%s", out)
			assert.Assert(t, strings.Contains(string(out), " "+string(m[1])+": execstub: "),
				"the failure is not at %s:\n%s", m[1], out)
		})
	}
}
