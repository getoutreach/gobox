// Package metrics implements the outreach metrics API
//
// This consists of the Count and Latency functions
package metrics

import (
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CallKind string

const (
	CallKindInternal CallKind = "internal"
	CallKindExternal CallKind = "external"
)

type ReportLatencyOptions struct {
	// Kind is the type of call that was made
	Kind CallKind
}

type ReportLatencyOption func(*ReportLatencyOptions)

// WithExternalCall reports that this call was an external call
func WithExternalCall() ReportLatencyOption {
	return func(opt *ReportLatencyOptions) {
		opt.Kind = CallKindExternal
	}
}

// WithCallKind sets the kind of call this was.
func WithCallKind(ck CallKind) ReportLatencyOption {
	return func(opt *ReportLatencyOptions) {
		opt.Kind = ck
	}
}

// httpCallLatency registers the http_request_handled metric for reporting latency of
// HTTP requests, in seconds.
var httpCallLatency = promauto.NewHistogramVec( // nolint:gochecknoglobals
	prometheus.HistogramOpts{
		Name:    "http_request_handled",
		Help:    "The latency of the HTTP request, in seconds",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"app", "call", "statuscode", "statuscategory", "kind"}, // Labels
)

// ReportHTTPLatency reports the http_request_handled metric for a request.
func ReportHTTPLatency(appName, callName string, latencySeconds float64, err error, options ...ReportLatencyOption) {
	opt := &ReportLatencyOptions{
		Kind: CallKindInternal, // Default to Internal, can be overridden with passed in options.
	}

	for _, f := range options {
		f(opt)
	}

	statusCode := statuscodes.OK
	if err != nil {
		// If it's not a StatusCodeWrapper, it will come back with UnknownError, which is fine.
		statusCode = orerr.ExtractErrorStatusCode(err)
	}

	httpCallLatency.WithLabelValues(appName, callName, statusCode.String(), statusCode.Category().String(), string(opt.Kind)).Observe(latencySeconds)
}

// grpcCallLatency registers the grpc_request_handled metric for reporting latency of
// gRPC requests, in seconds.
var grpcCallLatency = promauto.NewHistogramVec( // nolint:gochecknoglobals
	prometheus.HistogramOpts{
		Name:    "grpc_request_handled",
		Help:    "The latency of the gRPC request, in seconds",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"app", "call", "statuscode", "statuscategory", "kind"}, // Labels
)

// ReportGRPCLatency reports the grpc_request_handled metric for a request.
func ReportGRPCLatency(appName, callName string, latencySeconds float64, err error, options ...ReportLatencyOption) {
	opt := &ReportLatencyOptions{
		Kind: CallKindInternal, // Default to Internal, can be overridden with passed in options.
	}

	for _, f := range options {
		f(opt)
	}

	statusCode := statuscodes.OK
	if err != nil {
		// If it's not a StatusCodeWrapper, it will come back with UnknownError, which is fine.
		statusCode = orerr.ExtractErrorStatusCode(err)
	}

	grpcCallLatency.WithLabelValues(appName, callName, statusCode.String(), statusCode.Category().String(), string(opt.Kind)).Observe(latencySeconds)
}

// outboundCallLatency registers the outbound_call_seconds metric for reporting latency
// of outbound requests, in seconds.
var outboundCallLatency = promauto.NewHistogramVec( // nolint:gochecknoglobals
	prometheus.HistogramOpts{
		Name:    "outbound_call_seconds",
		Help:    "The latency of the outbound request, in seconds",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"app", "call", "statuscode", "statuscategory", "kind"}, // Labels
)

// ReportOutboundLatency reports the outbound_call_seconds metric for a request.
func ReportOutboundLatency(appName, callName string, latencySeconds float64, err error, options ...ReportLatencyOption) {
	opt := &ReportLatencyOptions{
		Kind: CallKindInternal, // Default to Internal, can be overridden with passed in options.
	}

	for _, f := range options {
		f(opt)
	}

	statusCode := statuscodes.OK
	if err != nil {
		// If it's not a StatusCodeWrapper, it will come back with UnknownError, which is fine.
		statusCode = orerr.ExtractErrorStatusCode(err)
	}

	outboundCallLatency.WithLabelValues(appName, callName, statusCode.String(), statusCode.Category().String(), string(opt.Kind)).Observe(latencySeconds)
}
