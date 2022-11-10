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
	linkTraceID, linkID := differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                    "inner2",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      differs.AnyString(),
			"spanContext.traceFlags":  "01",
			"parent.traceID":          traceID,
			"parent.spanID":           middleID,
			"parent.traceFlags":       "01",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.trace":        "inner2",
			"SampleRate":              int64(1),
			"links": []map[string]interface{}{
				{
					"spanContext.traceID": linkTraceID,
					"spanContext.spanID":  linkID,
				},
			},
		},
		{
			"name":                    "inner",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      middleID,
			"spanContext.traceFlags":  "01",
			"parent.traceID":          traceID,
			"parent.spanID":           rootID,
			"parent.traceFlags":       "01",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.from":         "inner_span",
			"attributes.trace":        "inner",
			"SampleRate":              int64(1),
		},
		{
			"name":                    "link-span",
			"spanContext.traceID":     linkTraceID, // link will have its own trace ID - it represents remote span
			"spanContext.spanID":      linkID,
			"spanContext.traceFlags":  "01",
			"parent.traceID":          "00000000000000000000000000000000",
			"parent.spanID":           "0000000000000000",
			"parent.traceFlags":       "00",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"SampleRate":              int64(1),
		},
		{
			"name":                    "root-span",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      rootID,
			"spanContext.traceFlags":  "01",
			"parent.traceID":          "00000000000000000000000000000000",
			"parent.spanID":           "0000000000000000",
			"parent.traceFlags":       "00",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.trace":        "outermost",
			"SampleRate":              int64(1),
		},
		{
			"name":                    "innerAsync",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      differs.AnyString(),
			"spanContext.traceFlags":  "01",
			"parent.traceID":          traceID,
			"parent.spanID":           middleID,
			"parent.traceFlags":       "01",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.trace":        "innerAsync",
			"SampleRate":              int64(1),
		},
	}

	recorder := tracetest.NewSpanRecorder()
	defer recorder.Close()

	rootCtx := trace.StartSpan(context.Background(), "root-span")
	trace.AddInfo(rootCtx, log.F{"trace": "outermost"})

	linkCtx := trace.StartSpan(context.Background(), "link-span")
	linkHeaders := trace.ToHeaders(linkCtx)

	inner := trace.StartSpan(rootCtx, "inner", log.F{"from": "inner_span"})
	trace.AddInfo(inner, log.F{"trace": "inner"})

	innerAsync := trace.StartSpanAsync(inner, "innerAsync")
	trace.AddSpanInfo(innerAsync, log.F{"trace": "innerAsync"})

	opts := []trace.SpanStartOption{trace.WithLink(linkHeaders)}
	inner2 := trace.StartSpanWithOptions(inner, "inner2", opts)
	trace.AddSpanInfo(inner2, log.F{"trace": "inner2"})

	trace.End(inner2)
	trace.End(inner)
	trace.End(linkCtx)
	trace.End(rootCtx)

	// async trace ends out of band!
	trace.End(innerAsync)

	ev := recorder.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) TestIncludesDevEmail(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()
	linkTraceID, linkID := differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                    "inner2",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      differs.AnyString(),
			"spanContext.traceFlags":  "01",
			"parent.traceID":          traceID,
			"parent.spanID":           middleID,
			"parent.traceFlags":       "01",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.dev.email":    "test@test.com",
			"attributes.trace":        "inner2",
			"SampleRate":              int64(1),
			"links": []map[string]interface{}{
				{
					"spanContext.traceID": linkTraceID,
					"spanContext.spanID":  linkID,
				},
			},
		},
		{
			"name":                    "inner",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      middleID,
			"spanContext.traceFlags":  "01",
			"parent.traceID":          traceID,
			"parent.spanID":           rootID,
			"parent.traceFlags":       "01",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.from":         "inner_span",
			"attributes.trace":        "inner",
			"attributes.dev.email":    "test@test.com",
			"SampleRate":              int64(1),
		},
		{
			"name":                    "link-span",
			"spanContext.traceID":     linkTraceID, // link will have its own trace ID - it represents remote span
			"spanContext.spanID":      linkID,
			"spanContext.traceFlags":  "01",
			"parent.traceID":          "00000000000000000000000000000000",
			"parent.spanID":           "0000000000000000",
			"parent.traceFlags":       "00",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.dev.email":    "test@test.com",
			"SampleRate":              int64(1),
		},
		{
			"name":                    "root-span",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      rootID,
			"spanContext.traceFlags":  "01",
			"parent.traceID":          "00000000000000000000000000000000",
			"parent.spanID":           "0000000000000000",
			"parent.traceFlags":       "00",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.trace":        "outermost",
			"attributes.dev.email":    "test@test.com",
			"SampleRate":              int64(1),
		},
		{
			"name":                    "innerAsync",
			"spanContext.traceID":     traceID,
			"spanContext.spanID":      differs.AnyString(),
			"spanContext.traceFlags":  "01",
			"parent.traceID":          traceID,
			"parent.spanID":           middleID,
			"parent.traceFlags":       "01",
			"parent.remote":           false,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"endTime":                 differs.AnyString(),
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.trace":        "innerAsync",
			"attributes.dev.email":    "test@test.com",
			"SampleRate":              int64(1),
		},
	}

	recorder := tracetest.NewSpanRecorderWithOptions(
		tracetest.Options{
			SamplePercent: 100.0,
			DevEmail:      "test@test.com",
		})
	defer recorder.Close()

	rootCtx := trace.StartSpan(context.Background(), "root-span")
	trace.AddInfo(rootCtx, log.F{"trace": "outermost"})

	linkCtx := trace.StartSpan(context.Background(), "link-span")
	linkHeaders := trace.ToHeaders(linkCtx)

	inner := trace.StartSpan(rootCtx, "inner", log.F{"from": "inner_span"})
	trace.AddInfo(inner, log.F{"trace": "inner"})

	innerAsync := trace.StartSpanAsync(inner, "innerAsync")
	trace.AddSpanInfo(innerAsync, log.F{"trace": "innerAsync"})

	opts := []trace.SpanStartOption{trace.WithLink(linkHeaders)}
	inner2 := trace.StartSpanWithOptions(inner, "inner2", opts)
	trace.AddSpanInfo(inner2, log.F{"trace": "inner2"})

	trace.End(inner2)
	trace.End(inner)
	trace.End(linkCtx)
	trace.End(rootCtx)

	// async trace ends out of band!
	trace.End(innerAsync)

	ev := recorder.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}
