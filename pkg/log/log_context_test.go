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

type orgInfo struct {
	guid      string
	shortname string
	bento     string
}

func (o *orgInfo) MarshalLog(addField func(key string, v interface{})) {
	addField("guid", o.guid)
	addField("shortname", o.shortname)
	addField("bento", o.bento)
}

func (logContextSuite) TestLogContext(t *testing.T) {
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()
	ctx = log.NewContext(ctx,
		log.F{"context.string": "test",
			"context": log.F{
				"number": 5,
				"bar":    "not_allowed"},
			"foo": "not_allowed",
		},
		log.F{"or.org": &orgInfo{
			"bab20e22-834c-466c-a1df-90873b8b22a6",
			"short",
			"bento",
		},
		})

	log.Debug(ctx, "Debug message", log.F{"some": "thing"})
	log.Info(ctx, "Info message", log.F{"some": "thing"})
	log.Warn(ctx, "Warn message", log.F{"some": "thing"})
	log.Error(ctx, "Warn message", log.F{"some": "thing"})

	expected := []log.F{
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "INFO",
			"message":          "Info message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "WARN",
			"message":          "Warn message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "DEBUG",
			"message":          "Debug message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "ERROR",
			"message":          "Warn message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
	}

	if diff := cmp.Diff(expected, logs.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected log entries", diff)
	}
}

func (logContextSuite) TestNestedLogContext(t *testing.T) {
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()
	ctx = log.NewContext(ctx,
		log.F{"context.string": "test",
			"context": log.F{
				"number": 5,
				"bar":    "not_allowed"},
			"foo": "not_allowed",
		})

	ctx = log.NewContext(ctx,
		log.F{"or.org": &orgInfo{
			"bab20e22-834c-466c-a1df-90873b8b22a6",
			"short",
			"bento",
		},
		})

	log.Debug(ctx, "Debug message", log.F{"some": "thing"})
	log.Info(ctx, "Info message", log.F{"some": "thing"})
	log.Warn(ctx, "Warn message", log.F{"some": "thing"})
	log.Error(ctx, "Warn message", log.F{"some": "thing"})

	expected := []log.F{
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "INFO",
			"message":          "Info message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "WARN",
			"message":          "Warn message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "DEBUG",
			"message":          "Debug message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
		{
			"@timestamp":       differs.RFC3339NanoTime(),
			"app.version":      differs.AnyString(),
			"context.string":   "test",
			"context.number":   float64(5),
			"level":            "ERROR",
			"message":          "Warn message",
			"or.org.guid":      "bab20e22-834c-466c-a1df-90873b8b22a6",
			"or.org.shortname": "short",
			"some":             "thing",
		},
	}

	if diff := cmp.Diff(expected, logs.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected log entries", diff)
	}
}
