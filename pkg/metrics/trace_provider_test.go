package metrics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"gotest.tools/v3/assert"
)

func TestTraceProvider(t *testing.T) {
	tracker := &call.Tracker{}
	ctx := context.Background()

	types := []call.Type{call.TypeHTTP, call.TypeGRPC, call.TypeOutbound}
	statuses := []string{"ok", "failed"}

	provider := &metrics.TraceProvider{}

	for _, callType := range types {
		for _, status := range statuses {
			name := string(callType) + "-" + status
			outer := tracker.StartCall(ctx, name, nil)
			outerInfo := tracker.Info(outer)
			provider.Start(outer, outerInfo)
			outerInfo.Type = callType
			if status != "ok" {
				outerInfo.SetStatus(outer, errors.New(status))
				assert.Assert(t, outerInfo.ErrInfo.RawError != nil)
			}
			tracker.EndCall(outer)
			provider.End(outer, outerInfo)
		}
	}

	metricsReported, err := prometheus.DefaultGatherer.Gather()
	assert.NilError(t, err)

	callFamily := map[call.Type]string{
		call.TypeHTTP:     "http_request_handled",
		call.TypeGRPC:     "grpc_request_handled",
		call.TypeOutbound: "outbound_call_seconds",
	}

	for _, callType := range types {
		for _, status := range statuses {
			name := string(callType) + "-" + status
			family := callFamily[callType]

			need := map[string]string{
				"call":           name,
				"kind":           "internal",
				"statuscategory": "CategoryOK",
				"statuscode":     "OK",
			}
			if status != "ok" {
				need["statuscategory"] = "CategoryServerError"
				need["statuscode"] = "UnknownError"
			}

			got := map[string]string{}

			for _, metricFamily := range metricsReported {
				if metricFamily.GetName() != family {
					continue
				}
				for _, metric := range metricFamily.Metric {
					got = map[string]string{}
					for _, labelPair := range metric.GetLabel() {
						key, value := labelPair.GetName(), labelPair.GetValue()
						if _, ok := need[key]; ok {
							got[key] = value
						}
					}

					if got["call"] == name {
						break
					}
				}
			}

			assert.DeepEqual(t, need, got)
		}
	}
}
