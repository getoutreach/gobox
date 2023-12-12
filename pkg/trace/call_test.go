// FIXME: restore stuff here.

package trace_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"gotest.tools/v3/assert"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type SQLQuery string

func (s SQLQuery) MarshalLog(addField func(k string, v interface{})) {
	addField("sql.query", string(s))
}

type Model struct {
	ID string
}

func (m *Model) MarshalLog(addField func(k string, v interface{})) {
	addField("model.id", m.ID)
}

func TestNestedCall(t *testing.T) {
	t.Skip("flaky test")

	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                           "sql",
			"spanContext.traceID":            traceID,
			"spanContext.spanID":             differs.AnyString(),
			"spanContext.traceFlags":         "01",
			"parent.traceID":                 traceID,
			"parent.spanID":                  middleID,
			"parent.traceFlags":              "01",
			"parent.remote":                  false,
			"spanKind":                       "internal",
			"startTime":                      differs.AnyString(),
			"endTime":                        differs.AnyString(),
			"attributes.app.name":            "gobox",
			"attributes.service_name":        "gobox",
			"attributes.app.version":         "testing",
			"attributes.sql.query":           "my query: some model id",
			"attributes.model.id":            "some model id",
			"attributes.error.kind":          "error",
			"attributes.error.error":         "sql error",
			"attributes.error.message":       "sql error",
			"attributes.error.stack":         differs.AnyString(),
			"attributes.info_1_key":          "info_1_val",
			"attributes.info_2_key":          "info_2_val",
			"attributes.timing.dequeued_at":  differs.RFC3339NanoTime(),
			"attributes.timing.finished_at":  differs.RFC3339NanoTime(),
			"attributes.timing.scheduled_at": differs.RFC3339NanoTime(),
			"attributes.timing.service_time": differs.AnyString(),
			"attributes.timing.total_time":   differs.AnyString(),
			"attributes.timing.wait_time":    differs.AnyString(),
			"SampleRate":                     int64(1),
		},
		{
			"name":                           "model",
			"spanContext.traceID":            traceID,
			"spanContext.spanID":             middleID,
			"spanContext.traceFlags":         "01",
			"parent.traceID":                 traceID,
			"parent.spanID":                  rootID,
			"parent.traceFlags":              "01",
			"parent.remote":                  false,
			"spanKind":                       "internal",
			"startTime":                      differs.AnyString(),
			"endTime":                        differs.AnyString(),
			"attributes.app.name":            "gobox",
			"attributes.service_name":        "gobox",
			"attributes.app.version":         "testing",
			"attributes.error.kind":          "error",
			"attributes.error.error":         "sql error",
			"attributes.error.message":       "sql error",
			"attributes.error.stack":         differs.AnyString(),
			"attributes.model.id":            "some model id",
			"attributes.info_3_key":          "info_3_val",
			"attributes.info_4_key":          "info_4_val",
			"attributes.timing.dequeued_at":  differs.RFC3339NanoTime(),
			"attributes.timing.finished_at":  differs.RFC3339NanoTime(),
			"attributes.timing.scheduled_at": differs.RFC3339NanoTime(),
			"attributes.timing.service_time": differs.AnyString(),
			"attributes.timing.total_time":   differs.AnyString(),
			"attributes.timing.wait_time":    differs.AnyString(),
			"SampleRate":                     int64(1),
		},
		{
			"name":                    "trace-test",
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
			"SampleRate":              int64(1),
		},
	}

	expectedLogs := []log.F{
		{
			"app.name":     "gobox",
			"service_name": "gobox",
			"app.version":  string("testing"),
			"@timestamp":   differs.RFC3339NanoTime(),
			"level":        "DEBUG",
			"message":      "calling: model",
			"model.id":     "some model id",
		},
		{
			"app.name":     "gobox",
			"service_name": "gobox",
			"app.version":  string("testing"),
			"@timestamp":   differs.RFC3339NanoTime(),
			"level":        "DEBUG",
			"message":      "calling: sql",
			"model.id":     "some model id",
			"sql.query":    "my query: some model id",
		},
		{
			"app.name":            "gobox",
			"service_name":        "gobox",
			"app.version":         string("testing"),
			"@timestamp":          differs.RFC3339NanoTime(),
			"honeycomb.trace_id":  differs.AnyString(),
			"honeycomb.parent_id": differs.AnyString(),
			"honeycomb.span_id":   differs.AnyString(),
			"event_name":          "trace",
			"error.kind":          "error",
			"error.error":         "sql error",
			"error.message":       "sql error",
			//nolint:lll // Why: Output comparision
			"error.stack":         differs.StackLike("gobox/pkg/trace/call_test.go:139 `trace_test.suite.TestNestedCall.func1`"),
			"level":               "ERROR",
			"message":             "sql",
			"model.id":            "some model id",
			"sql.query":           "my query: some model id",
			"info_1_key":          "info_1_val",
			"info_2_key":          "info_2_val",
			"timing.dequeued_at":  differs.RFC3339NanoTime(),
			"timing.finished_at":  differs.RFC3339NanoTime(),
			"timing.scheduled_at": differs.RFC3339NanoTime(),
			"timing.service_time": differs.FloatRange(0, 0.1),
			"timing.total_time":   differs.FloatRange(0, 0.1),
			"timing.wait_time":    float64(0),
		},
		{
			"app.name":            "gobox",
			"service_name":        "gobox",
			"app.version":         string("testing"),
			"honeycomb.trace_id":  differs.AnyString(),
			"honeycomb.parent_id": differs.AnyString(),
			"honeycomb.span_id":   differs.AnyString(),
			"event_name":          "trace",
			"@timestamp":          differs.RFC3339NanoTime(),
			"error.kind":          "error",
			"error.error":         "sql error",
			"error.message":       "sql error",
			//nolint:lll // Why: Output comparision
			"error.stack":         differs.StackLike("gobox/pkg/trace/call_test.go:139 `trace_test.suite.TestNestedCall.func1`"),
			"level":               "ERROR",
			"message":             "model",
			"model.id":            "some model id",
			"info_3_key":          "info_3_val",
			"info_4_key":          "info_4_val",
			"timing.dequeued_at":  differs.RFC3339NanoTime(),
			"timing.finished_at":  differs.RFC3339NanoTime(),
			"timing.scheduled_at": differs.RFC3339NanoTime(),
			"timing.service_time": differs.FloatRange(0, 0.1),
			"timing.total_time":   differs.FloatRange(0, 0.1),
			"timing.wait_time":    float64(0),
		},
	}

	recorder := tracetest.NewSpanRecorder()
	defer recorder.Close()

	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()

	//  most functions should look like this
	doSomeTableUpdate := func(ctx context.Context, rowID string, logs ...log.Marshaler) error {
		ctx = trace.StartCall(ctx, "sql", SQLQuery("my query: "+rowID), log.Many(logs))
		defer trace.EndCall(ctx)

		trace.AddInfo(ctx, log.F{"info_1_key": "info_1_val", "info_2_key": "info_2_val"})
		// do some query work

		// report errors
		return trace.SetCallStatus(ctx, errors.New("sql error"))
	}

	// *model* function calls doSomeTableUpdate
	outer := func(ctx context.Context, m *Model) error {
		ctx = trace.StartCall(ctx, "model", m)
		defer trace.EndCall(ctx)

		trace.AddInfo(ctx, log.F{"info_3_key": "info_3_val", "info_4_key": "info_4_val"})
		// note that m is passed here to ensure it gets logged along with SQL queries in M
		return trace.SetCallStatus(ctx, doSomeTableUpdate(ctx, m.ID, m))
	}

	// wrapping the main logic in a function so that we can call
	// defer per our accepted trace.StartSpan/trace.End pattern
	func() {
		ctx = trace.StartSpan(ctx, "trace-test")
		defer trace.End(ctx)

		if err := outer(ctx, &Model{ID: "some model id"}); err == nil {
			t.Fatal("unexpected success", err)
		}
	}()

	entries := logs.Entries()
	if diff := cmp.Diff(expectedLogs, entries, differs.Custom()); diff != "" {
		fmt.Printf("%#v", diff[1])
		t.Fatal("unexpected log entries", diff)
	}

	ev := recorder.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}

	errmap := map[string]error{
		"WARN":  orerr.NewErrorStatus(errors.New("invalid input data"), statuscodes.BadRequest),
		"ERROR": orerr.NewErrorStatus(errors.New("cannot access DB"), statuscodes.InternalServerError),
		"INFO":  orerr.NewErrorStatus(errors.New("success"), statuscodes.OK),
	}

	for level, err := range errmap {
		logs := logtest.NewLogRecorder(t)
		ctx = trace.StartCall(ctx, "start call")
		trace.SetCallError(ctx, err)
		trace.EndCall(ctx)
		// now check logs to see that the right warning message exists
		logs.Close()
		lastEntry := logs.Entries()[len(logs.Entries())-1]
		assert.Equal(t, lastEntry["level"], level)
		assert.Equal(t, lastEntry["message"], "start call")
	}
}

