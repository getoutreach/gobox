package tracer_test

import (
	"context"
	"strings"
	"testing"

	"github.com/honeycombio/beeline-go"
	"gotest.tools/v3/assert"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/internal/tracer"
	"github.com/getoutreach/gobox/pkg/cleanup"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
)

const (
	forceTraceTest = "force-rate-test"
	sampleRateTest = "sample-rate-test"
)

// TestTracer_core is a single relatively comprehensive test that covers a lot of
// core use cases.
//
// 1. It creates nested spans and validates that the parent/child relationships
//    are correct.
//
// 2. It creates an async span and validates this span can terminate after the parent.
//
// 3. It creates a new trace from the async span. This validates the headers as well
//    as starting a new trace from headers.
//
// 4. It calls AddSpanInfo and AddTraceInfo to validate the output has this info.
//
// 5. The trace log itself is built using SetPresendHook, so this is also tested.
//
// 6. This test also tests ForceTrace and custom sample rate APIs as variations.
func TestTracer_core(t *testing.T) {
	for _, test := range []string{forceTraceTest, sampleRateTest, "default"} {
		t.Run(test, func(t *testing.T) {
			tx := tracer.New(tracer.WithCallTracking(), tracer.WithHoneycomb())
			recorder := newRecorder(t, tx, 0.0)
			defer recorder.close()

			ctx := context.Background()
			root := tx.StartTrace(ctx, "new trace", nil)

			switch test {
			case forceTraceTest:
				tx.ForceTrace(root)
			case sampleRateTest:
				tx.SetCurrentSampleRate(root, 1)
			}

			// setup spans.
			traceID := tx.Info(root).TraceID
			rootSpanID := tx.Info(root).SpanID

			outer := tx.StartSpan(root, "outer span", tracer.SpanSync, logf.F{"outer span": "foo"})
			outerSpanID := tx.Info(outer).SpanID
			assert.Equal(t, rootSpanID, tx.Info(outer).ParentID)
			assert.Equal(t, traceID, tx.Info(outer).TraceID)

			inner := tx.StartSpan(outer, "inner span", tracer.SpanSync, logf.F{"inner span": "foo"})
			innerSpanID := tx.Info(inner).SpanID
			assert.Equal(t, outerSpanID, tx.Info(inner).ParentID)
			assert.Equal(t, traceID, tx.Info(inner).TraceID)

			async := tx.StartSpan(inner, "async span", tracer.SpanAsync, logf.F{"async span": "foo"})
			asyncSpanID := tx.Info(async).SpanID
			assert.Equal(t, innerSpanID, tx.Info(async).ParentID)
			assert.Equal(t, traceID, tx.Info(async).TraceID)
			asyncHeaders := tx.Headers(async)

			tx.AddSpanInfo(inner, tracer.SpanSync, log.F{"inner span": "override"})
			tx.EndSpan(inner, tracer.SpanSync)

			tx.AddSpanInfo(outer, tracer.SpanSync, log.F{"outer span": "override"})
			tx.EndSpan(outer, tracer.SpanSync)

			tx.AddTraceInfo(root, logf.F{"root span": "foo"})
			tx.EndTrace(root)

			// Create a new trace based on the async span.
			newTrace := tx.StartTrace(ctx, "async trace", asyncHeaders)

			// SampleRate doesn't automatically propagate.
			if test == sampleRateTest {
				tx.SetCurrentSampleRate(newTrace, 1)
			}

			asyncSpanRootID := tx.Info(newTrace).SpanID
			assert.Equal(t, asyncSpanID, tx.Info(newTrace).ParentID)
			assert.Equal(t, traceID, tx.Info(newTrace).TraceID)
			tx.EndTrace(newTrace)

			events := recorder.flush(ctx)
			expected := []logf.F{
				{
					"inner span":      "override",
					"meta.span_type":  "mid",
					"name":            "inner span",
					"trace.parent_id": outerSpanID,
					"trace.span_id":   innerSpanID,
					"trace.trace_id":  strings.TrimPrefix(traceID, "hctrace_"),
				},
				{
					"meta.span_type":  "leaf",
					"name":            "outer span",
					"outer span":      "override",
					"trace.parent_id": rootSpanID,
					"trace.span_id":   outerSpanID,
					"trace.trace_id":  strings.TrimPrefix(traceID, "hctrace_"),
				},
				{
					"meta.span_type": "root",
					"name":           "new trace",
					"root span":      "foo",
					"trace.span_id":  rootSpanID,
					"trace.trace_id": strings.TrimPrefix(traceID, "hctrace_"),
				},
				// The async span comes in the very end!
				{
					"meta.span_type":  "subroot", // subroot because it is a new sub trace
					"name":            "async trace",
					"trace.span_id":   asyncSpanRootID,
					"trace.parent_id": asyncSpanID,
					"trace.trace_id":  strings.TrimPrefix(traceID, "hctrace_"),
				},
			}

			expected = extendEventsWithForceTraceAndSampleRate(expected, test)

			for _, each := range expected {
				each["app.version"] = differs.AnyString()
				each["duration_ms"] = differs.FloatRange(0, 5)
				each["meta.beeline_version"] = differs.AnyString()
				each["meta.local_hostname"] = differs.AnyString()
				each["service_name"] = "tracer-test"
			}

			assert.DeepEqual(t, expected, events, differs.Custom())
		})
	}
}

