package trace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/metrics"
)

// This constant block contains predefined callType base types.
const (
	// callTypeHTTP is a constant that denotes the call type being an HTTP
	// request.
	callTypeHTTP = "http"

	// callTypeGRPC is a constant that denotes the call type being a gRPC
	// request.
	callTypeGRPC = "grpc"

	// callTypeSQL is a constant that denotes the call type being an SQL
	// request.
	callTypeSQL = "sql"

	// callTypeRedis is a constant that denotes the call type being a redis
	// request.
	callTypeRedis = "redis"

	// callTypeGithub is a constant that denotes the call type being a request
	// to the Github API.
	callTypeGithub = "github"
)

// callType is a type that defines the name of a trace using a base and an
// optional suffix.
type callType struct {
	// base is the main part of the call type we use to identify what type of
	// tracing call this is. This is useful for doing things like triggering
	// a prometheus metric push based off of a trace.
	base string

	// Suffix gets concatenated with the base via a period (.) to form the fully
	// qualified trace name.
	suffix string
}

// String returns the string representation of the receiver callType. This
// function also implements the fmt.Stringer interface for callType.
func (c callType) String() string {
	if c.suffix != "" {
		return strings.Join([]string{c.base, c.suffix}, ".")
	}
	return c.base
}

// NewGRPCCallType returns a callType used for gRPC tracing. The service parameter
// here refers to the gRPC service name that the method lives under, not the name
// of the service (repository, application, etc.) calling this trace.
func NewGRPCCallType(service, method string) callType { //nolint:golint // Why: We don't want people directly using callType.
	return callType{
		base:   callTypeGRPC,
		suffix: strings.Join([]string{service, method}, "."),
	}
}

// NewHTTPCallType returns a callType used for HTTP tracing.
func NewHTTPCallType(method, endpoint string) callType { //nolint:golint // Why: We don't want people directly using callType.
	return callType{
		base:   callTypeHTTP,
		suffix: strings.Join([]string{method, endpoint}, "."),
	}
}

// NewSQLCallType returns a callType used for SQL tracing. The operation parameter
// is meant to be a one word operation, like select, update, insert, or delete.
func NewSQLCallType(operation, table string) callType { //nolint:golint // Why: We don't want people directly using callType.
	return callType{
		base:   callTypeSQL,
		suffix: strings.Join([]string{operation, table}, "."),
	}
}

// NewRedisCallType returns a callType used for Redis tracing. The operation
// parameter is meant to be either put or get in most all cases.
func NewRedisCallType(operation, key string) callType { //nolint:golint // Why: We don't want people directly using callType.
	return callType{
		base:   callTypeRedis,
		suffix: strings.Join([]string{operation, key}, "."),
	}
}

// NewGithubCallType returns a callType used for Github tracing.
func NewGithubCallType(action string) callType { //nolint:golint // Why: We don't want people directly using callType.
	return callType{
		base:   callTypeGithub,
		suffix: action,
	}
}

// NewCustomCallType allows the caller to define a custom callType by specifying the
// fully qualified name of the trace to result.
func NewCustomCallType(fullyQualifiedName string) callType { //nolint:golint // Why: We don't want people directly using callType.
	return callType{
		base: fullyQualifiedName,
	}
}

// reportLatency reports the latency for a call depending on the underlying
// type stored in the receiver.
func (c callType) reportLatency(info *callInfo) {
	var err error
	if info.ErrorInfo != nil {
		err = info.ErrorInfo.RawError
	}

	if info.kind == metrics.CallKindExternal {
		info.ReportOutboundLatency(err)
		return
	}

	switch c.base { //nolint:exhaustive //Why: we only report latency metrics in this case on HTTP/gRPC call types.
	case callTypeHTTP:
		info.ReportHTTPLatency(err)
	case callTypeGRPC:
		info.ReportGRPCLatency(err)
	}
}

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
func (c *callInfo) ReportHTTPLatency(err error) {
	metrics.ReportHTTPLatency(app.Info().Name, c.name.String(), c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
}

// ReportGRPCLatency is a thin wrapper around metrics.ReportGRPCLatency to report latency metrics
// for gRPC calls.
func (c *callInfo) ReportGRPCLatency(err error) {
	metrics.ReportGRPCLatency(app.Info().Name, c.name.String(), c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
}

// ReportOutboundLatency is a thin wrapper around metrics.ReportOutboundLatency to report latency
// metrics for outbound calls.
func (c *callInfo) ReportOutboundLatency(err error) {
	metrics.ReportOutboundLatency(app.Info().Name, c.name.String(), c.ServiceSeconds, err, metrics.WithCallKind(c.kind))
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
	info.name.reportLatency(info)

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
