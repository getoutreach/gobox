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
	"github.com/honeycombio/beeline-go/propagation"
)

type initRoundTripperStateFunc func(t *testing.T) *roundtripperState
type callRoudTripperFunc func(t *testing.T, state *roundtripperState) []map[string]interface{}

func (suite) TestRoundtripper(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	testCases := []struct {
		name                  string
		initRoundTripperState initRoundTripperStateFunc
		callRoundTripper      callRoudTripperFunc
		expectedGen           func(traceId, rootID, middleID differs.CustomComparer) []map[string]interface{}
	}{
		{
			name:                  "OtelServerOtelClient",
			initRoundTripperState: otelInitRoundTripperState,
			callRoundTripper:      otelCallRoundTripper,
			expectedGen: func(traceID, rootID, middleID differs.CustomComparer) []map[string]interface{} {
				return []map[string]interface{}{
					{
						"name":                                 "ep",
						"spanContext.traceID":                  traceID,
						"spanContext.spanID":                   differs.AnyString(),
						"spanContext.traceFlags":               "01",
						"parent.traceID":                       traceID,
						"parent.spanID":                        differs.AnyString(),
						"parent.traceFlags":                    "01",
						"parent.remote":                        true,
						"spanKind":                             "server",
						"startTime":                            differs.AnyString(),
						"endTime":                              differs.AnyString(),
						"attributes.net.transport":             "ip_tcp",
						"attributes.net.peer.ip":               "127.0.0.1",
						"attributes.net.peer.port":             "",
						"attributes.net.host.ip":               "127.0.0.1",
						"attributes.net.host.port":             "",
						"attributes.http.method":               "GET",
						"attributes.http.target":               "/myendpoint",
						"attributes.http.server_name":          "ep",
						"attributes.app.name":                  "gobox",
						"attributes.service_name":              "gobox",
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
						"SampleRate":                           int64(1),
					},
					{
						"attributes.app.name":         "gobox",
						"attributes.service_name":     "gobox",
						"attributes.app.version":      "testing",
						"attributes.http.flavor":      "1.1",
						"attributes.http.host":        differs.AnyString(),
						"attributes.http.method":      "GET",
						"attributes.http.scheme":      "http",
						"attributes.http.status_code": "",
						"attributes.http.url":         differs.AnyString(),
						"endTime":                     differs.AnyString(),
						"name":                        "HTTP GET",
						"parent.remote":               false,
						"parent.spanID":               middleID,
						"parent.traceFlags":           "01",
						"parent.traceID":              traceID,
						"spanContext.spanID":          differs.AnyString(),
						"spanContext.traceFlags":      "01",
						"spanContext.traceID":         traceID,
						"spanKind":                    "client",
						"startTime":                   differs.AnyString(),
						"SampleRate":                  int64(1),
					},
					{
						"attributes.app.name":     "gobox",
						"attributes.service_name": "gobox",
						"attributes.app.version":  "testing",
						"attributes.trace":        "inner",
						"endTime":                 differs.AnyString(),
						"name":                    "inner",
						"parent.remote":           false,
						"parent.spanID":           rootID,
						"parent.traceFlags":       "01",
						"parent.traceID":          traceID,
						"spanContext.spanID":      middleID,
						"spanContext.traceFlags":  "01",
						"spanContext.traceID":     traceID,
						"spanKind":                "internal",
						"startTime":               differs.AnyString(),
						"SampleRate":              int64(1),
					},
					{
						"attributes.app.name":     "gobox",
						"attributes.service_name": "gobox",
						"attributes.app.version":  "testing",
						"endTime":                 differs.AnyString(),
						"name":                    "trace-rt",
						"parent.remote":           false,
						"parent.spanID":           "0000000000000000",
						"parent.traceFlags":       "00",
						"parent.traceID":          "00000000000000000000000000000000",
						"spanContext.spanID":      rootID,
						"spanContext.traceFlags":  "01",
						"spanContext.traceID":     traceID,
						"spanKind":                "internal",
						"startTime":               differs.AnyString(),
						"SampleRate":              int64(1),
					},
				}
			},
		},
		{
			name:                  "HoneycombServerOtelClient",
			initRoundTripperState: honeycombInitRoundTripperState,
			callRoundTripper:      otelCallRoundTripper,
			expectedGen: func(traceID, rootID, middleID differs.CustomComparer) []map[string]interface{} {
				return []map[string]interface{}{
					{
						"attributes.app.name":         "gobox",
						"attributes.service_name":     "gobox",
						"attributes.app.version":      "testing",
						"attributes.http.flavor":      "1.1",
						"attributes.http.host":        differs.AnyString(),
						"attributes.http.method":      "GET",
						"attributes.http.scheme":      "http",
						"attributes.http.status_code": "",
						"attributes.http.url":         differs.AnyString(),
						"endTime":                     differs.AnyString(),
						"name":                        "HTTP GET",
						"parent.remote":               false,
						"parent.spanID":               middleID,
						"parent.traceFlags":           "01",
						"parent.traceID":              traceID,
						"spanContext.spanID":          differs.AnyString(),
						"spanContext.traceFlags":      "01",
						"spanContext.traceID":         traceID,
						"spanKind":                    "client",
						"startTime":                   differs.AnyString(),
						"SampleRate":                  int64(1),
					},
					{
						"attributes.app.name":     "gobox",
						"attributes.service_name": "gobox",
						"attributes.app.version":  "testing",
						"attributes.trace":        "inner",
						"endTime":                 differs.AnyString(),
						"name":                    "inner",
						"parent.remote":           false,
						"parent.spanID":           rootID,
						"parent.traceFlags":       "01",
						"parent.traceID":          traceID,
						"spanContext.spanID":      middleID,
						"spanContext.traceFlags":  "01",
						"spanContext.traceID":     traceID,
						"spanKind":                "internal",
						"startTime":               differs.AnyString(),
						"SampleRate":              int64(1),
					},
					{
						"attributes.app.name":     "gobox",
						"attributes.service_name": "gobox",
						"attributes.app.version":  "testing",
						"endTime":                 differs.AnyString(),
						"name":                    "trace-rt",
						"parent.remote":           false,
						"parent.spanID":           "0000000000000000",
						"parent.traceFlags":       "00",
						"parent.traceID":          "00000000000000000000000000000000",
						"spanContext.spanID":      rootID,
						"spanContext.traceFlags":  "01",
						"spanContext.traceID":     traceID,
						"spanKind":                "internal",
						"startTime":               differs.AnyString(),
						"SampleRate":              int64(1),
					},
				}
			},
		},
		{
			name:                  "OtelServerHoneycombClient",
			initRoundTripperState: otelInitRoundTripperState,
			callRoundTripper:      honeycombCallRoundTripper,
			expectedGen: func(traceID, rootID, middleID differs.CustomComparer) []map[string]interface{} {
				return []map[string]interface{}{
					{
						"name":                                 "ep",
						"spanContext.traceID":                  "e19c8ec3bf261ba6ea13b9892a2564c3",
						"spanContext.spanID":                   differs.AnyString(),
						"spanContext.traceFlags":               "01",
						"parent.traceID":                       "e19c8ec3bf261ba6ea13b9892a2564c3",
						"parent.spanID":                        "11ca4c05edc139ae",
						"parent.traceFlags":                    "00",
						"parent.remote":                        true,
						"spanKind":                             "server",
						"startTime":                            differs.AnyString(),
						"endTime":                              differs.AnyString(),
						"attributes.net.transport":             "ip_tcp",
						"attributes.net.peer.ip":               "127.0.0.1",
						"attributes.net.peer.port":             "",
						"attributes.net.host.ip":               "127.0.0.1",
						"attributes.net.host.port":             "",
						"attributes.http.method":               "GET",
						"attributes.http.target":               "/myendpoint",
						"attributes.http.server_name":          "ep",
						"attributes.app.name":                  "gobox",
						"attributes.service_name":              "gobox",
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
						"SampleRate":                           int64(1),
					},
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.initRoundTripperState(t)
			defer state.Close()

			ev := tc.callRoundTripper(t, state)

			expected := tc.expectedGen(differs.CaptureString(), differs.CaptureString(), differs.CaptureString())
			if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
				t.Fatal("unexpected spans", diff)
			}
		})
	}
}

