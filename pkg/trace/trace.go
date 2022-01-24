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

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/internal/tracelog"
	"github.com/getoutreach/gobox/internal/tracer"
	"github.com/getoutreach/gobox/pkg/log"
)

// nolint:gochecknoglobals // Why: we need this for the module level access.
var defaultTracer = tracelog.Tracer

// Deprecated: Use InitTracer() instead.
// StartTracing starts the tracing infrastructure.
//
// This should be called at the start of the application.
func StartTracing(serviceName string) error {
	return defaultTracer.Init(context.Background(), serviceName)
}

// InitTracer starts all tracing infrastructure.
//
// This needs to be called before sending any traces
// otherwise they will not be published.
func InitTracer(ctx context.Context, serviceName string) error {
	return defaultTracer.Init(ctx, serviceName)
}

// Deprecated: Use CloseTracer() instead.
// EndTracing stops the tracing infrastructure.
//
// This should be called at the exit of the application.
func EndTracing() {
	defaultTracer.Close(context.Background())
}

// CloseTracer stops all tracing and sends any queued traces.
//
// This should be called when an application is exiting, or
// when you want to terminate the tracer.
func CloseTracer(ctx context.Context) {
	defaultTracer.Close(ctx)
}

// ContextFromHTTP starts a new trace from an incoming http request.
//
// Use trace.End to end this.
func ContextFromHTTP(r *http.Request, name string) context.Context {
	return defaultTracer.StartTrace(r.Context(), name, r.Header)
}

// FromHeaders fetches trace info from a headers map
func FromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	return defaultTracer.StartTrace(ctx, name, hdrs)
}

// FromHeadersAsync fetches trace info from a headers map and kicks off an async trace.
func FromHeadersAsync(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	return StartSpanAsync(FromHeaders(ctx, hdrs, name), name)
}

// ToHeaders writes the current trace context into a headers map
func ToHeaders(ctx context.Context) map[string][]string {
	return defaultTracer.Headers(ctx)
}

// StartTrace starts a new root span/trace.
//
// Use trace.End to end this.
func StartTrace(ctx context.Context, name string) context.Context {
	return defaultTracer.StartTrace(ctx, name, nil)
}

// StartCall is used to start an internal call. For external calls please
// use StartExternalCall.
//
// This takes care of standard logging, metrics and tracing for "calls"
//
// Typical usage:
//
//     ctx = trace.StartCall(ctx, "sql", SQLEvent{Query: ...})
//     defer trace.EndCall(ctx)
//
//     return trace.SetCallStatus(ctx, sqlCall(...));
//
// The callType should be broad category (such as "sql", "redis" etc) as
// as these are used for metrics and cardinality issues come into play.
// Do note that commonly used call types are exported as constants in this
// package and should be used whenever possible. The most common call types
// are http (trace.CallTypeHTTP) and grpc (trace.CallTypeGRPC).
//
// Use the extra args to add stuff to traces and logs and these can
// have more information as needed (including actual queries for
// instance).
//
// The log includes a initial Debug entry and a final Error entry if
// the call failed (but no IDs entry if the call succeeded).  Success
// or failure is determined by whether there was a SetCallStatus or
// not.  (Panics detected in EndCall are considered errors).
//
// StartCalls can be nested.
func StartCall(ctx context.Context, cType string, args ...log.Marshaler) context.Context {
	return defaultTracer.StartSpan(ctx, cType, log.Many(args), tracer.SpanCall)
}

// EndCall calculates the duration of the call, writes to metrics,
// standard logs and closes the trace span.
//
// This call should always be right after the StartCall in a defer
// pattern.  See StartCall for the right pattern of usage.
//
// EndCall, when called within a defer, catches any panics and
// rethrows them.  Any panics are converted to errors and cause error
// logging to happen (as do any SetCallStatus calls)
func EndCall(ctx context.Context) {
	defaultTracer.EndSpan(ctx, tracer.SpanCall)
}

