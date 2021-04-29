package trace_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
)

func (suite) TestForceTracingByHeader(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLogWithOptions(tracetest.Options{
		SamplePercent: 1.0,
	})
	defer trlog.Close()

	headers := http.Header{}

	headers.Set(trace.HeaderForceTracing, "true")

	ctx := trace.FromHeaders(context.Background(), headers, "trace-test")

	traceID := trace.ID(ctx)

	inner := trace.StartSpan(ctx, "inner")

	trace.End(inner)
	trace.End(ctx)

	rootID := differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"app.name":             "gobox",
			"app.version":          app.Info().Version,
			"duration_ms":          differs.FloatRange(0, 2),
			"force_trace":          "true",
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "leaf",
			"name":                 "inner",
			"service_name":         "log-testing",
			"trace.span_id":        differs.AnyString(),
			"trace.parent_id":      rootID,
			"trace.trace_id":       traceID[len("hctrace_"):],
		},
		{
			"app.name":             "gobox",
			"app.version":          app.Info().Version,
			"duration_ms":          differs.FloatRange(0, 2),
			"force_trace":          "true",
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "root",
			"name":                 "trace-test",
			"service_name":         "log-testing",
			"trace.span_id":        rootID,
			"trace.trace_id":       traceID[len("hctrace_"):],
		},
	}

	ev := trlog.HoneycombEvents()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) TestForceTracingWithCascading(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLogWithOptions(tracetest.Options{
		SamplePercent: 1.0,
	})
	defer trlog.Close()

	// Enable tracing in one process
	ctx := trace.StartTrace(context.Background(), "trace-test")
	ctx = trace.ForceTracing(ctx)

	headers := trace.ToHeaders(ctx)
	trace.End(ctx)

	app.SetName("gobox-dep")

	// Start tracing again
	ctx = trace.FromHeaders(context.Background(), headers, "trace-test-dep")

	traceID := trace.ID(ctx)
	inner := trace.StartSpan(ctx, "inner")
	trace.End(inner)
	trace.End(ctx)

	rootID := differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"app.name":             "gobox",
			"app.version":          app.Info().Version,
			"duration_ms":          differs.FloatRange(0, 2),
			"force_trace":          "true",
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "root",
			"name":                 "trace-test",
			"service_name":         "log-testing",
			"trace.span_id":        rootID,
			"trace.trace_id":       traceID[len("hctrace_"):],
		},
		{
			"app.name":             "gobox-dep",
			"app.version":          app.Info().Version,
			"duration_ms":          differs.FloatRange(0, 2),
			"force_trace":          "true",
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "leaf",
			"name":                 "inner",
			"service_name":         "log-testing",
			"trace.span_id":        differs.AnyString(),
			"trace.parent_id":      differs.AnyString(),
			"trace.trace_id":       traceID[len("hctrace_"):],
		},
		{
			"app.name":             "gobox-dep",
			"app.version":          app.Info().Version,
			"duration_ms":          differs.FloatRange(0, 2),
			"force_trace":          "true",
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "subroot",
			"name":                 "trace-test-dep",
			"service_name":         "log-testing",
			"trace.span_id":        differs.AnyString(),
			"trace.parent_id":      rootID,
			"trace.trace_id":       traceID[len("hctrace_"):],
		},
	}

	ev := trlog.HoneycombEvents()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}
