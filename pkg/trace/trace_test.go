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
	"github.com/prometheus/client_golang/prometheus"
	"gotest.tools/v3/assert"
)

func TestAll(t *testing.T) {
	shuffler.Run(t, suite{})
}

type suite struct{}

func (suite) TestNestedSpan(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

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
			"app.name":             "gobox",
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
			"app.name":             "gobox",
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
			"app.name":             "gobox",
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
	app.SetName("gobox")

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
			"app.name":             "gobox",
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
			"app.name":             "gobox",
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
	app.SetName("gobox")

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

func (suite) TestNestingIDs(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("go-outreach")

	trlog := tracetest.NewTraceLog()
	defer trlog.Close()

	ctx0 := context.Background()
	ctx1 := trace.StartTrace(ctx0, "trace-test")
	trace.AddInfo(ctx1, log.F{"trace": "outermost"})
	info1 := log.F{}
	trace.IDs(ctx1).MarshalLog(info1.Set)
	pID1, sID1 := info1["honeycomb.parent_id"], info1["honeycomb.span_id"]
	assert.Equal(t, pID1, sID1)

	ctx2 := trace.StartCall(ctx1, "call-test") // StartCall == StartSpan
	defer func() {
		prometheus.DefaultGatherer.Gather()
	}()
	trace.AddInfo(ctx2, log.F{"trace": "call"})
	info2 := log.F{}
	trace.IDs(ctx2).MarshalLog(info2.Set)
	pID2, sID2 := info2["honeycomb.parent_id"], info2["honeycomb.span_id"]
	assert.Equal(t, pID2, sID1)

	ctx3 := trace.StartSpan(ctx2, "ctx3")
	trace.AddInfo(ctx3, log.F{"trace": "ctx3"})
	info3 := log.F{}
	trace.IDs(ctx3).MarshalLog(info3.Set)
	pID3, sID3 := info3["honeycomb.parent_id"], info3["honeycomb.span_id"]
	assert.Equal(t, pID3, sID2)

	ctx4 := trace.StartSpan(ctx3, "inner2x")
	trace.AddInfo(ctx4, log.F{"trace": "inner2x"})
	info4 := log.F{}
	trace.IDs(ctx4).MarshalLog(info4.Set)
	pID4 := info4["honeycomb.parent_id"]
	assert.Equal(t, pID4, sID3)

	trace.End(ctx4)
	trace.End(ctx3)
	trace.EndCall(ctx2)
	trace.End(ctx1)
}