// TestTracer_calls tests the call tracking scenarios.
//
// It validates that all call types correctly expose the call
// information in honeycomb
func TestTracer_calls(t *testing.T) {
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	callSpans := map[tracer.SpanType]string{
		tracer.SpanInHTTP: "http",
		tracer.SpanInGRPC: "grpc",
		tracer.SpanOut:    "out",
		tracer.SpanCall:   "call",
	}

	for spanType, testName := range callSpans {
		t.Run(testName, func(t *testing.T) {
			tx := tracer.New(tracer.WithCallTracking(), tracer.WithHoneycomb())
			recorder := newRecorder(t, tx, 100.0)
			defer recorder.close()

			ctx := context.Background()
			root := tx.StartTrace(ctx, "new trace", nil)

			traceID := tx.Info(root).TraceID
			rootSpanID := tx.Info(root).SpanID

			call := tx.StartSpan(root, "call span", spanType, logf.F{"call span": "foo"})

			callSpanID := tx.Info(call).SpanID
			assert.Equal(t, rootSpanID, tx.Info(call).ParentID)
			assert.Equal(t, traceID, tx.Info(call).TraceID)

			tx.AddSpanInfo(call, spanType, log.F{"extra info": "foo"})
			tx.EndSpan(call, spanType)
			tx.EndTrace(root)

			events := recorder.flush(ctx)
			expected := []logf.F{
				{
					"meta.span_type":  "leaf",
					"name":            "call span",
					"call span":       "foo",
					"extra info":      "foo",
					"trace.parent_id": rootSpanID,
					"trace.span_id":   callSpanID,
					"trace.trace_id":  strings.TrimPrefix(traceID, "hctrace_"),

					// call timing entries
					"timing.dequeued_at":  differs.AnyString(),
					"timing.finished_at":  differs.AnyString(),
					"timing.scheduled_at": differs.AnyString(),
					"timing.service_time": differs.FloatRange(0, 5),
					"timing.total_time":   differs.FloatRange(0, 5),
					"timing.wait_time":    differs.FloatRange(0, 5),
				},
				{
					"meta.span_type": "root",
					"name":           "new trace",
					"trace.span_id":  rootSpanID,
					"trace.trace_id": strings.TrimPrefix(traceID, "hctrace_"),
				},
			}

			for _, each := range expected {
				each["app.version"] = differs.AnyString()
				each["duration_ms"] = differs.FloatRange(0, 5)
				each["meta.beeline_version"] = differs.AnyString()
				each["meta.local_hostname"] = differs.AnyString()
				each["service_name"] = "tracer-test"
			}

			assert.DeepEqual(t, expected, events, differs.Custom())
		})
	}
}

func extendEventsWithForceTraceAndSampleRate(expected []logf.F, test string) []logf.F {
	switch test {
	case forceTraceTest:
		for _, each := range expected {
			each["force_trace"] = "true"
		}
	case sampleRateTest:
		for _, each := range expected {
			each["sample_trace"] = uint(1)
		}
	default:
		expected = nil
	}
	return expected
}

func newRecorder(t *testing.T, tx *tracer.Tracer, samplePercent float64) *recorder {
	var r recorder
	var restoreSecrets, restoreConfig, closeTracer func()

	cleanups := cleanup.Funcs{&restoreSecrets, &restoreConfig, &closeTracer}
	defer cleanups.Run()

	restoreSecrets = secretstest.Fake(".honeycomb_api_key", "some fake value")
	restoreConfig = env.FakeTestConfig("trace.yaml", map[string]interface{}{
		"Honeycomb": map[string]interface{}{
			"SamplePercent": samplePercent,
			"APIHost":       "localhost",
			"Enabled":       true,
			"APIKey":        map[string]string{"Path": ".honeycomb_api_key"},
		},
	})

	ctx := context.Background()

	assert.NilError(t, tx.Init(ctx, "tracer-test"))
	closeTracer = func() { tx.Close(ctx) }

	tx.SetPresendHook(r.presendHook)

	r.close = cleanups.All()
	return &r
}

type recorder struct {
	events []logf.F
	close  func()
}

func (r *recorder) presendHook(event map[string]interface{}) {
	r.events = append(r.events, logf.F(event))
}

func (r *recorder) flush(ctx context.Context) []logf.F {
	beeline.Flush(ctx)
	return r.events
}
