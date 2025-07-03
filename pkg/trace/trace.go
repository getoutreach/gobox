// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides a generic wrapper for Go tracing libraries

// Package trace wraps standard tracing for outreach.
//
// # This package wraps honeycomb tracing
//
// # Trace Initialization
//
// Applications should call `trace.StartTracing(serviceName)` and
// `trace.StopTracing()` in their `main` like so:
//
//	func main() {
//	     trace.StartTracing("example")
//	     defer trace.StopTracing()
//
//	     ... main app logic ...
//	}
//
// See https://github.com/getoutreach/gobox/blob/master/cmd/example/main.go.
//
// # Servers and incoming requests
//
// The httpx/pkg/handlers package wraps the required trace
// header parsing logic and so applications that use
// `handlers.Endpoint` do not have to do anything special here.
//
// Custom http servers should wrap their request handling code like so:
//
//			trace.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) *roundtripperState {
//			  trace.StartSpan(r.Context(), "my endpoint")
//			  defer trace.End(r.Context())
//			  ... do actual request handling ...
//	     }), "my endpoint")
//
// Non-HTTP servers should wrap their request handling like so:
//
//	ctx = trace.StartSpan(ctx, "my endpoint")
//	defer trace.End(ctx)
//	... do actual request handling ...
//
// Clients should use a Client with the provided transport like so:
//
//	ctx = trace.StartSpan(ctx, "my call")
//	defer trace.End(ctx)
//	client := http.Client{Transport: trace.NewTransport(nil)}
//	... do actual call using the new client ...
//
// # Tracing calls
//
// Any interesting function (such as model fetches or redis fetches)
// should use the following pattern:
//
//	func MyInterestingRedisFunction(ctx context.Context, ...) error {
//	    ctx = trace.StartCall(ctx, "redis", RedisInfo{...})
//	    defer trace.EndCall(ctx)
//
//	    .... actual work ...
//	    trace.AddInfo(ctx, xyzInfo)
//
//	    return trace.SetCallStatus(ctx, err)
//	}
//
// This automatically updates metrics ("call_request_secconds" is the
// counter with "redis" as the name label), writes to debug/error logs
// and also writes traces to our tracing infrastructure
//
// Trace calls can be nested.
//
// # Creating spans
//
// Spans should rarely be needed but are available for when the metrics or
// default logging is not sufficient.
//
// Spans are automatically considered `children` of the current trace
// or span (based on the `context`).  The redis example above would
// look like so:
//
//	ctx = trace.StartSpan(ctx, "redis")
//	defer trace.End(ctx)
//	.... do actual redis call...
//
// # Adding tags
//
// Tags can be added to the `current` span (or trace or call) by simply
// calling `trace.AddInfo`.   Note that this accepts the same types that
// logging accepts.  For instance, to record an error with redis:
//
//	result, err := redis.Call(....)
//	if err != nil {
//	   // if you are using trace.Call, then do trace.SetCallStatus
//	   // instead.
//	   return trace.Error(ctx, err)
//	}
package trace

