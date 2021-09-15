// Package tester implements a test runner compatible with testing.T
//
// Usage:
//
//     t := tester.New()
//     tester.Run(t, "testA", func(t *tester.T) { ... })
//     results := tester.Results(t)
//
// This package implements a subset of the testing.T type. In
// particular, it implements the testing.TB interface and also
// provides helpers to run subtests using tester.Run.
//
// The package is intended to help build test suites that can not only
// run with `go test` but can also be run to validate things in
// production code.  The intention is to be able to write production
// validation code (which is meant to be compiled into command line
// tools or service binaries) in much the same way as regular tests.
//
// Individual tests can use t.Error, t.Errorf etc (as well as
// gotest.tools/v3/assert functions, for instance, or the
// stretchr/testify set of helpers).
//
// Unfortunately, indvidual tests written with the standard signature
// of `TestXYZ(t *testing.T)` cannot be used with tester.New() as
// the testing.T type is concrete and tester.New() does not return a
// type compatible with this..  Instead, the suggested approach
// is for test suites to be written against tester.T like so:
//
//      func TestXYZ(t *tester.T) { .... }
//
// And then, a TestAll function can be written that will work properly
// with testing.T:
//
//      func TestAll(t *testing.T) {
//          tester.Run("TestXYZ", TestXYZ)
//          ...
//      }
//
// When tester.Run is used in a `go test` type situation, it simply
// maps it to t.Run. But when it is used with a `tester.New()`
// instance, it correctly creates a sub task on that type.
//
// This allows tests written in the style of testing.T (but using the
// subset of methods in tester.T) to be executed either in a test
// environment (using testing.T) or in a production environment (using
// tester.New()).
//
// The main export of this package is the New() function.
//
// You can log test results using LogResults.
package tester

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
)

// Run is a polymorphic function where t is either *testing.T or tester.T.
// f is of type func(t *testing.T) or func(t tester.T) or some subset
// of tester.T.
//
// This is convenience function so that test code written with this
// will work with both `go test` and production scenarios (which use
// tester.T).
func Run(t T, name string, f interface{}) bool {
	args := []reflect.Value{reflect.ValueOf(name), reflect.ValueOf(f)}
	results := reflect.ValueOf(t).MethodByName("Run").Call(args)
	return results[0].Bool()
}

// RunTest executes a single test.
func RunTest(t T, f func(testing.TB)) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer LogResults(t)
		defer wg.Done()
		f(t)
	}()
	wg.Wait()
}

// TestFailure contains information about the test that failed.
type TestFailure struct {
	TestName, Failure string
}

// TestResults contains the test results.
type TestResults struct {
	Failed   bool
	Failures []*TestFailure
}

// Results returns the results of the tests so far.
func Results(t T) *TestResults {
	if t, ok := t.(*tester); ok {
		return &TestResults{Failed: t.Failed(), Failures: t.failures}
	}
	return &TestResults{Failed: t.Failed()}
}

// LogResults logs the results of the tests using log.Error.
func LogResults(t T) {
	tr := Results(t)
	ctx := context.Background()
	if !tr.Failed {
		log.Info(ctx, "All tests pass", nil)
	} else {
		for _, f := range tr.Failures {
			log.Error(ctx, "Test "+f.TestName+" has failed: "+f.Failure, nil)
		}
	}
}

// T is the interface implemented by this package.  It is a superset
// of testing.TB and a subset of *testing.T.
type T interface {
	testing.TB
	Parallel()
}

// Option defines a test option.
type Option func(t *tester)

// WithLogWriter configures the logging function.  The default is to
// use log.Info.
func WithLogWriter(logfunc func(name, message string)) Option {
	return func(t *tester) {
		t.log = logfunc
	}
}

// WithName configures the test name.  The default name is "test."
func WithName(name string) Option {
	return func(t *tester) {
		t.name = name
	}
}