func otelInitRoundTripperState(t *testing.T) *roundtripperState {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
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

func otelCallRoundTripper(t *testing.T, state *roundtripperState) []map[string]interface{} {
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
	res.Body.Close()

	trace.End(inner)
	trace.End(ctx)

	return state.Ended()
}

func honeycombInitRoundTripperState(t *testing.T) *roundtripperState {
	recorder := tracetest.NewSpanRecorder()
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			prop, err := propagation.UnmarshalHoneycombTraceContext(r.Header.Get(propagation.TracePropagationHTTPHeader))
			if err != nil {
				t.Fatal("Unexpected error", err)
			}
			if prop == nil {
				t.Fatal("Expected honeycomb propagation")
			}
		}))

	return &roundtripperState{recorder, server}
}

func honeycombCallRoundTripper(t *testing.T, state *roundtripperState) []map[string]interface{} {
	hcHeader := "1;trace_id=e19c8ec3bf261ba6ea13b9892a2564c3,parent_id=11ca4c05edc139ae,context=bnVsbA=="

	client := http.Client{}

	req, err := http.NewRequest("GET", state.Server.URL+"/myendpoint", http.NoBody)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}

	req.Header.Add("X-Honeycomb-Trace", hcHeader)

	res, err := client.Do(req)
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	defer res.Body.Close()

	return state.Ended()
}

