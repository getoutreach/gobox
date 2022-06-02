package trace

import (
	"context"
	"net/http"

	"github.com/honeycombio/beeline-go/propagation"
	"github.com/honeycombio/beeline-go/trace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
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

	if defaultTracer.isForce() {
		r.Header.Set(HeaderForceTracing, "true")
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
	handler   http.Handler
	operation string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Traceparent") == "" {
		hcHeaderToW3CHeaders(r)
	}

	var startOptions oteltrace.SpanStartEventOption

	handler := otelhttp.NewHandler(h.handler, h.operation)

	force := r.Header.Get(HeaderForceTracing)
	if force != "" {
		startOptions = oteltrace.WithAttributes(attribute.Bool(fieldForceTrace, force == "true"))
		handler = otelhttp.NewHandler(h.handler, h.operation, otelhttp.WithSpanOptions(startOptions))
	}

	handler.ServeHTTP(w, r)
}

func hcHeaderToW3CHeaders(r *http.Request) {
	prop, err := propagation.UnmarshalHoneycombTraceContext(r.Header.Get(propagation.TracePropagationHTTPHeader))
	if err != nil {
		return
	}

	_, headers := propagation.MarshalW3CTraceContext(r.Context(), prop)

	for k, v := range headers {
		r.Header.Set(k, v)
	}
}

func (t *otelTracer) newHandler(handler http.Handler, operation string) http.Handler {
	h := Handler{
		handler:   handler,
		operation: operation,
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
