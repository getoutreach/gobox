package trace

import (
	"context"
	"net/http"
	"strings"

	"github.com/honeycombio/beeline-go/propagation"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelpropagation "go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	// Header that enforces the tracing for particular request
	HeaderForceTracing = "X-Force-Trace"
	// Header used by OpenTelemetry to propagate traces
	OtelPropagationHeader = "Traceparent"
)

type roundtripper struct {
	old http.RoundTripper
}

func (rt roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	headers := map[string]string{
		strings.ToLower(OtelPropagationHeader): r.Header.Get(OtelPropagationHeader),
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

func (t *otelTracer) newTransport(old http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(&roundtripper{old})
}

type Handler struct {
	handler   http.Handler
	operation string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(OtelPropagationHeader) == "" {
		hcHeaderToW3CHeaders(r)
	}

	var startOptions oteltrace.SpanStartEventOption

	handler := otelhttp.NewHandler(h.handler, h.operation)

	force := r.Header.Get(HeaderForceTracing)
	if force != "" {
		startOptions = oteltrace.WithAttributes(attribute.String(fieldForceTrace, force))
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

func (t *otelTracer) toHeaders(ctx context.Context) map[string][]string {
	result := http.Header{}

	if !oteltrace.SpanFromContext(ctx).SpanContext().HasTraceID() {
		return result
	}

	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, otelpropagation.HeaderCarrier(result))

	// Honeycomb expects
	headers := map[string]string{
		strings.ToLower(OtelPropagationHeader): result.Get(OtelPropagationHeader),
	}

	_, prop, err := propagation.UnmarshalW3CTraceContext(ctx, headers)
	if err != nil {
		return result
	}

	result.Set(
		propagation.TracePropagationHTTPHeader,
		propagation.MarshalHoneycombTraceContext(prop))

	return result
}

func (t *otelTracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	header := http.Header(hdrs)

	force := header.Get(HeaderForceTracing)
	if force != "" {
		ctx = ForceTracing(ctx)
	}

	if header.Get(OtelPropagationHeader) == "" {
		prop, err := propagation.UnmarshalHoneycombTraceContext(header.Get(propagation.TracePropagationHTTPHeader))
		if err == nil {
			_, headers := propagation.MarshalW3CTraceContext(ctx, prop)
			for k, v := range headers {
				header.Set(k, v)
			}
		}
	}

	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, otelpropagation.HeaderCarrier(header))
	ctx = StartSpan(ctx, name)

	return ctx
}
