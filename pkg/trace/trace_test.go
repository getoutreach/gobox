package trace_test

import (
	"context"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/shuffler"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
	hctrace "github.com/honeycombio/beeline-go/trace"
	"gotest.tools/v3/assert"
)

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestNestedSpan(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("go-outreach")

	trlog := tracetest.NewTraceLog()
	defer trlog.Close()

	ctx := trace.StartTrace(context.Background(), "trace-test")
	trace.AddInfo(ctx, log.F{"trace": "outermost"})

	inner := trace.StartSpan(ctx, "inner")
	trace.AddInfo(inner, log.F{"trace": "inner"})

	inner2 := trace.StartSpan(inner, "inner2")
	trace.AddInfo(inner2, log.F{"trace": "inner2"})

	trace.End(inner2)
	trace.End(inner)
	trace.End(ctx)

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "leaf",
			"name":                 "inner2",
			"service_name":         "log-testing",
			"trace":                "inner2",
			"trace.parent_id":      middleID,
			"trace.span_id":        differs.AnyString(),
			"trace.trace_id":       traceID,
		},
		{
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "leaf",
			"name":                 "inner",
			"service_name":         "log-testing",
			"trace":                "inner",
			"trace.parent_id":      rootID,
			"trace.span_id":        middleID,
			"trace.trace_id":       traceID,
		},
		{
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "root",
			"name":                 "trace-test",
			"service_name":         "log-testing",
			"trace":                "outermost",
			"trace.span_id":        rootID,
			"trace.trace_id":       traceID,
		},
	}

	ev := trlog.HoneycombEvents()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) TestTrace(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("go-outreach")

	trlog := tracetest.NewTraceLog()
	defer trlog.Close()

	ctx := trace.StartTrace(context.Background(), "trace-test")
	span := hctrace.GetSpanFromContext(ctx)
	if span == nil {
		t.Fatal("failed to create a root span")
	}
	if parent := span.GetParent(); parent != nil {
		t.Fatal("Did not create a root span!", parent)
	}

	// try creating another trace and ensure that it is also
	// a root span
	ctx2 := trace.StartTrace(ctx, "trace-inner")
	span = hctrace.GetSpanFromContext(ctx2)
	if span == nil || span.GetParent() != nil {
		t.Fatal("Did not create a root span")
	}
	trace.AddInfo(ctx2, log.F{"inner": "inner"})
	trace.End(ctx2)
	trace.AddInfo(ctx, log.F{"outer": "outer"})
	trace.End(ctx)

	expected := []map[string]interface{}{
		{
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
			"inner":                "inner",
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "root",
			"name":                 "trace-inner",
			"service_name":         "log-testing",
			"trace.span_id":        differs.AnyString(),
			"trace.trace_id":       differs.AnyString(),
		},
		{
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "root",
			"name":                 "trace-test",
			"outer":                "outer",
			"service_name":         "log-testing",
			"trace.span_id":        differs.AnyString(),
			"trace.trace_id":       differs.AnyString(),
		},
	}

	ev := trlog.HoneycombEvents()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) TestID(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("go-outreach")

	trlog := tracetest.NewTraceLog()
	defer trlog.Close()

	assert.Equal(t, "", trace.ID(context.Background()))

	ctx := trace.StartTrace(context.Background(), "trace-test")
	traceID := trace.ID(ctx)
	assert.Check(t, traceID != "")

	inner := trace.StartSpan(ctx, "inner")
	assert.Equal(t, traceID, trace.ID(inner))

	trace.End(inner)
	trace.End(ctx)

	rootID := differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
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
			"app.name":             "go-outreach",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
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
