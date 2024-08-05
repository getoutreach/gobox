// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides propagation capabilities for traces

package trace

import (
	"context"
	"net/http"

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
	// Pass along the `X-Force-Trace` header if we received one.
	if isTracingForced(r.Context()) {
		r.Header.Set(HeaderForceTracing, "true")
	}

	return rt.old.RoundTrip(r)
}

func (t *otelTracer) newTransport(old http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(&roundtripper{old})
}

type Handler struct {
	handler          http.Handler
	operation        string
	publicEndpointFn func(*http.Request) bool
}

type HandlerOption func(*Handler)

func WithPublicEndpointFn(fn func(*http.Request) bool) HandlerOption {
	return func(h *Handler) {
		h.publicEndpointFn = fn
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var startOptions oteltrace.SpanStartEventOption

	handler := otelhttp.NewHandler(h.handler, h.operation)

	force := r.Header.Get(HeaderForceTracing)
	if force != "" {
		startOptions = oteltrace.WithAttributes(attribute.String(fieldForceTrace, force))
		handler = otelhttp.NewHandler(h.handler, h.operation,
			otelhttp.WithSpanOptions(startOptions),
			otelhttp.WithPublicEndpointFn(h.publicEndpointFn), // passing nil function is equivalent to "not configured"
		)
		r = r.WithContext(forceTracing(r.Context()))
	}

	handler.ServeHTTP(w, r)
}

func (t *otelTracer) newHandler(handler http.Handler, operation string, opts ...HandlerOption) http.Handler {
	h := Handler{
		handler:   handler,
		operation: operation,
	}
	for _, opt := range opts {
		opt(&h)
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

	return result
}

// contextFromHeaders loads the headers into a context as 'detached'. This method does not create a new span,
// thus do not end it and do not expose it directly to the callers (it is for FromHeaders and for WithLink usage only).
func (t *otelTracer) contextFromHeaders(ctx context.Context, hdrs map[string][]string) context.Context {
	header := http.Header(hdrs)

	force := header.Get(HeaderForceTracing)
	if force != "" {
		ctx = ForceTracing(ctx)
	}

	propagator := otel.GetTextMapPropagator()
	ctx = propagator.Extract(ctx, otelpropagation.HeaderCarrier(header))
	// this method does not create new span to allow WithLink-related methods use the context to generate otel's Link
	return ctx
}

func (t *otelTracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	ctx = t.contextFromHeaders(ctx, hdrs)
	ctx = StartSpan(ctx, name)
	return ctx
}
