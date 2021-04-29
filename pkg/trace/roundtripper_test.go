package trace_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
)

func (suite) TestRoundtripper(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	state := (suite{}).initRoundTripperState(t)
	defer state.Close()

	ctx := trace.StartTrace(context.Background(), "trace-rt")
	inner := trace.StartSpan(ctx, "inner")
	trace.AddInfo(inner, log.F{"trace": "inner"})

	client := http.Client{Transport: trace.NewTransport(nil)}
	req, err := http.NewRequest("GET", state.Server.URL+"/myendpoint", nil)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	res, err := client.Do(req.WithContext(inner))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	defer res.Body.Close()

	trace.End(inner)
	trace.End(ctx)

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"app.name":                  "gobox",
			"app.version":               "testing",
			"duration":                  differs.FloatRange(0, 10),
			"duration_ms":               differs.FloatRange(0, 10),
			"http.method":               string("GET"),
			"http.referer":              string(""),
			"http.request_id":           string(""),
			"http.status_code":          int(200),
			"http.url_details.endpoint": string("ep"),
			"http.url_details.path":     string("/myendpoint"),
			"http.url_details.uri":      string("/myendpoint"),
			"meta.beeline_version":      differs.AnyString(),
			"meta.local_hostname":       differs.AnyString(),
			"meta.span_type":            string("subroot"), // <--- this indicates trace is connected to remote event
			"name":                      string("ep"),
			"network.bytes_read":        int(0),
			"network.bytes_written":     int(2),
			"network.client.ip":         string(""),
			"network.destination.ip":    string(""),
			"service_name":              string("log-testing"),
			"timing.dequeued_at":        differs.RFC3339NanoTime(),
			"timing.finished_at":        differs.RFC3339NanoTime(),
			"timing.scheduled_at":       differs.RFC3339NanoTime(),
			"timing.service_time":       differs.FloatRange(0, 0.1),
			"timing.total_time":         differs.FloatRange(0, 0.1),
			"timing.wait_time":          differs.FloatRange(0, 0.1),
			"trace.span_id":             differs.AnyString(),
			"trace.trace_id":            traceID,
			"trace.parent_id":           middleID,
		},
		{
			"app.name":             "gobox",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 10),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       string("leaf"),
			"name":                 string("inner"),
			"service_name":         string("log-testing"),
			"trace":                string("inner"),
			"trace.parent_id":      rootID,
			"trace.span_id":        middleID,
			"trace.trace_id":       traceID,
		},
		{
			"app.name":             "gobox",
			"app.version":          "testing",
			"duration_ms":          differs.FloatRange(0, 10),
			"meta.beeline_version": differs.AnyString(),
			"meta.local_hostname":  differs.AnyString(),
			"meta.span_type":       string("root"),
			"name":                 string("trace-rt"),
			"service_name":         string("log-testing"),
			"trace.span_id":        rootID,
			"trace.trace_id":       traceID,
		},
	}

	ev := state.HoneycombEvents()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) initRoundTripperState(t *testing.T) *roundtripperState {
	trlog := tracetest.NewTraceLog()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(trace.ContextFromHTTP(r, "ep"))
		defer trace.End(r.Context())

		var info events.HTTPRequest
		info.FillFieldsFromRequest(r)
		info.Endpoint = "ep"
		if n, err := w.Write([]byte("OK")); err != nil {
			t.Fatal("Got error", err)
		} else {
			info.FillResponseInfo(n, 200)
		}
		trace.AddInfo(r.Context(), &info)
	}))

	return &roundtripperState{trlog, server}
}

type roundtripperState struct {
	*tracetest.TraceLog
	*httptest.Server
}

func (rt *roundtripperState) Close() {
	rt.TraceLog.Close()
	rt.Server.Close()
}
