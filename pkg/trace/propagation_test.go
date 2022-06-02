package trace_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/getoutreach/gobox/pkg/trace/tracetest"
	"github.com/google/go-cmp/cmp"
)

func (suite) TestForceTracingByHeader(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	recorder := tracetest.NewSpanRecorderWithOptions(tracetest.Options{
		SamplePercent: 0.1,
	})

	state := propagationInitRoundTripperState(t, recorder)
	defer state.Close()

	ctx := trace.StartSpan(context.Background(), "trace-test")

	client := http.Client{Transport: trace.NewTransport(nil)}
	req, err := http.NewRequestWithContext(ctx, "GET", state.Server.URL+"/myendpoint", http.NoBody)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}

	req.Header.Set(trace.HeaderForceTracing, "true")

	res, err := client.Do(req)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	defer res.Body.Close()

	trace.End(ctx)

	traceID := trace.ID(ctx)
	rootID := differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                   "ep",
			"spanContext.traceID":    traceID,
			"spanContext.spanID":     differs.AnyString(),
			"spanContext.traceFlags": "01",
			"parent.traceID":         traceID,
			"parent.spanID":          rootID,
			"parent.traceFlags":      "00",
			"parent.remote":          true,
			"spanKind":               "server",
			"startTime":              differs.AnyString(),
			"endTime":                differs.AnyString(),
			// Set as a boolean which has no string representation
			"attributes.force_trace":               "",
			"attributes.net.transport":             "ip_tcp",
			"attributes.net.peer.ip":               "127.0.0.1",
			"attributes.net.peer.port":             "",
			"attributes.net.host.ip":               "127.0.0.1",
			"attributes.net.host.port":             "",
			"attributes.http.method":               "GET",
			"attributes.http.target":               "/myendpoint",
			"attributes.http.server_name":          "ep",
			"attributes.app.name":                  "gobox",
			"attributes.app.version":               "testing",
			"attributes.duration":                  differs.AnyString(),
			"attributes.http.flavor":               "1.1",
			"attributes.http.host":                 differs.AnyString(),
			"attributes.http.referer":              "",
			"attributes.http.request_id":           "",
			"attributes.http.scheme":               "http",
			"attributes.http.status_code":          "200",
			"attributes.http.url_details.endpoint": "ep",
			"attributes.http.url_details.path":     "/myendpoint",
			"attributes.http.url_details.uri":      "/myendpoint",
			"attributes.http.user_agent":           "Go-http-client/1.1",
			"attributes.network.bytes_read":        "0",
			"attributes.network.bytes_written":     "2",
			"attributes.network.client.ip":         "",
			"attributes.network.destination.ip":    "",
			"attributes.timing.dequeued_at":        differs.RFC3339NanoTime(),
			"attributes.timing.finished_at":        differs.RFC3339NanoTime(),
			"attributes.timing.scheduled_at":       differs.RFC3339NanoTime(),
			"attributes.timing.service_time":       differs.AnyString(),
			"attributes.timing.total_time":         differs.AnyString(),
			"attributes.timing.wait_time":          differs.AnyString(),
		},
	}

	ev := recorder.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func (suite) TestForceTracing(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	recorder := tracetest.NewSpanRecorderWithOptions(tracetest.Options{
		SamplePercent: 0.1,
	})

	state := propagationInitRoundTripperState(t, recorder)
	defer state.Close()

	ctx := trace.StartSpan(context.Background(), "trace-test")
	ctx = trace.ForceTracing(ctx)

	client := http.Client{Transport: trace.NewTransport(nil)}
	req, err := http.NewRequestWithContext(ctx, "GET", state.Server.URL+"/myendpoint", http.NoBody)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	defer res.Body.Close()

	trace.End(ctx)

	traceID := trace.ID(ctx)
	rootID := differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                   "ep",
			"spanContext.traceID":    traceID,
			"spanContext.spanID":     differs.AnyString(),
			"spanContext.traceFlags": "01",
			"parent.traceID":         traceID,
			"parent.spanID":          rootID,
			"parent.traceFlags":      "00",
			"parent.remote":          true,
			"spanKind":               "server",
			"startTime":              differs.AnyString(),
			"endTime":                differs.AnyString(),
			// Set as a boolean which has no string representation
			"attributes.force_trace":               "",
			"attributes.net.transport":             "ip_tcp",
			"attributes.net.peer.ip":               "127.0.0.1",
			"attributes.net.peer.port":             "",
			"attributes.net.host.ip":               "127.0.0.1",
			"attributes.net.host.port":             "",
			"attributes.http.method":               "GET",
			"attributes.http.target":               "/myendpoint",
			"attributes.http.server_name":          "ep",
			"attributes.app.name":                  "gobox",
			"attributes.app.version":               "testing",
			"attributes.duration":                  differs.AnyString(),
			"attributes.http.flavor":               "1.1",
			"attributes.http.host":                 differs.AnyString(),
			"attributes.http.referer":              "",
			"attributes.http.request_id":           "",
			"attributes.http.scheme":               "http",
			"attributes.http.status_code":          "200",
			"attributes.http.url_details.endpoint": "ep",
			"attributes.http.url_details.path":     "/myendpoint",
			"attributes.http.url_details.uri":      "/myendpoint",
			"attributes.http.user_agent":           "Go-http-client/1.1",
			"attributes.network.bytes_read":        "0",
			"attributes.network.bytes_written":     "2",
			"attributes.network.client.ip":         "",
			"attributes.network.destination.ip":    "",
			"attributes.timing.dequeued_at":        differs.RFC3339NanoTime(),
			"attributes.timing.finished_at":        differs.RFC3339NanoTime(),
			"attributes.timing.scheduled_at":       differs.RFC3339NanoTime(),
			"attributes.timing.service_time":       differs.AnyString(),
			"attributes.timing.total_time":         differs.AnyString(),
			"attributes.timing.wait_time":          differs.AnyString(),
		},
	}

	ev := recorder.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected events", diff)
	}
}

func propagationInitRoundTripperState(t *testing.T, recorder *tracetest.SpanRecorder) *roundtripperState {
	t.Helper()
	server := httptest.NewServer(
		trace.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	return &roundtripperState{recorder, server}
}
