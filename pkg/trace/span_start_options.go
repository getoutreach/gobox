// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides various options for starting a span

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
	otelOption() trace.SpanStartOption
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
func WithLink(traceHeaders map[string][]string) Link {
	if defaultTracer == nil {
		return Link{}
	}

	// must have fresh context as an input to avoid any external pollution on the linked context
	return Link{linkContext: defaultTracer.contextFromHeaders(context.Background(), traceHeaders)}
}

// Link implements SpanStartOption
type Link struct {
	// linkContext is a dummy context to store linked trace/span ID headers
	linkContext context.Context
}

// TraceID returns the trace ID from the linked context or empty if such is unavailable
func (l Link) TraceID() string {
	return ID(l.linkContext)
}

// SpanID returns the span ID from the linked context or empty if such is unavailable
func (l Link) SpanID() string {
	return SpanID(l.linkContext)
}

// _ makes sure Link conforms with the SpanStartOption interface
var _ SpanStartOption = Link{}

// otelOption generates a link to be attached to the StartSpan call
func (l Link) otelOption() trace.SpanStartOption {
	if l.linkContext == nil {
		return nil
	}

	link := trace.LinkFromContext(l.linkContext)
	return trace.WithLinks(link)
}
