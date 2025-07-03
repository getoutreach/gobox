// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Declares a generic interface for a tracer with full tracing capabilities

package trace

import (
	"context"
	"fmt"
	"net/http"

	"github.com/getoutreach/gobox/pkg/log"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanStatus is the status of a span
//
// [status]: https://opentelemetry.io/docs/concepts/signals/traces/#span-status
type SpanStatus int

// possible SpanStatus values
const (
	// SpanStatusUnset is the default status value.
	SpanStatusUnset SpanStatus = iota
	// SpanStatusOK means the span was explicitly marked error-free
	SpanStatusOK
	// SpanStatusError means the span has an error, e.g. a 500
	SpanStatusError
)

// MarshalLog implements logf.Marshaler.
func (s SpanStatus) MarshalLog(addField func(key string, v any)) {
	addField("span.status", s.String())
}

// String implements fmt.Stringer.
func (s SpanStatus) String() string {
	return s.toOtel().String()
}

// toOtel converts gobox status to otel status
func (s SpanStatus) toOtel() codes.Code {
	switch s {
	case SpanStatusError:
		return codes.Error
	case SpanStatusOK:
		return codes.Ok
	case SpanStatusUnset:
		return codes.Unset
	default:
		return codes.Unset
	}
}

// type assertions for SpanStatus
var (
	// it must implement log.Mashaler
	_ log.Marshaler = SpanStatus(0)
	// it must implement Stringer
	_ fmt.Stringer = SpanStatus(0)
)

// recordErrorConfig is the config RecordErrorOptions accrue into
type recordErrorConfig struct {
	log.Many
	includeStacktrace bool
}

// RecordErrorOption describes a configuration option for recording an error
type RecordErrorOption interface {
	addErrOpt(*recordErrorConfig)
}

// recordErrOptFunc is a functional RecordErrorOption
type errOptFunc func(*recordErrorConfig)

// addErrOpt implements RecordErrorOption.
func (e errOptFunc) addErrOpt(r *recordErrorConfig) {
	e(r)
}

// WithMarshalers adds the attributes to the error event.
func WithMarshalers(attrs ...log.Marshaler) RecordErrorOption {
	return errOptFunc(func(rec *recordErrorConfig) {
		rec.Many = append(rec.Many, attrs...)
	})
}

// WithStackTrace defines whether a stack trace should be sent with the error event
func WithStackTrace(enabled bool) RecordErrorOption {
	return errOptFunc(func(rec *recordErrorConfig) {
		rec.includeStacktrace = enabled
	})
}

type tracer interface {
	registerSpanProcessor(s sdktrace.SpanProcessor)
	// Deprecated: Use initTracer() instead.
	startTracing(serviceName string) error

	initTracer(ctx context.Context, serviceName string) error

	// Deprecated: Use closeTracer() instead.
	endTracing()

	closeTracer(ctx context.Context)

	// Deprecated: Use startSpan() instead.
	startTrace(ctx context.Context, name string) context.Context

	id(ctx context.Context) string

	startSpan(ctx context.Context, name string, opts ...SpanStartOption) context.Context

	// Deprecated: Use startSpan() instead.
	startSpanAsync(ctx context.Context, name string) context.Context

	end(ctx context.Context)

	addInfo(ctx context.Context, args ...log.Marshaler)

	// sendEvent sends an event in the span.
	// an [event] is a marker in the span that *something* happened, commonly an exception or
	// something of interest.
	//
	// [event]: https://opentelemetry.io/docs/concepts/signals/traces/#span-events
	sendEvent(ctx context.Context, name string, attributes ...log.Marshaler)

	// setStatus sets the [status] of the span. [status] is basically error, no
	// error, or not set
	//
	// Span status comes with a [description], which can be set with the third
	// argument, or left empty
	//
	//
	// [status]: https://opentelemetry.io/docs/concepts/signals/traces/#span-status
	// [description]: https://opentelemetry.io/docs/specs/otel/trace/api/#set-status
	setStatus(context.Context, SpanStatus, string)

	// recordError proxies to otel's record errors api: https://opentelemetry.io/docs/languages/go/instrumentation/#record-errors
	// It is highly recommended that you also set a span’s status to Error when
	// using RecordError, unless you do not wish to consider the span tracking a failed
	// operation as an error span. The RecordError function does not automatically set
	// a span status when called.
	recordError(context.Context, error, ...RecordErrorOption)

	spanID(ctx context.Context) string

	// Deprecated: Will be removed with full migration to OpenTelemetry.
	// OpenTelemetry automatically handle adding parentID to traces
	parentID(ctx context.Context) string

	newTransport(http.RoundTripper) http.RoundTripper

	newHandler(handler http.Handler, operation string, opts ...HandlerOption) http.Handler

	toHeaders(ctx context.Context) map[string][]string

	contextFromHeaders(ctx context.Context, hdrs map[string][]string) context.Context

	// fromHeaders is similar to contextFromHeaders + it starts a new span
	fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context
}
