package trace

import (
	"context"
	"net/http"

	"github.com/honeycombio/beeline-go/propagation"
	"github.com/honeycombio/beeline-go/trace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	// Header that enforces the tracing for particular request
	HeaderForceTracing = "X-Force-Trace"
)

type roundtripper struct {
	old http.RoundTripper
}

func (rt roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	for k, v := range ToHeaders(r.Context()) {
		r.Header[k] = v
	}
	return rt.old.RoundTrip(r)
}

func (t *honeycombTracer) newTransport(old http.RoundTripper) http.RoundTripper {
	return &roundtripper{old}
}

// fromHeaders fetches trace info from a headers map
func (t *honeycombTracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	header := http.Header(hdrs)

	if t.Honeycomb.Enabled {
		//nolint:errcheck // Why: we don't report errors in our current interface
		prop, _ := propagation.UnmarshalHoneycombTraceContext(header.Get(propagation.TracePropagationHTTPHeader))
		ctx = t.startHoneycombTrace(ctx, name, prop)
	}

	if _, exists := header[HeaderForceTracing]; exists {
		ctx = ForceTracing(ctx)
	}
	return ctx
}

// toHeaders writes the current trace context into a headers map
func (t *honeycombTracer) toHeaders(ctx context.Context) map[string][]string {
	result := http.Header{}

	if span := trace.GetSpanFromContext(ctx); span != nil {
		result.Set(propagation.TracePropagationHTTPHeader, span.SerializeHeaders())
	}

	return result
}

func (t *otelTracer) newTransport(old http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(old)
}

// Deprecated: will be removed in favor of automatic instrumentation
// fromHeaders fetches trace info from a headers map
func (t *otelTracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	return ctx
}

// Deprecated: will be removed in favor of automatic instrumentation
// toHeaders writes the current trace context into a headers map
func (t *otelTracer) toHeaders(ctx context.Context) map[string][]string {
	return http.Header{}
}
