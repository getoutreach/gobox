package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/metrics"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

// This constant block contains predefined callType base types.
const (
	// callTypeHTTP is a constant that denotes the call type being an HTTP
	// request.
	callTypeHTTP = "http"

	// callTypeGRPC is a constant that denotes the call type being a gRPC
	// request.
	callTypeGRPC = "grpc"

	// callTypeOutbound is a constant that denotes the call type being an
	// outbound request.
	callTypeOutbound = "outbound"
)

type callInfo struct {
	name     string
	callType string
	kind     metrics.CallKind
	args     []log.Marshaler
	events.Times
	events.Durations
	*events.ErrorInfo
}

// reportLatency reports the latency for a call depending on the call type
// passed in *callInfo.
func (c *callInfo) reportLatency() {
	var err error
	if c.ErrorInfo != nil {
		err = c.ErrorInfo.RawError
	}

	switch c.callType { //nolint:exhaustive //Why: we only report latency metrics in this case on HTTP/gRPC call types.
	case callTypeHTTP:
		c.ReportHTTPLatency(err)
	case callTypeGRPC:
		c.ReportGRPCLatency(err)
	case callTypeOutbound:
		c.ReportOutboundLatency(err)
	}
}

// nolint:gochecknoglobals
var infoKey = &callInfo{}

func (c *callInfo) MarshalLog(addField func(key string, v interface{})) {
	c.Times.MarshalLog(addField)
	c.Durations.MarshalLog(addField)
	log.Many(c.args).MarshalLog(addField)
	if c.ErrorInfo != nil {
		c.ErrorInfo.MarshalLog(addField)
	}
}

// ReportHTTPLatency is a thin wrapper around metrics.ReportHTTPLatency to report latency metrics
// for HTTP calls.
func (c *callInfo) ReportHTTPLatency(err error) {
	metrics.ReportHTTPLatency(app.Info().Name, c.name, c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
}

// ReportGRPCLatency is a thin wrapper around metrics.ReportGRPCLatency to report latency metrics
// for gRPC calls.
func (c *callInfo) ReportGRPCLatency(err error) {
	metrics.ReportGRPCLatency(app.Info().Name, c.name, c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
}

// ReportOutboundLatency is a thin wrapper around metrics.ReportOutboundLatency to report latency
// metrics for outbound calls.
func (c *callInfo) ReportOutboundLatency(err error) {
	metrics.ReportOutboundLatency(app.Info().Name, c.name, c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
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
// the call failed (but no Info entry if the call succeeded).  Success
// or failure is determined by whether there was a SetCallStatus or
// not.  (Panics detected in EndCall are considered errors).
//
// StartCalls can be nested.
func StartCall(ctx context.Context, cType string, args ...log.Marshaler) context.Context {
	info := &callInfo{
		name: cType,
		args: args,
		kind: metrics.CallKindInternal,
		Times: events.Times{
			Started: time.Now(),
		},
	}
	log.Debug(ctx, fmt.Sprintf("calling: %s", cType), args...)

	ctx = StartSpan(context.WithValue(ctx, infoKey, info), cType)
	AddInfo(ctx, args...)

	return ctx
}

// SetTypeGRPC is meant to set the call type to GRPC on a context that has
// already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeGRPC(ctx context.Context) context.Context {
	ctx.Value(infoKey).(*callInfo).callType = callTypeGRPC
	return ctx
}

// SetTypeHTTP is meant to set the call type to HTTP on a context that has
// already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeHTTP(ctx context.Context) context.Context {
	ctx.Value(infoKey).(*callInfo).callType = callTypeHTTP
	return ctx
}

// SetCallTypeOutbound is meant to set the call type to Outbound on a context that
// has already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeOutbound(ctx context.Context) context.Context {
	ctx.Value(infoKey).(*callInfo).callType = callTypeOutbound
	return ctx
}

// SetCallStatus can be optionally called to set status of the call.
// When the error occurs on the current call, the error will be traced.
// When the error is nil, no-op from this function
func SetCallStatus(ctx context.Context, err error) error {
	if err != nil {
		info := ctx.Value(infoKey).(*callInfo)
		info.ErrorInfo = events.NewErrorInfo(err)
	}
	return err
}

// SetCallError is deprecated and will directly call into SetCallStatus for backward compatibility
func SetCallError(ctx context.Context, err error) error {
	return SetCallStatus(ctx, err)
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
	defer End(ctx)

	info := ctx.Value(infoKey).(*callInfo)
	info.Finished = time.Now()
	info.Durations = *info.Times.Durations()

	r := recover()
	if r != nil {
		info.ErrorInfo = events.NewErrorInfoFromPanic(r)
		// rethrow at the end of the function
		defer panic(r)
	}

	addDefaultTracerInfo(ctx, info)
	info.reportLatency()

	traceInfo := log.F{
		"honeycomb.trace_id": ID(ctx),
		"event_name":         "trace",
	}

	if info.ErrorInfo != nil {
		switch category := orerr.ExtractErrorStatusCategory(info.ErrorInfo.RawError); category {
		case statuscodes.CategoryClientError:
			log.Warn(ctx, info.name, info, traceInfo)
		case statuscodes.CategoryServerError:
			log.Error(ctx, info.name, info, traceInfo)
		case statuscodes.CategoryOK: // just in case if someone will return non-nil error on success
			log.Info(ctx, info.name, info, traceInfo)
		}
	} else {
		log.Info(ctx, info.name, info, traceInfo)
	}
}

// addArgsToCallInfo appends the log marshalers passed in as arguments
// to the callInfo struct
func addArgsToCallInfo(ctx context.Context, args ...log.Marshaler) bool {
	if infoKeyVal := ctx.Value(infoKey); infoKeyVal != nil {
		callInfo := infoKeyVal.(*callInfo)
		callInfo.args = append(callInfo.args, args...)
		return true
	}
	return false
}
