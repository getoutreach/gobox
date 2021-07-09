package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/metrics"
)

// callType is an alias type for string. The reason this type is explicitly
// created as opposed to just using strings for call types is to force the
// caller into thinking about their decision before blindly setting a call
// type. This is important because of cardinality issues in relation to what
// is set as the call type.
//
// This type implements the fmt.Stringer interface.
type callType string

// CustomCallType returns a callType from a given string. Before using this
// function please check the constant block that immediately follows this
// function to see if a predefined call type already exists that your call
// could fit into.
func CustomCallType(in string) callType { //nolint:golint //Why: We want this type to be "annoying" to use to provoke thought as opposed to increasing cardinality.
	return callType(in)
}

// String returns the string representation of the receiver callType. This
// function also implements the fmt.Stringer interface for callType.
func (c callType) String() string {
	return string(c)
}

// This constant block contains predefined CallType types.
const (
	// CallTypeHTTP is a constant that denotes the call type being an HTTP
	// request. This constant is used in StartCall or StartExternalCall.
	CallTypeHTTP callType = "http"

	// CallTypeGRPC is a constant that denotes the call type being a gRPC
	// request. This constant is used in StartCall or StartExternalCall.
	CallTypeGRPC callType = "grpc"

	// CallTypeSQL is a constant that denotes the call type being an SQL
	// request. This constant is used in StartCall or StartExternalCall.
	CallTypeSQL callType = "sql"

	// CallTypeRedis is a constant that denotes the call type being a redis
	// request. This constant is used in StartCall or StartExternalCall.
	CallTypeRedis callType = "redis"
)

type callInfo struct {
	name callType
	kind metrics.CallKind
	args []log.Marshaler
	events.Times
	events.Durations
	*events.ErrorInfo
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
func (c *callInfo) ReportHTTPLatency() {
	metrics.ReportHTTPLatency(app.Info().Name, c.name.String(), c.ServiceSeconds, metrics.WithCallKind(c.kind))
}

// ReportGRPCLatency is a thin wrapper around metrics.ReportGRPCLatency to report latency metrics
// for gRPC calls.
func (c *callInfo) ReportGRPCLatency() {
	metrics.ReportGRPCLatency(app.Info().Name, c.name.String(), c.ServiceSeconds, metrics.WithCallKind(c.kind))
}

// ReportOutboundLatency is a thin wrapper around metrics.ReportOutboundLatency to report latency
// metrics for outbound calls.
func (c *callInfo) ReportOutboundLatency() {
	metrics.ReportOutboundLatency(app.Info().Name, c.name.String(), c.ServiceSeconds, metrics.WithCallKind(c.kind))
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
func StartCall(ctx context.Context, cType callType, args ...log.Marshaler) context.Context {
	info := &callInfo{
		name: cType,
		args: args,
		kind: metrics.CallKindInternal,
		Times: events.Times{
			Started: time.Now(),
		},
	}
	log.Debug(ctx, fmt.Sprintf("calling: %s", cType.String()), args...)

	ctx = StartSpan(context.WithValue(ctx, infoKey, info), cType.String())
	AddInfo(ctx, args...)

	return ctx
}

// StartExternalCall calls StartCall() and designates that this call is an
// external call (came to our service, not from it)
func StartExternalCall(ctx context.Context, cType callType, args ...log.Marshaler) context.Context {
	ctx = StartCall(ctx, cType, args...)
	ctx.Value(infoKey).(*callInfo).kind = metrics.CallKindExternal

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

	if info.kind == metrics.CallKindExternal {
		info.ReportOutboundLatency()
	} else {
		switch info.name { //nolint:exhaustive //Why: we only report latency metrics in this case on HTTP/gRPC call types.
		case CallTypeHTTP:
			info.ReportHTTPLatency()
		case CallTypeGRPC:
			info.ReportGRPCLatency()
		}
	}

	traceInfo := log.F{
		"honeycomb.trace_id": ID(ctx),
		"event_name":         "trace",
	}

	if info.ErrorInfo != nil {
		log.Error(ctx, info.name.String(), info, traceInfo)
	} else {
		log.Info(ctx, info.name.String(), info, traceInfo)
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
