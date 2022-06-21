// Package trace wraps standard tracing for outreach.
//
// This package wraps honeycomb tracing
//
// Trace Initialization
//
// Applications should call `trace.StartTracing(serviceName)` and
// `trace.StopTracing()` in their `main` like so:
//
//     func main() {
//          trace.StartTracing("example")
//          defer trace.StopTracing()
//
//          ... main app logic ...
//     }
//
//
// See https://github.com/getoutreach/gobox/blob/master/cmd/example/main.go.
//
//
// Servers and incoming requests
//
// The httpx/pkg/handlers package wraps the required trace
// header parsing logic and so applications that use
// `handlers.Endpoint` do not have to do anything special here.
//
// Custom http servers should wrap their request handling code like so:
//
// 		trace.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) *roundtripperState {
// 		  trace.StartSpan(r.Context(), "my endpoint")
// 		  defer trace.End(r.Context())
// 		  ... do actual request handling ...
//      }), "my endpoint")
//
// Non-HTTP servers should wrap their request handling like so:
//
//      ctx = trace.StartTrace(ctx, "my endpoint")
//      defer trace.End(ctx)
//      ... do actual request handling ...
//
//
// Clients should use a Client with the provided transport like so:
//
//      ctx = trace.StartTrace(ctx, "my call")
//      defer trace.End(ctx)
//      client := http.Client{Transport: trace.NewTransport(nil)}
//      ... do actual call using the new client ...
//
// Tracing calls
//
// Any interesting function (such as model fetches or redis fetches)
// should use the following pattern:
//
//     func MyInterestingRedisFunction(ctx context.Context, ...) error {
//         ctx = trace.StartCall(ctx, "redis", RedisInfo{...})
//         defer trace.EndCall(ctx)
//
//         .... actual work ...
//         trace.AddInfo(ctx, xyzInfo)
//
//         return trace.SetCallStatus(ctx, err)
//     }
//
//
// This automatically updates metrics ("call_request_secconds" is the
// counter with "redis" as the name label), writes to debug/error logs
// and also writes traces to our tracing infrastructure
//
// Trace calls can be nested.
//
// Creating spans
//
// Spans should rarely be needed but are available for when the metrics or
// default logging is not sufficient.
//
// Spans are automatically considered `children` of the current trace
// or span (based on the `context`).  The redis example above would
// look like so:
//
//     ctx = trace.StartTrace(ctx, "redis")
//     defer trace.End(ctx)
//     .... do actual redis call...
//
//
// Adding tags
//
// Tags can be added to the `current` span (or trace or call) by simply
// calling `trace.AddInfo`.   Note that this accepts the same types that
// logging accepts.  For instance, to record an error with redis:
//
//     result, err := redis.Call(....)
//     if err != nil {
//        // if you are using trace.Call, then do trace.SetCallStatus
//        // instead.
//        trace.AddInfo(ctx, events.NewErrorInfo(err))
//     }
//
//
package trace

import (
	"context"
	"os"

	"github.com/getoutreach/gobox/pkg/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// nolint:gochecknoglobals
var defaultTracer tracer

// Deprecated: Use InitTracer() instead.
// StartTracing starts the tracing infrastructure.
//
// This should be called at the start of the application.
func StartTracing(serviceName string) error {
	if err := setDefaultTracer(); err != nil {
		return err
	}

	return defaultTracer.startTracing(serviceName)
}

// InitTracer starts all tracing infrastructure.
//
// This needs to be called before sending any traces
// otherwise they will not be published.
func InitTracer(ctx context.Context, serviceName string) error {
	if err := setDefaultTracer(); err != nil {
		return err
	}
	return defaultTracer.initTracer(ctx, serviceName)
}

func RegisterSpanProcessor(s sdktrace.SpanProcessor) {
	defaultTracer.registerSpanProcessor(s)
}

func setDefaultTracer() error {
	config := Config{}
	err := config.Load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if config.Otel.Enabled {
		defaultTracer = &otelTracer{Config: config}
	}

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

	defaultTracer.closeTracer(ctx)
}

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

// Deprecated: You can just use StartSpan
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

// ID returns an ID for use with external services to propagate
// tracing context.  The ID returned will be the honeycomb trace ID
// (if honeycomb is enabled) or an empty string if neither are enabled.
func ID(ctx context.Context) string {
	if defaultTracer == nil {
		return ""
	}

	return defaultTracer.id(ctx)
}

// spanID returns the root tracing spanID for use when it is needed to correlate the logs belonging to same flow.
// The spanID returned will be the honeycomb trace spanID (if honeycomb is enabled) or an empty string if neither are enabled
func spanID(ctx context.Context) string {
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
	return defaultTracer.toHeaders(ctx)
}

// FromHeaders fetches trace info from a headers map
//
// Only use for GRPC. Prefer NewHandler for http calls.
func FromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	return defaultTracer.fromHeaders(ctx, hdrs, name)
}
