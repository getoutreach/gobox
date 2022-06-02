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
	"gotest.tools/v3/assert"
)

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestNestedSpan(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                   "inner2",
			"spanContext.traceID":    traceID,
			"spanContext.spanID":     differs.AnyString(),
			"spanContext.traceFlags": "01",
			"parent.traceID":         traceID,
			"parent.spanID":          middleID,
			"parent.traceFlags":      "01",
			"parent.remote":          false,
			"spanKind":               "internal",
			"startTime":              differs.AnyString(),
			"endTime":                differs.AnyString(),
			"attributes.app.name":    "gobox",
			"attributes.app.version": "testing",
			"attributes.trace":       "inner2",
		},
		{
			"name":                   "inner",
			"spanContext.traceID":    traceID,
			"spanContext.spanID":     middleID,
			"spanContext.traceFlags": "01",
			"parent.traceID":         traceID,
			"parent.spanID":          rootID,
			"parent.traceFlags":      "01",
			"parent.remote":          false,
			"spanKind":               "internal",
			"startTime":              differs.AnyString(),
			"endTime":                differs.AnyString(),
			"attributes.app.name":    "gobox",
			"attributes.app.version": "testing",
			"attributes.from":        "inner_span",
			"attributes.trace":       "inner",
		},
		{
			"name":                   "trace-test",
			"spanContext.traceID":    traceID,
			"spanContext.spanID":     rootID,
			"spanContext.traceFlags": "01",
			"parent.traceID":         "00000000000000000000000000000000",
			"parent.spanID":          "0000000000000000",
			"parent.traceFlags":      "00",
			"parent.remote":          false,
			"spanKind":               "internal",
			"startTime":              differs.AnyString(),
			"endTime":                differs.AnyString(),
			"attributes.app.name":    "gobox",
			"attributes.app.version": "testing",
			"attributes.trace":       "outermost",
		},
		{
			"name":                   "innerAsync",
			"spanContext.traceID":    traceID,
			"spanContext.spanID":     differs.AnyString(),
			"spanContext.traceFlags": "01",
			"parent.traceID":         traceID,
			"parent.spanID":          middleID,
			"parent.traceFlags":      "01",
			"parent.remote":          false,
			"spanKind":               "internal",
			"startTime":              differs.AnyString(),
			"endTime":                differs.AnyString(),
			"attributes.app.name":    "gobox",
			"attributes.app.version": "testing",
			"attributes.trace":       "innerAsync",
		},
	}

	recorder := tracetest.NewSpanRecorder()
	defer recorder.Close()

	ctx := trace.StartTrace(context.Background(), "trace-test")
	trace.AddInfo(ctx, log.F{"trace": "outermost"})

	inner := trace.StartSpan(ctx, "inner", log.F{"from": "inner_span"})
	trace.AddInfo(inner, log.F{"trace": "inner"})

	innerAsync := trace.StartSpanAsync(inner, "innerAsync")
	trace.AddSpanInfo(innerAsync, log.F{"trace": "innerAsync"})

	inner2 := trace.StartSpan(inner, "inner2")
	trace.AddSpanInfo(inner2, log.F{"trace": "inner2"})

	trace.End(inner2)
	trace.End(inner)
	trace.End(ctx)

	// async trace ends out of band!
	trace.End(innerAsync)

	ev := recorder.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) TestIDHoneycomb(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLog("honeycomb")
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
			"app.name":             "gobox",
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
			"app.name":             "gobox",
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

func (suite) TestIDOtel(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLog("otel")
	defer trlog.Close()

	assert.Equal(t, "", trace.ID(context.Background()))

	ctx := trace.StartTrace(context.Background(), "trace-test")
	traceID := trace.ID(ctx)
	assert.Check(t, traceID != "")

	inner := trace.StartSpan(ctx, "inner")
	assert.Equal(t, traceID, trace.ID(inner))

	trace.End(inner)
	trace.End(ctx)
}