// New returns an implementation of a T (which includes testing.TB).
//
// Individual tests can be run using the Run and the final results can
// be obtained using Results.
//
// You can customize the root test name using WithName.  The default
// is "test".
//
// You can customize how intermediate test information is logged.  The
// default is log.Info (use the LogResults function to summarize log
// results into meaningful log.Error statements).
func New(opts ...Option) T {
	t := &tester{
		completed: make(chan struct{}),
		name:      "test",
		log: func(testName, message string) {
			ctx := context.Background()
			log.Info(ctx, message, nil)
		},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

type tester struct {
	// testing.T is embedded as testing.TB has private methods and
	// this is the only way to get them.  Those methods are never
	// actually used, so it doesn't matter.
	testing.T

	parent *tester
	name   string
	log    func(name, message string)

	// completed is closed (by tester.Run) when a test is done.
	completed chan struct{}

	// cleanups and failures need to be protected with a lock
	mu       sync.Mutex
	skipped  bool
	cleanups []func()
	failures []*TestFailure
}

// Run is roughly the same as testing.T.Run.
func (t *tester) Run(name string, f interface{}) (result bool) {
	name = t.name + "/" + name
	inner := &tester{completed: make(chan struct{}), name: name, log: t.log, parent: t}
	go func() {
		defer inner.complete()
		reflect.ValueOf(f).Call([]reflect.Value{reflect.ValueOf(inner)})
	}()
	<-inner.completed
	return !inner.Failed()
}

// Cleanup implements testing.TB.Cleanup.
func (t *tester) Cleanup(cleanup func()) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanups = append(t.cleanups, cleanup)
}

// Log implements testing.TB.Log.
func (t *tester) Log(args ...interface{}) {
	t.log(t.name, fmt.Sprint(args...))
}

// Logf implements testing.TB.Logf.
func (t *tester) Logf(format string, args ...interface{}) {
	t.log(t.name, fmt.Sprintf(format, args...))
}

// Error implements testing.TB.Error.
func (t *tester) Error(args ...interface{}) {
	t.Log(args...)
	t.Fail()
}

// Errorf implements testing.TB.Errorf.
func (t *tester) Errorf(format string, args ...interface{}) {
	t.Logf(format, args...)
	t.Fail()
}

// Fail implements testing.TB.Fail.
func (t *tester) Fail() {
	t.addFailure(t.name, "")
	select {
	case <-t.completed:
		panic("Failed in goroutine after " + t.name + " has completed")
	default:
	}
}

// FailNow implements testing.TB.Fail.
func (t *tester) FailNow() {
	t.Fail()
	runtime.Goexit()
}

// Fatal implements testing.TB.Fatal.
func (t *tester) Fatal(args ...interface{}) {
	t.Log(args...)
	t.FailNow()
}

// Fatalf implements testing.TB.Fatalf.
func (t *tester) Fatalf(format string, args ...interface{}) {
	t.Logf(format, args...)
	t.FailNow()
}

// Skip implements testing.TB.Skip.
func (t *tester) Skip(args ...interface{}) {
	t.Log(args...)
	t.SkipNow()
}

// Skipf implementse testing.TB.Skipf.
func (t *tester) Skipf(format string, args ...interface{}) {
	t.Logf(format, args...)
	t.SkipNow()
}

// SkipNow implements testing.TB.SkipNow.
func (t *tester) SkipNow() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skipped = true
	runtime.Goexit()
}

// Failed implements testing.TB.Failed.
func (t *tester) Failed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.failures) > 0
}

// Skipped implementse testing.TB.Skipped.
func (t *tester) Skipped() bool {
	t.mu.Lock()
	defer t.mu.Unlock() //nolint: unnecessaryDefer
	return t.skipped
}

// TempDir implements testing.TB.TempDir.
func (t *tester) TempDir() string {
	// see https://golang.org/src/testing/testing.go?s=28272:28314#L905
	pattern := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(t.Name())
	dir, err := ioutil.TempDir("", pattern)
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("TempDir RemoveAll cleanup: %v", err)
		}
	})
	return dir
}

// Name implements testing.TB.Name.
func (t *tester) Name() string {
	return t.name
}

// Parallel is effectively ignored.
func (t *tester) Parallel() {
	// NYI
}

// Helper is not yet implemented.
func (t *tester) Helper() {
	// NYI
}

func (t *tester) complete() {
	if r := recover(); r != nil {
		failure := fmt.Sprintf("panic %v", r)
		t.Log(failure)
		t.addFailure(t.name, failure)
	}

	t.mu.Lock()
	cleanups := t.cleanups
	t.cleanups = nil
	t.mu.Unlock()

	defer close(t.completed)
	for _, cleanup := range cleanups {
		cleanup := cleanup
		defer cleanup()
	}
}

func (t *tester) addFailure(name, failure string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures = append(t.failures, &TestFailure{name, failure})
	if t.parent != nil {
		t.parent.addFailure(name, failure)
	}
}
