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
	headers := map[string]string{
		"traceparent": r.Header.Get("Traceparent"),
	}
	_, prop, err := propagation.UnmarshalW3CTraceContext(r.Context(), headers)
	if err != nil {
		return nil, err
	}

	hcHeaders := http.Header{}
	hcHeaders.Set(propagation.TracePropagationHTTPHeader, propagation.MarshalHoneycombTraceContext(prop))
	for k, v := range hcHeaders {
		r.Header[k] = v
	}

	return rt.old.RoundTrip(r)
}

func (t *honeycombTracer) newTransport(old http.RoundTripper) http.RoundTripper {
	return &roundtripper{old}
}

func (t *honeycombTracer) newHandler(_ http.Handler, operation string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(ContextFromHTTP(r, operation))
		defer End(r.Context())
	})
}

// fromHeaders fetches trace info from a headers map
func (t *honeycombTracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	header := http.Header(hdrs)
	//nolint:errcheck // Why: we don't report errors in our current interface
	prop, _ := propagation.UnmarshalHoneycombTraceContext(header.Get(propagation.TracePropagationHTTPHeader))
	ctx = t.startHoneycombTrace(ctx, name, prop)

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
	return otelhttp.NewTransport(&roundtripper{old})
}

type Handler struct {
	handler http.Handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Traceparent") != "" {
		h.handler.ServeHTTP(w, r)
		return
	}

	prop, err := propagation.UnmarshalHoneycombTraceContext(r.Header.Get(propagation.TracePropagationHTTPHeader))
	if err != nil {
		h.handler.ServeHTTP(w, r)
		return
	}

	_, headers := propagation.MarshalW3CTraceContext(r.Context(), prop)

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	h.handler.ServeHTTP(w, r)
}

func (t *otelTracer) newHandler(handler http.Handler, operation string) http.Handler {
	h := Handler{
		handler: otelhttp.NewHandler(handler, operation),
	}

	return &h
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
