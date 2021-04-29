package trace

import (
	"context"
	"time"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/metrics"
)

// nolint:gochecknoglobals
var infoKey = &callInfo{}

// StartCall is used to start a call.
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
func StartCall(ctx context.Context, callType string, args ...log.Marshaler) context.Context {
	info := &callInfo{name: callType, args: args}
	info.Started = time.Now()
	info.kind = metrics.CallKindInternal
	log.Debug(ctx, "calling: "+callType, args...)
	ctx = StartSpan(context.WithValue(ctx, infoKey, info), callType)
	AddInfo(ctx, args...)
	return ctx
}

// StartExternalCall calls StartCall() and designates that this call is an
// external call (came to our service, not from it)
func StartExternalCall(ctx context.Context, callType string, args ...log.Marshaler) context.Context {
	ctx = StartCall(ctx, callType, args...)
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
	info.ReportLatency()
	traceInfo := log.F{"honeycomb.trace_id": ID(ctx), "event_name": "trace"}
	if info.ErrorInfo != nil {
		log.Error(ctx, info.name, info, traceInfo)
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

type callInfo struct {
	name string
	kind metrics.CallKind
	args []log.Marshaler
	events.Times
	events.Durations
	*events.ErrorInfo
}

func (c *callInfo) MarshalLog(addField func(key string, v interface{})) {
	c.Times.MarshalLog(addField)
	c.Durations.MarshalLog(addField)
	log.Many(c.args).MarshalLog(addField)
	if c.ErrorInfo != nil {
		c.ErrorInfo.MarshalLog(addField)
	}
}

func (c *callInfo) ReportLatency() {
	var err error
	if c.ErrorInfo != nil {
		err = c.ErrorInfo.RawError
	}

	metrics.ReportLatency(app.Info().Name, c.name, c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
}
