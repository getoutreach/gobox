//go:build !or_e2e

package log_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/google/go-cmp/cmp"
)

type fatalSuite struct{}

func (fatalSuite) TestFatal(t *testing.T) {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	// we can't actually call log.Fatal as it will call os.Exit.
	// Instead, we call regular log.Error but override "level" to
	// "FATAL".  This goes through all the motions of log.Fatal
	// except for os.Exit(1)
	log.Error(context.Background(), "example", log.F{"level": "FATAL"})

	got := logs.Entries()
	want := []log.F{{
		"@timestamp":    differs.RFC3339NanoTime(),
		"app.version":   differs.AnyString(),
		"error.kind":    "fatal",
		"error.error":   "fatal occurred",
		"error.message": "fatal occurred",
		//nolint:lll // Why: Output comparision
		"error.stack": differs.StackLike("goroutine\nruntime/debug.Stack\ndebug/stack.go\nlog.generateFatalFields\nlog/log.go\nlog.format\nlog/log.go\nlog.Error\nlog/log.go\nlog_test.fatalSuite.TestFatal"),
		"level":       "FATAL",
		"message":     "example",
		"source":      "github.com/getoutreach/gobox",
	}}

	if diff := cmp.Diff(want, got, differs.Custom()); diff != "" {
		t.Error("custom error mismatched", diff)
	}
}

func (fatalSuite) TestFatalWithError(t *testing.T) {
	logs := logtest.NewLogRecorder(nil)
	defer logs.Close()

	// we can't actually call log.Fatal as it will call os.Exit.
	// Instead, we call regular log.Error but override "level" to
	// "FATAL".  This goes through all the motions of log.Fatal
	// except for os.Exit(1)
	err := errors.New("my error")
	log.Error(context.Background(), "example", log.F{"level": "FATAL"}, events.NewErrorInfo(err))

	got := logs.Entries()
	want := []log.F{{
		"@timestamp":    differs.RFC3339NanoTime(),
		"app.version":   differs.AnyString(),
		"error.kind":    "fatal",
		"error.error":   "fatal occurred: my error",
		"error.message": "fatal occurred",
		//nolint:lll // Why: Output comparision
		"error.stack":         differs.StackLike("goroutine\nruntime/debug.Stack\ndebug/stack.go\nlog.generateFatalFields\nlog/log.go\nlog.format\nlog/log.go\nlog.Error\nlog/log.go\nlog_test.fatalSuite.TestFatal"),
		"error.cause.error":   "my error",
		"error.cause.kind":    "error",
		"error.cause.message": "my error",
		"message":             "example",
		"level":               "FATAL",
		"source":              "github.com/getoutreach/gobox",
	}}

	if diff := cmp.Diff(want, got, differs.Custom()); diff != "" {
		t.Error("custom error mismatched", diff)
	}
}
