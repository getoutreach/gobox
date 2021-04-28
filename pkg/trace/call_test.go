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
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"

	"github.com/prometheus/client_golang/prometheus"
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

func (suite) TestNestedCall(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	trlog := tracetest.NewTraceLog()
	defer trlog.Close()
	logs := logtest.NewLogRecorder(t)
	defer logs.Close()

	ctx := context.Background()

	//  most functions should look like this
	doSomeTableUpdate := func(ctx context.Context, rowID string, logs ...log.Marshaler) error {
		ctx = trace.StartCall(ctx, "sql", SQLQuery("my query: "+rowID), log.Many(logs))
		defer trace.EndCall(ctx)

		// do some query work

		// report errors
		return trace.SetCallStatus(ctx, errors.New("sql error"))
	}

	// *model* function calls doSomeTableUpdate
	outer := func(ctx context.Context, m *Model) error {
		ctx = trace.StartCall(ctx, "model", m)
		defer trace.EndCall(ctx)

		// note that m is passed here to ensure it gets logged along with SQL queries in M
		return trace.SetCallStatus(ctx, doSomeTableUpdate(ctx, m.ID, m))
	}

	// wrapping the main logic in a function so that we can call
	// defer per our accepted trace.StartTrace/trace.End pattern
	func() {
		ctx = trace.StartTrace(ctx, "trace-test")
		defer trace.End(ctx)

		if err := outer(ctx, &Model{ID: "some model id"}); err == nil {
			t.Fatal("unexpected success", err)
		}
	}()

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"app.name":             "gobox",
			"app.version":          "testing",
			"sql.query":            "my query: some model id",
			"model.id":             "some model id",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "leaf",
			"name":                 "sql",
			"service_name":         "log-testing",
			"error.kind":           "error",
			"error.error":          "sql error",
			"error.message":        "sql error",
			"error.stack":          differs.AnyString(),
			"trace.parent_id":      middleID,
			"trace.span_id":        differs.AnyString(),
			"trace.trace_id":       traceID,
			"timing.dequeued_at":   differs.RFC3339NanoTime(),
			"timing.finished_at":   differs.RFC3339NanoTime(),
			"timing.scheduled_at":  differs.RFC3339NanoTime(),
			"timing.service_time":  differs.FloatRange(0, 1),
			"timing.total_time":    differs.FloatRange(0, 1),
			"timing.wait_time":     differs.FloatRange(0, 1),
		},
		{
			"app.name":             "gobox",
			"app.version":          "testing",
			"error.kind":           "error",
			"error.error":          "sql error",
			"error.message":        "sql error",
			"error.stack":          differs.AnyString(),
			"model.id":             "some model id",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "leaf",
			"name":                 "model",
			"service_name":         "log-testing",
			"trace.parent_id":      rootID,
			"trace.span_id":        middleID,
			"trace.trace_id":       traceID,
			"timing.dequeued_at":   differs.RFC3339NanoTime(),
			"timing.finished_at":   differs.RFC3339NanoTime(),
			"timing.scheduled_at":  differs.RFC3339NanoTime(),
			"timing.service_time":  differs.FloatRange(0, 1),
			"timing.total_time":    differs.FloatRange(0, 1),
			"timing.wait_time":     differs.FloatRange(0, 1),
		},
		{
			"app.name":             "gobox",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 2),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       "root",
			"name":                 "trace-test",
			"trace.span_id":        rootID,
			"trace.trace_id":       traceID,
			"service_name":         "log-testing",
		},
	}

	ev := trlog.HoneycombEvents()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}

	expectedLogs := []log.F{
		{
			"app.name":    "gobox",
			"app.version": string("testing"),
			"@timestamp":  differs.RFC3339NanoTime(),
			"level":       "DEBUG",
			"message":     "calling: model",
			"model.id":    "some model id",
		},
		{
			"app.name":    "gobox",
			"app.version": string("testing"),
			"@timestamp":  differs.RFC3339NanoTime(),
			"level":       "DEBUG",
			"message":     "calling: sql",
			"model.id":    "some model id",
			"sql.query":   "my query: some model id",
		},
		{
			"app.name":            "gobox",
			"app.version":         string("testing"),
			"@timestamp":          differs.RFC3339NanoTime(),
			"error.kind":          "error",
			"error.error":         "sql error",
			"error.message":       "sql error",
			"error.stack":         differs.StackLike("gobox/pkg/trace/call_test.go:54 `trace_test.suite.TestNestedCall.func1`"),
			"level":               "ERROR",
			"message":             "sql",
			"model.id":            "some model id",
			"sql.query":           "my query: some model id",
			"timing.dequeued_at":  differs.RFC3339NanoTime(),
			"timing.finished_at":  differs.RFC3339NanoTime(),
			"timing.scheduled_at": differs.RFC3339NanoTime(),
			"timing.service_time": differs.FloatRange(0, 0.1),
			"timing.total_time":   differs.FloatRange(0, 0.1),
			"timing.wait_time":    float64(0),
		},
		{
			"app.name":            "gobox",
			"app.version":         string("testing"),
			"@timestamp":          differs.RFC3339NanoTime(),
			"error.kind":          "error",
			"error.error":         "sql error",
			"error.message":       "sql error",
			"error.stack":         differs.StackLike("gobox/pkg/trace/call_test.go:54 `trace_test.suite.TestNestedCall.func1`"),
			"level":               "ERROR",
			"message":             "model",
			"model.id":            "some model id",
			"timing.dequeued_at":  differs.RFC3339NanoTime(),
			"timing.finished_at":  differs.RFC3339NanoTime(),
			"timing.scheduled_at": differs.RFC3339NanoTime(),
			"timing.service_time": differs.FloatRange(0, 0.1),
			"timing.total_time":   differs.FloatRange(0, 0.1),
			"timing.wait_time":    float64(0),
		},
	}
	if diff := cmp.Diff(expectedLogs, logs.Entries(), differs.Custom()); diff != "" {
		t.Fatal("unexpected log entries", diff)
	}

	// TODO: convert this to a recorder pattern as well
	metricsInfo := getMetricsInfo(t)
	expectedMetrics := []map[string]interface{}{
		{
			"bucket":       "[cumulative_count:1 upper_bound:0.005  cumulative_count:1 upper_bound:0.01  cumulative_count:1 upper_bound:0.025  cumulative_count:1 upper_bound:0.05  cumulative_count:1 upper_bound:0.1  cumulative_count:1 upper_bound:0.25  cumulative_count:1 upper_bound:0.5  cumulative_count:1 upper_bound:1  cumulative_count:1 upper_bound:2.5  cumulative_count:1 upper_bound:5  cumulative_count:1 upper_bound:10 ]",
			"help":         "The latency of the call",
			"label":        `[name:"app" value:"gobox"  name:"call" value:"model"  name:"status" value:"error" ]`,
			"name":         "call_request_seconds",
			"sample count": uint64(0x01),
			"summary":      "<nil>",
			"type":         "HISTOGRAM",
		},
		{
			"bucket":       "[cumulative_count:1 upper_bound:0.005  cumulative_count:1 upper_bound:0.01  cumulative_count:1 upper_bound:0.025  cumulative_count:1 upper_bound:0.05  cumulative_count:1 upper_bound:0.1  cumulative_count:1 upper_bound:0.25  cumulative_count:1 upper_bound:0.5  cumulative_count:1 upper_bound:1  cumulative_count:1 upper_bound:2.5  cumulative_count:1 upper_bound:5  cumulative_count:1 upper_bound:10 ]",
			"help":         "The latency of the call",
			"label":        `[name:"app" value:"gobox"  name:"call" value:"sql"  name:"status" value:"error" ]`,
			"name":         "call_request_seconds",
			"sample count": uint64(0x01),
			"summary":      "<nil>",
			"type":         "HISTOGRAM",
		},
	}
	if diff := cmp.Diff(expectedMetrics, metricsInfo); diff != "" {
		t.Fatal("unexpected metrics entries", diff)
	}
}

func getMetricsInfo(t *testing.T) []map[string]interface{} {
	got, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal("metrics info", err)
	}

	result := []map[string]interface{}{}
	for _, metricFamily := range got {
		if metricFamily.GetName() == "call_request_seconds" {
			for _, metric := range metricFamily.Metric {
				found := false
				for _, labelPair := range metric.GetLabel() {
					if labelPair.GetName() == "status" {
						found = true
					}
				}
				if !found {
					continue
				}
				info := map[string]interface{}{
					"name":         metricFamily.GetName(),
					"help":         metricFamily.GetHelp(),
					"type":         fmt.Sprint(metricFamily.GetType()),
					"label":        fmt.Sprint(metric.GetLabel()),
					"summary":      fmt.Sprint(metric.GetSummary()),
					"sample count": metric.GetHistogram().GetSampleCount(),
					"bucket":       fmt.Sprint(metric.GetHistogram().GetBucket()),
				}
				result = append(result, info)
			}
		}
	}
	return result
}
