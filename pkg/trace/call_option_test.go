package trace_test

import (
	"context"
	"testing"
	"time"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

// ignoreVariableFields ignores the "deployment.namespace" and
// "app.version" fields because they can be variable across testing
// environments.
func ignoreVariableFields() cmp.Option {
	return cmpopts.IgnoreMapEntries(func(key string, value interface{}) bool {
		return key == "deployment.namespace" || key == "app.version"
	})
}

func TestWithInfoLoggingManuallyEnabled(t *testing.T) {
	// Test that the default is false
	callInfo := startCall(func(c *call.Info) {})
	assert.Equal(t, false, callInfo.Opts.EnableInfoLogging)

	// Test that trace.WithInfoLoggingEnabled() sets the EnableInfoLogging
	// option to true.
	callInfo = startCall(trace.WithInfoLoggingEnabled())
	assert.Equal(t, true, callInfo.Opts.EnableInfoLogging)

	// Make a call and ensure that info logs are not emitted.
	recorder := logtest.NewLogRecorder(t)
	defer recorder.Close()

	ctx := trace.StartCall(context.Background(), "test")
	trace.EndCall(ctx)

	if diff := cmp.Diff([]logf.F(nil), recorder.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}

	// now make a call with info logging enabled and ensure that info logs are
	// emitted.
	ctx = trace.StartCall(context.Background(), "test", trace.WithInfoLoggingEnabled())
	trace.EndCall(ctx)

	expected := []log.F{
		{
			"@timestamp":          differs.AnyString(),
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
	if diff := cmp.Diff(expected, recorder.Entries(), differs.Custom(), ignoreVariableFields()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func TestWithInfoLoggingManuallyDisabled(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorderWithOptions(tracetest.Options{
		SamplePercent:     100.0,
		LogCallsByDefault: true,
	})
	defer spanRecorder.Close()

	// Test that the default is true.
	callInfo := startCall(func(c *call.Info) {})
	assert.Equal(t, true, callInfo.Opts.EnableInfoLogging)

	// Test that trace.WithInfoLoggingDisabled() sets the EnableInfoLogging
	// option to false.
	callInfo = startCall(trace.WithInfoLoggingDisabled())
	assert.Equal(t, false, callInfo.Opts.EnableInfoLogging)

	// Make a call and ensure that info logs are not emitted.
	logRecorder := logtest.NewLogRecorder(t)
	defer logRecorder.Close()

	ctx := trace.StartCall(context.Background(), "test", trace.WithInfoLoggingDisabled())
	trace.EndCall(ctx)

	if diff := cmp.Diff([]logf.F(nil), logRecorder.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}

	// Make a call with info and ensure that info logs are emitted.
	ctx = trace.StartCall(context.Background(), "test")
	trace.EndCall(ctx)

	expected := []log.F{
		{
			"@timestamp":          differs.AnyString(),
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
	if diff := cmp.Diff(expected, logRecorder.Entries(), differs.Custom(), ignoreVariableFields()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func TestWithNoCallInfo(t *testing.T) {
	ctx := context.Background()

	recorder := logtest.NewLogRecorder(t)
	defer recorder.Close()

	// This shouldn't panic or produce any logs.
	trace.EndCall(ctx)

	expected := []log.F(nil)
	if diff := cmp.Diff(expected, recorder.Entries(), differs.Custom(), ignoreVariableFields()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}
