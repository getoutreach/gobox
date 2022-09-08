package trace

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// SpanStartOption is an interface for various options to be provided
// during span creation (StartSpan).
type SpanStartOption interface {
	// otelOption converts the current option to the native otel's SpanStartOption
	// it is intentionally unexported to block external implementations of SpanStartOption
	otelOption(t *otelTracer) trace.SpanStartOption
}

// WithLink links an external span to the current one. The link can only happen
// when trace span starts (they cannot be attached mid-span lifetime).
//
// The caller should not modify the argument traceHeaders map after calling WithLink.
//
// The method accepts trace headers as a map: assumption is current 'context' hosts the parent span
// and the linked contexts come from 'outside' by other means (e.g. clerk system event headers).
// In case you need to link trace with a span and you have direct access to that Span's context,
// you can use trace.ToHeaders to extract the same headers map.
func WithLink(traceHeaders map[string][]string) SpanStartOption {
	return linkOption{traceHeaders: traceHeaders}
}

// linkOption implements SpanStartOption
type linkOption struct {
	// traceHeaders to create span from
	traceHeaders map[string][]string
}

// otelOption generates a link to be attached to the StartSpan call
func (o linkOption) otelOption(t *otelTracer) trace.SpanStartOption {
	// must have fresh context as an input to avoid any external pollution of the span context
	linkCtx := t.contextFromHeaders(context.Background(), o.traceHeaders)
	link := trace.LinkFromContext(linkCtx)
	return trace.WithLinks(link)
}
