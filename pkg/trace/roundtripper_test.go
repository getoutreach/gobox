package trace_test

import (
	"context"
	"fmt"
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
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func (suite) TestRoundtripper(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	// don't care about specific ids but make sure same IDs are used in both settings
	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	tests := map[string]struct {
		tracerType string
		expected   []map[string]interface{}
	}{
		"honeycomb": {
			tracerType: "honeycomb",
			expected: []map[string]interface{}{
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
			},
		},
		"otel": {
			tracerType: "otel",
			expected: []map[string]interface{}{
				{
					"app.name":                  "gobox",
					"app.version":               "testing",
					"duration":                  differs.AnyString(),
					"http.flavor":               string("1.1"),
					"http.host":                 differs.AnyString(),
					"http.method":               string("GET"),
					"http.referer":              string(""),
					"http.request_id":           string(""),
					"http.scheme":               string("http"),
					"http.server_name":          string("ep"),
					"http.status_code":          string("200"),
					"http.target":               string("/myendpoint"),
					"http.url_details.endpoint": string("ep"),
					"http.url_details.path":     string("/myendpoint"),
					"http.url_details.uri":      string("/myendpoint"),
					"http.user_agent":           string("Go-http-client/1.1"),
					"net.host.ip":               string("127.0.0.1"),
					"net.host.port":             string(""),
					"net.peer.ip":               string("127.0.0.1"),
					"net.peer.port":             string(""),
					"net.transport":             string("ip_tcp"),
					"network.bytes_read":        string("0"),
					"network.bytes_written":     string("2"),
					"network.client.ip":         string(""),
					"network.destination.ip":    string(""),
					"timing.dequeued_at":        differs.RFC3339NanoTime(),
					"timing.finished_at":        differs.RFC3339NanoTime(),
					"timing.scheduled_at":       differs.RFC3339NanoTime(),
					"timing.service_time":       differs.AnyString(),
					"timing.total_time":         differs.AnyString(),
					"timing.wait_time":          differs.AnyString(),
				},
				{
					"app.name":    "gobox",
					"app.version": "testing",
					"trace":       string("inner"),
				},
				{
					"app.name":    "gobox",
					"app.version": "testing",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			state := (suite{}).initRoundTripperState(t, tc.tracerType)
			defer state.Close()

			ctx := trace.StartTrace(context.Background(), "trace-rt")
			inner := trace.StartSpan(ctx, "inner")
			trace.AddInfo(inner, log.F{"trace": "inner"})

			client := http.Client{Transport: trace.NewTransport(nil)}
			req, err := http.NewRequestWithContext(inner, "GET", state.Server.URL+"/myendpoint", http.NoBody)
			if err != nil {
				t.Fatal("Unexpected error", err)
			}
			res, err := client.Do(req)
			if err != nil {
				t.Fatal("Unexpected error", err)
			}
			defer res.Body.Close()

			trace.End(inner)
			trace.End(ctx)

			fmt.Println("ended spans")

			ev := state.HoneycombEvents()
			t.Logf("state: %#v", state)
			t.Logf("events: %#v", ev)
			if diff := cmp.Diff(tc.expected, ev, differs.Custom()); diff != "" {
				t.Fatal("unexpected events", diff)
			}
		})
	}
}

func (suite) initRoundTripperState(t *testing.T, tracerType string) *roundtripperState {
	trlog := tracetest.NewTraceLog(tracerType)
	if tracerType == "otel" {
		server := httptest.NewServer(
			otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trace.StartSpan(r.Context(), "ep")
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
			}), "ep"))

		return &roundtripperState{trlog, server}
	}
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