// SetTypeGRPC is meant to set the call type to GRPC on a context that has
// already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeGRPC(ctx context.Context) context.Context {
	defaultTracer.Info(ctx).Call.Type = call.TypeGRPC
	return ctx
}

// SetTypeHTTP is meant to set the call type to HTTP on a context that has
// already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeHTTP(ctx context.Context) context.Context {
	defaultTracer.Info(ctx).Call.Type = call.TypeHTTP
	return ctx
}

// SetCallTypeOutbound is meant to set the call type to Outbound on a context that
// has already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeOutbound(ctx context.Context) context.Context {
	defaultTracer.Info(ctx).Call.Type = call.TypeOutbound
	return ctx
}

// SetCallStatus can be optionally called to set status of the call.
// When the error occurs on the current call, the error will be traced.
// When the error is nil, no-op from this function
func SetCallStatus(ctx context.Context, err error) error {
	if err != nil {
		defaultTracer.SetSpanStatus(ctx, tracer.SpanCall, err)
	}
	return err
}

// SetCallError is deprecated and will directly call into SetCallStatus for backward compatibility
func SetCallError(ctx context.Context, err error) error {
	return SetCallStatus(ctx, err)
}

// StartSpan starts a new span.
//
// Use trace.End to end this.
func StartSpan(ctx context.Context, name string, args ...log.Marshaler) context.Context {
	return defaultTracer.StartSpan(ctx, name, log.Many(args), tracer.SpanSync)
}

// StartSpanAsync starts a new async span.
//
// An async span does not have to complete before the parent span completes.
//
// Use trace.End to end this.
func StartSpanAsync(ctx context.Context, name string, args ...log.Marshaler) context.Context {
	return defaultTracer.StartSpan(ctx, name, log.Many(args), tracer.SpanAsync)
}

// End ends a span (or a trace started via StartTrace or ContextFromHTTP).
func End(ctx context.Context) {
	info := defaultTracer.Info(ctx)

	if info.ParentID == info.SpanID {
		defaultTracer.EndTrace(ctx)
	} else {
		defaultTracer.EndSpan(ctx, tracer.SpanSync)
	}
}

// AddInfo updates the current span with the provided fields.
// If a call exists, it updates the call info args with the
// passed in log marshalers
//
// This is not propagated to child spans automatically.
//
// It does nothing if there isn't a current span.
func AddInfo(ctx context.Context, args ...log.Marshaler) {
	spanType := tracer.SpanSync
	if info := defaultTracer.Info(ctx); info.Call != nil {
		spanType = tracer.SpanCall
	}
	defaultTracer.AddSpanInfo(ctx, spanType, log.Many(args))
}

// ID returns an ID for use with external services to propagate
// tracing context.  The ID returned will be the honeycomb trace ID
// (if honeycomb is enabled) or an empty string if neither are enabled.
func ID(ctx context.Context) string {
	return defaultTracer.Info(ctx).TraceID
}

// IDs returns a log-compatible tracing scope (IDs) data built
// from the context suitable for logging.
func IDs(ctx context.Context) log.Marshaler {
	info := *(defaultTracer.Info(ctx))
	info.Call = nil
	return &info
}

// ForceTracing will enforce tracing for processing started with returned context
// and all downstream services that will be invoken on the way.
func ForceTracing(ctx context.Context) context.Context {
	return defaultTracer.ForceTrace(ctx)
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
	return defaultTracer.SetCurrentSampleRate(ctx, rate)
}

// AddSpanInfo updates the current span with the provided fields.
//
// It does nothing if there isn't a current span.
func AddSpanInfo(ctx context.Context, args ...log.Marshaler) {
	defaultTracer.AddSpanInfo(ctx, tracer.SpanSync, log.Many(args))
}

// SetTestPresendHook sets the honeycomb presend hook for testing
func SetTestPresendHook(hook func(map[string]interface{})) {
	defaultTracer.SetPresendHook(hook)
}
