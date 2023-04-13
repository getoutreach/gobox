package trace_test

import (
	"context"
	"testing"
	"time"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
)

func startCall(opt call.Option) *call.Info {
	var callInfo *call.Info
	trace.StartCall(
		context.Background(),
		"test",
		call.Option(func(c *call.Info) {
			callInfo = c
		}),
		opt,
	)
	return callInfo
}

func TestWithOptions(t *testing.T) {
	scheduledAt := time.Now()
	callInfo := startCall(trace.WithScheduledTime(scheduledAt))
	assert.Equal(t, scheduledAt, callInfo.Times.Scheduled)
}

func TestAsGRPCCall(t *testing.T) {
	callInfo := startCall(trace.AsGRPCCall())
	assert.Equal(t, call.TypeGRPC, callInfo.Type)
}

func TestAsOutboundCall(t *testing.T) {
	callInfo := startCall(trace.AsOutboundCall())
	assert.Equal(t, call.TypeOutbound, callInfo.Type)
}

func TestWithInfoLoggingDisabled(t *testing.T) {
	// Fixes a test break in VSCode, where the app version is not set.
	if app.Info().Version == "" {
		app.Info().Version = "testing"
	}

	// Test that the default is false
	callInfo := startCall(func(c *call.Info) {})
	assert.Equal(t, false, callInfo.Opts.DisableInfoLogging)

	// Test that trace.WithInfoLoggingDisabled() sets the DisableInfoLogging
	// option to true.
	callInfo = startCall(trace.WithInfoLoggingDisabled())
	assert.Equal(t, true, callInfo.Opts.DisableInfoLogging)

	// Make a call and ensure that info logs are not emitted.
	recorder := logtest.NewLogRecorder(t)
	defer recorder.Close()

	ctx := trace.StartCall(context.Background(), "test", trace.WithInfoLoggingDisabled())
	trace.EndCall(ctx)

	if diff := cmp.Diff([]logf.F(nil), recorder.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}

	// now make a call with info logging enabled and ensure that info logs are
	// emitted.
	ctx = trace.StartCall(context.Background(), "test")
	trace.EndCall(ctx)

	expected := []log.F{
		{
			"@timestamp":          differs.AnyString(),
			"app.version":         differs.AnyString(),
			"event_name":          "trace",
			"honeycomb.parent_id": differs.AnyString(),
			"honeycomb.span_id":   differs.AnyString(),
			"honeycomb.trace_id":  differs.AnyString(),
			"level":               "INFO",
			"message":             "test",
			"module":              "github.com/getoutreach/gobox",
			"timing.dequeued_at":  differs.AnyString(),
			"timing.finished_at":  differs.AnyString(),
			"timing.scheduled_at": differs.AnyString(),
			"timing.service_time": differs.AnyFloat64(),
			"timing.total_time":   differs.AnyFloat64(),
			"timing.wait_time":    differs.AnyFloat64(),
		},
	}
	if diff := cmp.Diff(expected, recorder.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}