type roundtripperState struct {
	*tracetest.SpanRecorder
	*httptest.Server
}

func (rt *roundtripperState) Close() {
	rt.SpanRecorder.Close()
	rt.Server.Close()
}

func (suite) TestHeadersRoundtripper(t *testing.T) {
	defer app.SetName(app.Info().Name)
	app.SetName("gobox")

	state := (suite{}).initHeaderRoundTripperState(t)
	defer state.Close()

	ctx := trace.StartSpan(context.Background(), "trace-rt")
	inner := trace.StartSpan(ctx, "inner")
	trace.AddInfo(inner, log.F{"trace": "inner"})

	client := http.Client{Transport: &headerroundtripper{http.DefaultTransport}}
	req, err := http.NewRequest("GET", state.Server.URL+"/myendpoint", http.NoBody)
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

	traceID, rootID, middleID := differs.CaptureString(), differs.CaptureString(), differs.CaptureString()

	expected := []map[string]interface{}{
		{
			"name":                                 "ep",
			"spanContext.traceID":                  traceID,
			"spanContext.spanID":                   differs.AnyString(),
			"spanContext.traceFlags":               "01",
			"parent.traceID":                       traceID,
			"parent.spanID":                        middleID,
			"parent.traceFlags":                    "01",
			"parent.remote":                        true,
			"spanKind":                             "internal",
			"startTime":                            differs.AnyString(),
			"endTime":                              differs.AnyString(),
			"attributes.http.method":               "GET",
			"attributes.app.name":                  "gobox",
			"attributes.service_name":              "gobox",
			"attributes.app.version":               "testing",
			"attributes.duration":                  differs.AnyString(),
			"attributes.http.referer":              "",
			"attributes.http.request_id":           "",
			"attributes.http.status_code":          "200",
			"attributes.http.url_details.endpoint": "ep",
			"attributes.http.url_details.path":     "/myendpoint",
			"attributes.http.url_details.uri":      "/myendpoint",
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
			"SampleRate":                           int64(1),
		},
		{
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"attributes.trace":        "inner",
			"endTime":                 differs.AnyString(),
			"name":                    "inner",
			"parent.remote":           false,
			"parent.spanID":           rootID,
			"parent.traceFlags":       "01",
			"parent.traceID":          traceID,
			"spanContext.spanID":      middleID,
			"spanContext.traceFlags":  "01",
			"spanContext.traceID":     traceID,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"SampleRate":              int64(1),
		},
		{
			"attributes.app.name":     "gobox",
			"attributes.service_name": "gobox",
			"attributes.app.version":  "testing",
			"endTime":                 differs.AnyString(),
			"name":                    "trace-rt",
			"parent.remote":           false,
			"parent.spanID":           "0000000000000000",
			"parent.traceFlags":       "00",
			"parent.traceID":          "00000000000000000000000000000000",
			"spanContext.spanID":      rootID,
			"spanContext.traceFlags":  "01",
			"spanContext.traceID":     traceID,
			"spanKind":                "internal",
			"startTime":               differs.AnyString(),
			"SampleRate":              int64(1),
		},
	}

	ev := state.Ended()
	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected spans", diff)
	}
}

type headerroundtripper struct {
	old http.RoundTripper
}

func (rt headerroundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	for k, v := range trace.ToHeaders(r.Context()) {
		r.Header[k] = v
	}
	return rt.old.RoundTrip(r)
}

func (suite) initHeaderRoundTripperState(t *testing.T) *roundtripperState {
	recorder := tracetest.NewSpanRecorder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(trace.FromHeaders(r.Context(), r.Header, "ep"))
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

	return &roundtripperState{recorder, server}
}

func (suite) TestRoundripNoTracer(t *testing.T) {
	recorder := tracetest.NewSpanRecorderWithOptions(tracetest.Options{
		SamplePercent: 100.0,
		NoTracer:      true,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(trace.FromHeaders(r.Context(), r.Header, "ep"))
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

	state := &roundtripperState{recorder, server}
	defer state.Close()

	ctx := trace.StartSpan(context.Background(), "trace-rt")
	inner := trace.StartSpan(ctx, "inner")
	trace.AddInfo(inner, log.F{"trace": "inner"})

	client := http.Client{Transport: &headerroundtripper{http.DefaultTransport}}
	req, err := http.NewRequest("GET", state.Server.URL+"/myendpoint", http.NoBody)
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

	ev := otelCallRoundTripper(t, state)

	expected := []map[string]interface{}{}

	if diff := cmp.Diff(expected, ev, differs.Custom()); diff != "" {
		t.Fatal("unexpected spans", diff)
	}
}
