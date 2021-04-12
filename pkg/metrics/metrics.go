// Package metrics implements the outreach metrics API
//
// This consists of the Count and Latency functions
package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// nolint:gochecknoglobals
var callLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{

		Name: "call_request_seconds",
		Help: "The latency of the call",
		// use prometheus.DefBuckets which is
		// []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	},
	[]string{"app", "call", "status"},
)

// ReportLatency reports the latency metric for this request
func ReportLatency(appName, callName string, latencySeconds float64, success bool) {
	status := "ok"
	if !success {
		status = "error"
	}
	callLatency.WithLabelValues(appName, callName, strings.ToLower(status)).Observe(latencySeconds)
}