func TestReportLatencyMetrics(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()

	httpCall := func(ctx context.Context) error {
		ctx = trace.StartCall(ctx, "test", trace.AsHTTPCall())
		defer trace.EndCall(ctx)

		return trace.SetCallStatus(ctx, nil)
	}

	// wrapping the main logic in a function so that we can call
	// defer per our accepted trace.StartSpan/trace.End pattern
	func() {
		ctx = trace.StartSpan(ctx, "trace-test")
		defer trace.End(ctx)

		if err := httpCall(ctx); err != nil {
			t.Fatal("unexpected error", err)
		}
	}()

	metricsInfo, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal("metrics info", err)
	}

	var relevantMetricFamily *dto.MetricFamily
	for _, metricFamily := range metricsInfo {
		if metricFamily.GetName() == "http_request_handled" {
			relevantMetricFamily = metricFamily
		}
	}
	if relevantMetricFamily == nil {
		t.Fatal("no http_request_duration_seconds metric found")
	}
	assert.Check(t, cmp.Equal(relevantMetricFamily.GetType(), dto.MetricType_HISTOGRAM))
	assert.Check(t, cmp.Equal(relevantMetricFamily.GetHelp(), "The latency of the HTTP request, in seconds"))

	assert.Equal(t, len(relevantMetricFamily.GetMetric()), 1)
	metric := relevantMetricFamily.GetMetric()[0]

	assert.Check(t, cmp.Equal(metric.GetHistogram().GetSampleCount(), uint64(1)))
	assert.Check(t, cmp.Equal(metric.GetHistogram().GetBucket(), []*dto.Bucket{
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.005)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.01)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.025)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.05)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.1)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.25)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.5)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(1)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(2.5)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(5)},
		{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(10)},
	}, protocmp.Transform()))

	assert.Check(t, cmp.Equal(metric.GetLabel(), []*dto.LabelPair{
		{Name: proto.String("app"), Value: proto.String("gobox")},
		{Name: proto.String("call"), Value: proto.String("test")},
		{Name: proto.String("kind"), Value: proto.String("internal")},
		{Name: proto.String("statuscategory"), Value: proto.String("CategoryOK")},
		{Name: proto.String("statuscode"), Value: proto.String("OK")},
	}, protocmp.Transform()))
}

func TestEndCallDoesNotPanicWithNilError(t *testing.T) {
	t.Skip("requires method to clear metrics between tests")

	ctx := trace.StartCall(context.Background(), "")
	trace.EndCall(ctx)
}

func TestSetCallStatusDoesNotPanicWithNilInfo(t *testing.T) {
	trace.SetCallStatus(context.Background(), errors.New(""))
}
