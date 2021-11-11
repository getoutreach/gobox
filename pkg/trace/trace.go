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
//      r = r.WithContext(trace.ContextFromHTTP(r, "my endpoint"))
//      defer trace.End(r.Context()
//      ... do actual request handling ...
//
// Non-HTTP servers should wrap their request handling like so:
//
//      ctx = trace.StartTrace(ctx, "my endpoint")
//      defer trace.End(ctx)
//      ... do actual request handling ...
//
//
// Clients
//
// Propagating trace headers to HTTP clients or non-HTTP clients is
// not yet implemented here.  Please see ETC-190.
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
	"net/http"

	"github.com/getoutreach/gobox/pkg/log"
)

// nolint:gochecknoglobals
var defaultTracer = &tracer{}

// Deprecated: Use InitTracer() instead.
// StartTracing starts the tracing infrastructure.
//
// This should be called at the start of the application.
func StartTracing(serviceName string) error {
	return defaultTracer.startTracing(serviceName)
}

// InitTracer starts all tracing infrastructure.
//
// This needs to be called before sending any traces
// otherwise they will not be published.
func InitTracer(ctx context.Context, serviceName string) error {
	return defaultTracer.initTracer(ctx, serviceName)
}

// Deprecated: Use CloseTracer() instead.
// EndTracing stops the tracing infrastructure.
//
// This should be called at the exit of the application.
func EndTracing() {
	defaultTracer.endTracing()
}

// CloseTracer stops all tracing and sends any queued traces.
//
// This should be called when an application is exiting, or
// when you want to terminate the tracer.
func CloseTracer(ctx context.Context) {
	defaultTracer.closeTracer(ctx)
}

// ContextFromHTTP starts a new trace from an incoming http request.
//
// Use trace.End to end this.
func ContextFromHTTP(r *http.Request, name string) context.Context {
	return FromHeaders(r.Context(), r.Header, name)
}

// FromHeaders fetches trace info from a headers map
func FromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	return defaultTracer.fromHeaders(ctx, hdrs, name)
}

// FromHeadersAsync fetches trace info from a headers map and kicks off an async trace.
func FromHeadersAsync(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	return StartSpanAsync(defaultTracer.fromHeaders(ctx, hdrs, name), name)
}

// ToHeaders writes the current trace context into a headers map
func ToHeaders(ctx context.Context) map[string][]string {
	return defaultTracer.toHeaders(ctx)
}

// StartTrace starts a new root span/trace.
//
// Use trace.End to end this.
func StartTrace(ctx context.Context, name string) context.Context {
	return defaultTracer.startTrace(ctx, name)
}

// StartSpan starts a new span.
//
// Use trace.End to end this.
func StartSpan(ctx context.Context, name string, args ...log.Marshaler) context.Context {
	newCtx := defaultTracer.startSpan(ctx, name)
	addDefaultTracerInfo(newCtx, args...)
	return newCtx
}

// StartSpanAsync starts a new async span.
//
// An async span does not have to complete before the parent span completes.
//
// Use trace.End to end this.
func StartSpanAsync(ctx context.Context, name string, args ...log.Marshaler) context.Context {
	newCtx := defaultTracer.startSpanAsync(ctx, name)
	addDefaultTracerInfo(newCtx, args...)
	return newCtx
}

// End ends a span (or a trace started via StartTrace or ContextFromHTTP).
func End(ctx context.Context) {
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
	return defaultTracer.id(ctx)
}

// spanID returns the root tracing spanID for use when it is needed to correlate the logs belonging to same flow.
// The spanID returned will be the honeycomb trace spanID (if honeycomb is enabled) or an empty string if neither are enabled
func spanID(ctx context.Context) string {
	return defaultTracer.spanID(ctx)
}

// parentID returns the tracing parentID for use when it is needed to correlate the logs belonging to same flow.
// The parentID returned will be the honeycomb trace parentID (if honeycomb is enabled) or an empty string if neither are enabled
func parentID(ctx context.Context) string {
	return defaultTracer.parentID(ctx)
}

// ForceTracing will enforce tracing for processing started with returned context
// and all downstream services that will be invoken on the way.
func ForceTracing(ctx context.Context) context.Context {
	return forceTracing(ctx)
}

// ForceSampleRate will force a desired sample rate for the given trace and all children
// of said trace. The sample rate in practice will be 1/<rate>.
//
// For example, if you invoked:
//	ctx = trace.ForceSampleRate(ctx, 1000)
//
// The trace spawned from that and all of it's children would be sampled at a rate of
// 1/1000, or 1/10 of a percent (.1%).
func ForceSampleRate(ctx context.Context, rate uint) context.Context {
	return sampleAt(ctx, rate)
}

// AddSpanInfo updates the current span with the provided fields.
//
// It does nothing if there isn't a current span.
func AddSpanInfo(ctx context.Context, args ...log.Marshaler) {
	addDefaultTracerInfo(ctx, args...)
}

func addDefaultTracerInfo(ctx context.Context, args ...log.Marshaler) {
	defaultTracer.addInfo(ctx, args...)
}
