package log_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
)

type logContextSuite struct{}

func (logContextSuite) String() string {
	return "logContextSuite"
}

func (logContextSuite) TestLogContext(t *testing.T) {
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()
	ctx = log.NewLogContext(ctx)
	log.SetAllowedLogContextFields("context.string", "context.number")
	log.AddInfo(ctx, log.F{"context.string": "test", "context.number": 5, "foo": "not_allowed"})
	log.Debug(ctx, "Debug message", log.F{"some": "thing"})
	log.Info(ctx, "Info message", log.F{"some": "thing"})
	log.Warn(ctx, "Warn message", log.F{"some": "thing"})
	log.Error(ctx, "Warn message", log.F{"some": "thing"})

	expected := []log.F{
		{
			"@timestamp":     differs.RFC3339NanoTime(),
			"app.version":    differs.AnyString(),
			"context.string": "test",
			"context.number": float64(5),
			"level":          "INFO",
			"message":        "Info message",
			"some":           "thing",
		},
		{
			"@timestamp":     differs.RFC3339NanoTime(),
			"app.version":    differs.AnyString(),
			"context.string": "test",
			"context.number": float64(5),
			"level":          "WARN",
			"message":        "Warn message",
			"some":           "thing",
		},
		{
			"@timestamp":     differs.RFC3339NanoTime(),
			"app.version":    differs.AnyString(),
			"context.string": "test",
			"context.number": float64(5),
			"level":          "DEBUG",
			"message":        "Debug message",
			"some":           "thing",
		},
		{
			"@timestamp":     differs.RFC3339NanoTime(),
			"app.version":    differs.AnyString(),
			"context.string": "test",
			"context.number": float64(5),
			"level":          "ERROR",
			"message":        "Warn message",
			"some":           "thing",
		},
	}

	if diff := cmp.Diff(expected, logs.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected log entries", diff)
	}
}
