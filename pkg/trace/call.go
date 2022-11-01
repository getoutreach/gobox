// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides call functionality for tracing which enables passing additional context

package trace

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/internal/call"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

// nolint:nochecknoglobals // Why: we use this as a singleton.
var callTracker = &call.Tracker{}

// StartCall is used to start an internal call. For external calls please
// use StartExternalCall.
//
// This takes care of standard logging, metrics and tracing for "calls"
//
// Typical usage:
//
//	ctx = trace.StartCall(ctx, "sql", SQLEvent{Query: ...})
//	defer trace.EndCall(ctx)
//
//	return trace.SetCallStatus(ctx, sqlCall(...));
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
	log.Debug(ctx, fmt.Sprintf("calling: %s", cType), args...)

	ctx = StartSpan(callTracker.StartCall(ctx, cType, args), cType)
	AddInfo(ctx, args...)

	return ctx
}

// Deprecated: use AsGrpcCall call.Option instead
// SetTypeGRPC is meant to set the call type to GRPC on a context that has
// already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeGRPC(ctx context.Context) context.Context {
	callTracker.Info(ctx).Type = call.TypeGRPC
	return ctx
}

// Deprecated: use AsHttpCall call.Option instead
// SetTypeHTTP is meant to set the call type to HTTP on a context that has
// already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeHTTP(ctx context.Context) context.Context {
	callTracker.Info(ctx).Type = call.TypeHTTP
	return ctx
}

// Deprecated: use AsOutboundCall call.Option instead
// SetCallTypeOutbound is meant to set the call type to Outbound on a context that
// has already been initialized for tracing via StartCall or StartExternalCall.
func SetCallTypeOutbound(ctx context.Context) context.Context {
	callTracker.Info(ctx).Type = call.TypeOutbound
	return ctx
}

// SetCallStatus can be optionally called to set status of the call.
// When the error occurs on the current call, the error will be traced.
// When the error is nil, no-op from this function
func SetCallStatus(ctx context.Context, err error) error {
	if err != nil {
		info := callTracker.Info(ctx)
		if info != nil {
			info.SetStatus(ctx, err)
		}
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

	defer func(info *call.Info) {
		addDefaultTracerInfo(ctx, info)
		info.ReportLatency(ctx)

		if info.ErrInfo != nil {
			switch category := orerr.ExtractErrorStatusCategory(info.ErrInfo.RawError); category {
			case statuscodes.CategoryClientError:
				log.Warn(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
			case statuscodes.CategoryServerError:
				log.Error(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
			case statuscodes.CategoryOK: // just in case if someone will return non-nil error on success
				log.Info(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
			}
		} else {
			log.Info(ctx, info.Name, info, IDs(ctx), traceEventMarker{})
		}
	}(callTracker.Info(ctx))

	callTracker.EndCall(ctx)
}

// addArgsToCallInfo appends the log marshalers passed in as arguments
// to the callInfo struct
func addArgsToCallInfo(ctx context.Context, args ...log.Marshaler) bool {
	if callInfo := callTracker.Info(ctx); callInfo != nil {
		callInfo.AddArgs(ctx, args...)
		return true
	}
	return false
}

// IDs returns a log-compatible tracing scope (IDs) data built from the context suitable for logging.
func IDs(ctx context.Context) log.Marshaler {
	return traceInfo{ctx}
}

type traceInfo struct {
	context.Context
}

func (c traceInfo) MarshalLog(addField func(field string, value interface{})) {
	addField("honeycomb.trace_id", ID(c))
	addField("honeycomb.parent_id", parentID(c))
	addField("honeycomb.span_id", SpanID(c))
}

type traceEventMarker struct{}

func (traceEventMarker) MarshalLog(addField func(k string, v interface{})) {
	addField("event_name", "trace")
}
