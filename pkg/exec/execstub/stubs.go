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

// AnyArg in the args of [Exact] matches any one argument at its position,
// such as the path of a temporary file. It is not a valid argument of a
// real command, because it holds a NUL byte. It has no effect on the name.
const AnyArg = "\x00execstub.AnyArg\x00"

// Exact returns a Matcher for the command name with exactly args. An
// [AnyArg] in args matches any one argument.
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
	// Func, if not nil, runs first, for example to read the input or to
	// write a file in cmd.Dir. If Func returns an error, Wait returns it,
	// and the stub writes no Stdout and Stderr. Otherwise the stub then
	// writes Stdout and Stderr and exits with ExitCode, as if Func was not
	// set. After a kill, Wait returns the error for "signal: killed".
	Func RunFunc
	// Real, if true, runs the command as a real process, as
	// [exec.NewRealExec] does, and the other fields are ignored. The
	// command keeps all of its fields, such as Dir, Env, Stdin, Stdout and
	// Stderr, and its context. [Signals] returns nil for it. Like other
	// stubs, the stub answers one command, or any number of commands if it
	// is from [Stubs.RegisterAnyTimes].
	Real bool
}

// Option changes the behavior of a Stubs. Give it to [NewStubs].
type Option func(*Stubs)

// InOrder returns an Option that makes the stubs answer commands in the
// order that they were registered. A command that matches a stub while an
// earlier stub that answers one command is not used yet fails the test.
// The stub answers the command all the same. The stubs from
// [Stubs.RegisterAnyTimes] are not in the order.
func InOrder() Option {
	return func(s *Stubs) { s.inOrder = true }
}

// Stubs is a fake exec.Exec that answers each command with the first
// registered stub that matches it and was not used yet. Each stub from
// [Stubs.Register] answers one command. If none matches, the first stub
// from [Stubs.RegisterAnyTimes] that matches answers. A command that no
// stub answers fails the test, and Wait returns an error for it. At the end
// of the test, each stub from Register that answered no command fails the
// test. By default, the stubs answer commands in any order. Use [InOrder]
// to check the order. Stubs is safe for use by many goroutines.
type Stubs struct {
	// tb gets the test failures.
	tb testing.TB

	// inOrder is true if the stubs must answer in the order of Register.
	inOrder bool

	// mu guards stubs.
	mu sync.Mutex

	// stubs are the registered stubs, in order.
	stubs []*stub
}

// stub is a registered Matcher and its Result.
type stub struct {
	matcher Matcher
	result  Result
	// anyTimes is true for a stub from RegisterAnyTimes.
	anyTimes bool
	used     bool
}

// NewStubs returns an empty Stubs that reports failures to tb.
func NewStubs(tb testing.TB, opts ...Option) *Stubs {
	tb.Helper()

	s := &Stubs{tb: tb}
	for _, opt := range opts {
		opt(s)
	}
	tb.Cleanup(s.checkUsed)

	return s
}

// Register adds a stub that answers one command that m matches, with r.
func (s *Stubs) Register(m Matcher, r Result) {
	s.add(&stub{matcher: m, result: r})
}

// RegisterAnyTimes adds a stub that answers each command that m matches
// and that no stub from Register answers, with r. It can answer any number
// of commands, also none, and it is not in the order of [InOrder].
func (s *Stubs) RegisterAnyTimes(m Matcher, r Result) {
	s.add(&stub{matcher: m, result: r, anyTimes: true})
}

// CommandContext returns a command for the stub that answers name and args.
// If no stub answers, it fails the test with Errorf, and returns a command
// whose Wait returns an error.
func (s *Stubs) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	s.tb.Helper()

	st := s.take(name, args)
	switch {
	case st == nil:
		s.tb.Errorf("execstub: unexpected command: %s", commandLine(name, args))
		return newCmd(ctx, unexpected(name, args), name, args)
	case st.result.Real:
		return exec.NewRealExec().CommandContext(ctx, name, args...)
	default:
		return newCmd(ctx, st.result.runFunc(), name, args)
	}
}

// add appends st to the stubs.
func (s *Stubs) add(st *stub) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stubs = append(s.stubs, st)
}

// take returns the stub that answers name and args, or nil if no stub
// does. It marks a stub from Register as used, and checks the order for
// InOrder.
func (s *Stubs) take(name string, args []string) *stub {
	s.tb.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	var firstUnused, anyTimes *stub
	for _, st := range s.stubs {
		switch {
		case st.anyTimes:
			if anyTimes == nil && st.matcher.Match(name, args) {
				anyTimes = st
			}
		case st.used:
			// A used stub from Register answers no more commands.
		case st.matcher.Match(name, args):
			if s.inOrder && firstUnused != nil {
				s.tb.Errorf("execstub: command out of order: %s ran before %s", commandLine(name, args), firstUnused.matcher)
			}
			st.used = true
			return st
		case firstUnused == nil:
			firstUnused = st
		}
	}

	return anyTimes
}

// checkUsed fails the test for each stub from Register that answered no
// command.
func (s *Stubs) checkUsed() {
	// As a helper in a cleanup function, the failure points to the line
	// that called NewStubs.
	s.tb.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, st := range s.stubs {
		if !st.anyTimes && !st.used {
			s.tb.Errorf("execstub: stub never used: %s", st.matcher)
		}
	}
}

// runFunc returns the RunFunc of a fake command for r: Func, if set, and
// then the canned output and exit code.
func (r Result) runFunc() RunFunc {
	static := canned(r.Stdout, r.Stderr, r.ExitCode)
	if r.Func == nil {
		return static
	}

	return func(ctx context.Context, cmd *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) error {
		if err := r.Func(ctx, cmd, stdin, stdout, stderr); err != nil || ctx.Err() != nil {
			return err
		}

		return static(ctx, cmd, stdin, stdout, stderr)
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

// Match reports whether name and args are equal to those of m. An AnyArg
// in m matches any argument.
func (m exactMatcher) Match(name string, args []string) bool {
	return name == m.name && slices.EqualFunc(m.args, args, func(want, got string) bool {
		return want == AnyArg || want == got
	})
}

// String returns the command line, with <any> for each AnyArg.
func (m exactMatcher) String() string {
	args := make([]string, len(m.args))
	for i, arg := range m.args {
		if arg == AnyArg {
			arg = "<any>"
		}
		args[i] = arg
	}

	return commandLine(m.name, args)
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
