package trace_test

import (
	"context"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
)

func testForceSampler(ctx context.Context, force bool, handler func(context.Context, *tracetest.TraceLog)) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLogWithOptions(tracetest.Options{
		SamplePercent: 1,
	})
	defer trlog.Close()

	ctx = trace.StartTrace(ctx, "trace-test")
	if force {
		ctx = trace.ForceTracing(ctx)
	}

	trace.End(ctx)

	handler(ctx, trlog)
}

func (suite) TestForceSamplerWhenForced(t *testing.T) {
	ctx := context.Background()
	testForceSampler(ctx, true, func(ctx context.Context, trlog *tracetest.TraceLog) {
		traceID := trace.ID(ctx)
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
		}

		ev := trlog.HoneycombEvents()
		if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
			t.Fatal("unexpected events", diff)
		}
	})
}

func (suite) TestForceSamplerWhenNotForced(t *testing.T) {
	ctx := context.Background()
	testForceSampler(ctx, false, func(ctx context.Context, trlog *tracetest.TraceLog) {
		if ev := trlog.HoneycombEvents(); ev != nil {
			t.Fatal("unexpected events", ev)
		}
	})
}

func testSampleAtSampler(ctx context.Context, sampleRate uint, handler func(context.Context, *tracetest.TraceLog)) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLogWithOptions(tracetest.Options{
		SamplePercent: 1,
	})
	defer trlog.Close()

	ctx = trace.StartTrace(ctx, "trace-test")
	ctx = trace.ForceSampleRate(ctx, sampleRate)

	trace.End(ctx)

	handler(ctx, trlog)
}

func (suite) TestTraceSampleAt(t *testing.T) {
	ctx := context.Background()
	testSampleAtSampler(ctx, 1, func(ctx context.Context, trlog *tracetest.TraceLog) {
		traceID := trace.ID(ctx)
		rootID := differs.CaptureString()
		expected := []map[string]interface{}{
			{
				"app.name":             "gobox",
				"app.version":          app.Info().Version,
				"duration_ms":          differs.FloatRange(0, 2),
				"sample_trace":         uint(1),
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
	})
}
