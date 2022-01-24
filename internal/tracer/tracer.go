// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the honeycomb trace implementation.

// Package tracer implements the trace provider setup and a honeycomb
// provider.
package tracer

// Note: this package does not depend on pkg/log or pkg/trace.
import (
	"context"

	"github.com/getoutreach/gobox/internal/call"
)

// New creates a new tracer with the provided options.
//
// Use tracer.WithProvider to add providers to thet tracer.
func New(opts ...Option) *Tracer {
	t := &Tracer{}
	for _, opt := range opts {
		t = opt(t)
	}
	return t
}

// Option configures a tracer
type Option func(t *Tracer) *Tracer

// WithProvider adds the provider to the list of providers.
func WithProvider(p provider) Option {
	return func(t *Tracer) *Tracer {
		t.providers = append(t.providers, p)
		return t
	}
}

// SpanType indicates the type of a span.
type SpanType int

const (
	// SpanSync is the default span type.  It is meant for
	// pure tracing-only uses which are also synchronous.
	SpanSync SpanType = iota

	// SpanAsync is an async span.
	SpanAsync

	// SpanInHTTP is an incoming HTTP call span.
	SpanInHTTP

	// SpanInGRPC is an incoming gRPC call span.
	SpanInGRPC

	// SpanOut is a generic outbound call.
	SpanOut

	// SpanCall is a generic call type.  A further call may
	// refine the type.  This is only there to support the current
	// setup where the call type is done via a separate call.
	SpanCall
)

// IsCall returns true if the span type is an inbound or outboune call.
func (s SpanType) IsCall() bool {
	return s > SpanAsync
}

// IsInboundCall returns true if the span type is an inbound call.
func (s SpanType) InCallInbound() bool {
	return s == SpanInHTTP || s == SpanInGRPC
}

// IsOutboundCall returns true if the span type is an outbound call.
func (s SpanType) IsCallOutbound() bool {
	return s == SpanOut
}

// Info holds information about the current context.
type Info struct {
	// SpanID is the current span ID
	SpanID string

	// ParentID is the ID of the parent span, if one exists.
	ParentID string

	// TraceID is the overall trace ID.
	TraceID string

	// Call is the current call Info.
	Call *call.Info
}

func (info *Info) MarshalLog(addField func(field string, value interface{})) {
	addField("honeycomb.trace_id", info.TraceID)
	addField("honeycomb.parent_id", info.ParentID)
	addField("honeycomb.span_id", info.SpanID)
	if info.Call != nil {
		info.Call.MarshalLog(addField)
	}
}

// Tracer implements shared tracing functionality.
type Tracer struct {
	// providers is embedded here as it provides
	// nearly all the methods needed.
	providers
}

// Init changes the signature from the provider by removing the
// unnecessary tracer arg.
func (t *Tracer) Init(ctx context.Context, name string) error {
	return t.providers.Init(ctx, t, name)
}

// Info returns the current trace info.  It guarantees a non-nil result.
func (t *Tracer) Info(ctx context.Context) *Info {
	var result Info
	t.providers.CurrentInfo(ctx, &result)
	return &result
}

// Headers retuns the current trace state serialized into HTTP or gRPC headers.
func (t *Tracer) Headers(ctx context.Context) map[string][]string {
	result := map[string][]string{}
	t.providers.CurrentHeaders(ctx, result)
	return result
}