import (
	"context"
	"fmt"
	"os"

	"github.com/getoutreach/gobox/pkg/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// nolint:gochecknoglobals // Why: need to allow overriding
var defaultTracer tracer

// Deprecated: Use InitTracer() instead.
// StartTracing starts the tracing infrastructure.
//
// This should be called at the start of the application.
func StartTracing(serviceName string) error {
	return InitTracer(context.Background(), serviceName)
}

// InitTracer starts all tracing infrastructure.
//
// This needs to be called before sending any traces
// otherwise they will not be published.
func InitTracer(_ context.Context, serviceName string) error {
	if err := setDefaultTracer(serviceName); err != nil {
		return err
	}
	if defaultTracer == nil {
		return fmt.Errorf("no tracer configured, please check your 'trace.yaml' config")
	}

	return nil
}

func RegisterSpanProcessor(s sdktrace.SpanProcessor) {
	defaultTracer.registerSpanProcessor(s)
}

// setDefaultTracer sets the default tracer to use
func setDefaultTracer(serviceName string) error {
	config := &Config{}
	if err := config.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Ensure only one tracer is enabled
	if config.Otel.Enabled && config.LogFile.Enabled {
		return fmt.Errorf("more than one tracer enabled, please check your 'trace.yaml' config")
	}

	if config.Otel.Enabled {
		var err error
		defaultTracer, err = NewOtelTracer(context.Background(), serviceName, config)
		if err != nil {
			return fmt.Errorf("unable to start otel tracer: %w", err)
		}
	}

	if config.LogFile.Enabled {
		var err error
		// Note: NewLogFileTracer doesn't call tracer.initTracer to prevent otelTracer
		// from being initialized twice and overwriting itself.
		defaultTracer, err = NewLogFileTracer(context.Background(), serviceName, config)
		if err != nil {
			return fmt.Errorf("unable to start log file tracer: %w", err)
		}
	}

	logCallsByDefault = config.LogCallsByDefault

	return nil
}

// Deprecated: Use CloseTracer() instead.
// EndTracing stops the tracing infrastructure.
//
// This should be called at the exit of the application.
func EndTracing() {
	if defaultTracer == nil {
		return
	}

	defaultTracer.endTracing()
}

// CloseTracer stops all tracing and sends any queued traces.
//
// This should be called when an application is exiting, or
// when you want to terminate the tracer.
func CloseTracer(ctx context.Context) {
	if defaultTracer == nil {
		return
	}

	// In a nod to tests, reset the logCallsByDefault flag to its default
	// state.  This ensures the `trace.CloseTracer` call at the end of a
	// tracetest run cleans up any non-default values it created when it was
	// initialized.
	logCallsByDefault = false

	defaultTracer.closeTracer(ctx)
}

// Deprecated: use StartSpan() instead. It will start a trace automatically
// if the context does not contain one already.
// StartTrace starts a new root span/trace.
//
// Use trace.End to end this.
func StartTrace(ctx context.Context, name string) context.Context {
	if defaultTracer == nil {
		return ctx
	}

	return defaultTracer.startTrace(ctx, name)
}

// StartSpan starts a new span.
//
// Use trace.End to end this.
func StartSpan(ctx context.Context, name string, args ...log.Marshaler) context.Context {
	if defaultTracer == nil {
		return ctx
	}

	newCtx := defaultTracer.startSpan(ctx, name)
	addDefaultTracerInfo(newCtx, args...)
	return newCtx
}

// StartSpanWithOptions starts a new span, with extra start options (such as links to external spans).
//
// Use trace.End to end this.
func StartSpanWithOptions(ctx context.Context, name string, opts []SpanStartOption, args ...log.Marshaler) context.Context {
	if defaultTracer == nil {
		return ctx
	}

	newCtx := defaultTracer.startSpan(ctx, name, opts...)
	addDefaultTracerInfo(newCtx, args...)
	return newCtx
}

// Deprecated: use StartSpan() instead. It will handle async traces automatically.
// StartSpanAsync starts a new async span.
//
// An async span does not have to complete before the parent span completes.
//
// Use trace.End to end this.
func StartSpanAsync(ctx context.Context, name string, args ...log.Marshaler) context.Context {
	if defaultTracer == nil {
		return ctx
	}

	newCtx := defaultTracer.startSpanAsync(ctx, name)
	addDefaultTracerInfo(newCtx, args...)
	return newCtx
}

// SendEvent sends an event in the span.
//
// This is a wrapper around span.AddEvent that marshals the attributes to
// OpenTelemetry attributes.
func SendEvent(ctx context.Context, name string, attributes ...log.Marshaler) {
	if defaultTracer == nil {
		return
	}

	defaultTracer.sendEvent(ctx, name, attributes...)
}

// End ends a span (or a trace started via StartTrace).
func End(ctx context.Context) {
	if defaultTracer == nil {
		return
	}

	defaultTracer.end(ctx)
}

// AddInfo updates the current span with the provided fields.
// If a call exists, it updates the call info args with the
// passed in log marshalers
//
// This is not propagated to child spans automatically.
//
// It does nothing if there isn't a current span.
func AddInfo(ctx context.Context, args ...log.Marshaler) {
	if callExists := addArgsToCallInfo(ctx, args...); !callExists {
		addDefaultTracerInfo(ctx, args...)
	}
}

// Error is a convenience for attaching an error to a span.
//
// for the ultimate format, we conform to the specification:
// https://opentelemetry.io/docs/specs/otel/trace/exceptions/#recording-an-exception
func Error(ctx context.Context, err error, opts ...RecordErrorOption) error {
	// if the error is nil we no-op
	// if tracing is not enabled, no-op
	if err == nil || defaultTracer == nil {
		return nil
	}

	// if the error was also a log marshaler, respect that
	if m, ok := err.(log.Marshaler); ok {
		defaultTracer.addInfo(ctx, m)
	}

	defaultTracer.recordError(ctx, err, opts...)
	// https://opentelemetry.io/docs/languages/go/instrumentation/#record-errors
	// docs say you gotto set the status too, so we do so
	defaultTracer.setStatus(ctx, SpanStatusError, err.Error())

	return err
}

// ID returns an ID for use with external services to propagate
// tracing context.  The ID returned will be the honeycomb trace ID
// (if honeycomb is enabled) or an empty string if neither are enabled.
func ID(ctx context.Context) string {
	if defaultTracer == nil {
		return ""
	}

	return defaultTracer.id(ctx)
}

// SpanID returns the root tracing SpanID for use when it is needed to correlate the logs belonging to same flow.
// The SpanID returned will be the honeycomb trace SpanID (if honeycomb is enabled) or an empty string if neither are enabled
func SpanID(ctx context.Context) string {
	if defaultTracer == nil {
		return ""
	}

	return defaultTracer.spanID(ctx)
}

// parentID returns the tracing parentID for use when it is needed to correlate the logs belonging to same flow.
// The parentID returned will be the honeycomb trace parentID (if honeycomb is enabled) or an empty string if neither are enabled
func parentID(ctx context.Context) string {
	if defaultTracer == nil {
		return ""
	}
	return defaultTracer.parentID(ctx)
}

// ForceTracing will enforce tracing for processing started with returned context
// and all downstream services that will be invoken on the way.
func ForceTracing(ctx context.Context) context.Context {
	return forceTracing(ctx)
}

// AddSpanInfo updates the current span with the provided fields.
//
// It does nothing if there isn't a current span.
func AddSpanInfo(ctx context.Context, args ...log.Marshaler) {
	addDefaultTracerInfo(ctx, args...)
}

func addDefaultTracerInfo(ctx context.Context, args ...log.Marshaler) {
	if defaultTracer == nil {
		return
	}

	defaultTracer.addInfo(ctx, args...)
}

// ToHeaders writes the current trace context into a headers map
//
// Only use for GRPC. Prefer NewTransport for http calls.
func ToHeaders(ctx context.Context) map[string][]string {
	if defaultTracer == nil {
		return map[string][]string{}
	}

	return defaultTracer.toHeaders(ctx)
}

// FromHeaders fetches trace info from a headers map and starts a new Span on top of the extracted
// span context (which can be either local or remote). You must end this context with End.
//
// Only use for GRPC. Prefer NewHandler for http calls.
func FromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	if defaultTracer == nil {
		return ctx
	}
	return defaultTracer.fromHeaders(ctx, hdrs, name)
}
