// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: A fake of exec.Exec that matches commands against registered stubs.

package execstub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/getoutreach/gobox/pkg/exec"
)

// errUnexpectedCommand is returned by Wait for a command that no stub
// matched.
var errUnexpectedCommand = errors.New("execstub: unexpected command")

// Matcher selects the commands that a stub answers.
type Matcher interface {
	// Match reports whether the command name with args matches.
	Match(name string, args []string) bool
	// String describes the matcher in test failures.
	String() string
}

// Exact returns a Matcher for the command name with exactly args.
func Exact(name string, args ...string) Matcher {
	return exactMatcher{name: name, args: args}
}

// Regexp returns a Matcher for the commands whose command line matches re.
// The command line is name and args, joined by spaces.
func Regexp(re *regexp.Regexp) Matcher {
	return regexpMatcher{re: re}
}

// MatchFunc returns a Matcher for the commands for which fn returns true.
// desc describes the commands in test failures.
func MatchFunc(desc string, fn func(name string, args []string) bool) Matcher {
	return funcMatcher{desc: desc, fn: fn}
}

// Result is what a stub does for a command.
type Result struct {
	// Stdout is the standard output of the command.
	Stdout string
	// Stderr is the standard error of the command.
	Stderr string
	// ExitCode is the exit status. 0 succeeds. A code from 1 to 255 makes
	// Wait return an *exec.ExitError for that status. Any other code makes
	// Wait return the error from NewExitError.
	ExitCode int
	// Func, if not nil, does the work of the command, and the other fields
	// are ignored.
	Func RunFunc
}

// Stubs is a fake exec.Exec that answers each command with the first
// registered stub that matches it and was not used yet. Each stub answers one
// command. A command that no stub answers fails the test, and Wait returns
// an error for it. At the end of the test, each stub that answered no
// command fails the test. Stubs is safe for use by many goroutines.
type Stubs struct {
	// tb gets the test failures.
	tb testing.TB

	// mu guards stubs.
	mu sync.Mutex

	// stubs are the registered stubs, in order.
	stubs []*stub
}

// stub is a registered Matcher and its Result.
type stub struct {
	matcher Matcher
	result  Result
	used    bool
}

// NewStubs returns an empty Stubs that reports failures to tb.
func NewStubs(tb testing.TB) *Stubs {
	tb.Helper()

	s := &Stubs{tb: tb}
	tb.Cleanup(s.checkUsed)

	return s
}

// Register adds a stub that answers one command that m matches, with r.
func (s *Stubs) Register(m Matcher, r Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stubs = append(s.stubs, &stub{matcher: m, result: r})
}

// CommandContext returns a fake command for the first unused stub that
// matches name and args. If no stub matches, it fails the test with Errorf,
// and returns a command whose Wait returns an error.
func (s *Stubs) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	st := s.take(name, args)
	if st == nil {
		s.tb.Errorf("execstub: unexpected command: %s", commandLine(name, args))
		return newCmd(ctx, unexpected(name, args), name, args)
	}

	fn := st.result.Func
	if fn == nil {
		fn = canned(st.result.Stdout, st.result.Stderr, st.result.ExitCode)
	}

	return newCmd(ctx, fn, name, args)
}

// take marks the first unused stub that matches as used, and returns it. It
// returns nil if no stub matches.
func (s *Stubs) take(name string, args []string) *stub {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, st := range s.stubs {
		if !st.used && st.matcher.Match(name, args) {
			st.used = true
			return st
		}
	}

	return nil
}

// checkUsed fails the test for each stub that answered no command.
func (s *Stubs) checkUsed() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, st := range s.stubs {
		if !st.used {
			s.tb.Errorf("execstub: stub never used: %s", st.matcher)
		}
	}
}

// unexpected returns a RunFunc that fails with errUnexpectedCommand.
func unexpected(name string, args []string) RunFunc {
	return func(context.Context, *exec.Cmd, io.Reader, io.Writer, io.Writer) error {
		return fmt.Errorf("%w: %s", errUnexpectedCommand, commandLine(name, args))
	}
}

// commandLine returns name and args, joined by spaces.
func commandLine(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

// exactMatcher is the Matcher from Exact.
type exactMatcher struct {
	name string
	args []string
}

// Match reports whether name and args are equal to those of m.
func (m exactMatcher) Match(name string, args []string) bool {
	return name == m.name && slices.Equal(args, m.args)
}

// String returns the command line.
func (m exactMatcher) String() string {
	return commandLine(m.name, m.args)
}

// regexpMatcher is the Matcher from Regexp.
type regexpMatcher struct {
	re *regexp.Regexp
}

// Match reports whether the command line matches the regular expression.
func (m regexpMatcher) Match(name string, args []string) bool {
	return m.re.MatchString(commandLine(name, args))
}

// String returns the regular expression.
func (m regexpMatcher) String() string {
	return "regexp " + m.re.String()
}

// funcMatcher is the Matcher from MatchFunc.
type funcMatcher struct {
	desc string
	fn   func(name string, args []string) bool
}

// Match returns the result of the function.
func (m funcMatcher) Match(name string, args []string) bool {
	return m.fn(name, args)
}

// String returns the description.
func (m funcMatcher) String() string {
	return m.desc
}
