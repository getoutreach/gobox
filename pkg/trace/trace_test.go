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
			"SampleRate":             int64(1),
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
			"SampleRate":             int64(1),
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
			"SampleRate":             int64(1),
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
			"SampleRate":             int64(1),
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
