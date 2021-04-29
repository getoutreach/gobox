// Package metrics implements the outreach metrics API
//
// This consists of the Count and Latency functions
package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

type CallKind string

const (
	CallKindInternal CallKind = "internal"
	CallKindExternal CallKind = "external"
)

// nolint:gochecknoglobals
var callLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{

		Name: "call_request_seconds",
		Help: "The latency of the call",
		// use prometheus.DefBuckets which is
		// []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	},
	[]string{"app", "call", "status", "statuscode", "statuscategory", "kind"},
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

// ReportLatency reports the latency metric for this request
func ReportLatency(appName, callName string, latencySeconds float64, err error, options ...ReportLatencyOption) {
	opt := &ReportLatencyOptions{Kind: CallKindInternal}
	for _, f := range options {
		f(opt)
	}

	statusCode := statuscodes.OK
	if err != nil {
		// If it's not a StatusCodeWrapper, it will come back with UnknownError, which is fine.
		statusCode = orerr.ExtractErrorStatusCode(err)
	}

	// Legacy status str for older services.  Should be able to deprecate this fairly soon after the
	// new status code system rolls out.
	statusStr := "ok"
	if statusCode != statuscodes.OK {
		statusStr = "error"
	}

	callLatency.WithLabelValues(appName, callName, strings.ToLower(statusStr), statusCode.String(), statusCode.Category().String(), string(opt.Kind)).Observe(latencySeconds)
}
