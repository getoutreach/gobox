//go:build !or_e2e

package log_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
)

type withSuite struct{}

func (withSuite) TestWith(t *testing.T) {
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	logger := log.With(log.F{"with": "hey"})
	ctx := context.Background()

	logger.Debug(ctx, "Debug message", log.F{"some": "thing"})
	logger.Info(ctx, "Info message", log.F{"some": "thing"})
	logger.Warn(ctx, "Warn message", log.F{"some": "thing"})
	logger.Error(ctx, "Warn message", log.F{"some": "thing"})

	expected := []log.F{
		{
			"@timestamp":  differs.RFC3339NanoTime(),
			"app.version": "testing",
			"level":       "INFO",
			"message":     "Info message",
			"some":        "thing",
			"source":      "getoutreach/gobox",
			"with":        "hey",
		},
		{
			"@timestamp":  differs.RFC3339NanoTime(),
			"app.version": "testing",
			"level":       "WARN",
			"message":     "Warn message",
			"some":        "thing",
			"source":      "getoutreach/gobox",
			"with":        "hey",
		},
		{
			"@timestamp":  differs.RFC3339NanoTime(),
			"app.version": "testing",
			"level":       "DEBUG",
			"message":     "Debug message",
			"some":        "thing",
			"source":      "getoutreach/gobox",
			"with":        "hey",
		},
		{
			"@timestamp":  differs.RFC3339NanoTime(),
			"app.version": "testing",
			"level":       "ERROR",
			"message":     "Warn message",
			"some":        "thing",
			"source":      "getoutreach/gobox",
			"with":        "hey",
		},
	}

	if diff := cmp.Diff(expected, logs.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected log entries", diff)
	}
}
