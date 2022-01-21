package call_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/differs"
	"github.com/prometheus/client_golang/prometheus"
	"gotest.tools/v3/assert"
)

func TestTracker_nestedCall(t *testing.T) {
	tracker := &call.Tracker{}
	ctx := context.Background()

	outer := tracker.StartCall(ctx, "outer", logf.Many{logf.F{"outer": true}})
	outerInfo := tracker.Info(outer)

	if assert.Check(t, outerInfo != nil) {
		assert.Assert(t, !outerInfo.Times.Started.IsZero())
		assert.Equal(t, outerInfo.Name, "outer")
	}

	inner := tracker.StartCall(outer, "inner", logf.Many{logf.F{"inner": true}})
	innerInfo := tracker.Info(inner)

	if assert.Check(t, innerInfo != nil) {
		assert.Assert(t, innerInfo != outerInfo)
		assert.Assert(t, !innerInfo.Times.Started.IsZero())
		assert.Equal(t, innerInfo.Name, "inner")
	}

	time.Sleep(time.Millisecond * 5)

	tracker.EndCall(inner)
	tracker.EndCall(outer)

	assert.Assert(t, tracker.Info(ctx) == nil)

	assert.DeepEqual(t, marshalInfo(innerInfo), logf.F{
		"inner":               true,
		"timing.dequeued_at":  differs.RFC3339NanoTime(),
		"timing.finished_at":  differs.RFC3339NanoTime(),
		"timing.scheduled_at": differs.RFC3339NanoTime(),
		"timing.service_time": differs.FloatRange(0, 0.1),
		"timing.total_time":   differs.FloatRange(0, 0.1),
		"timing.wait_time":    differs.FloatRange(0, 0.001),
	}, differs.Custom())

	assert.DeepEqual(t, marshalInfo(outerInfo), logf.F{
		"outer":               true,
		"timing.dequeued_at":  differs.RFC3339NanoTime(),
		"timing.finished_at":  differs.RFC3339NanoTime(),
		"timing.scheduled_at": differs.RFC3339NanoTime(),
		"timing.service_time": differs.FloatRange(0, 0.1),
		"timing.total_time":   differs.FloatRange(0, 0.1),
		"timing.wait_time":    differs.FloatRange(0, 0.001),
	}, differs.Custom())
}

func TestTracker_panic(t *testing.T) {
	tracker := &call.Tracker{}
	ctx := context.Background()

	outer := tracker.StartCall(ctx, "outer", logf.Many{logf.F{"outer": true}})
	outerInfo := tracker.Info(outer)

	defer func() {
		// The main validation happens within the panic.
		r := recover()
		assert.Equal(t, r, "testing panic")

		assert.DeepEqual(t, marshalInfo(outerInfo), logf.F{
			"error.error":         "testing panic",
			"error.kind":          "panic",
			"error.message":       "testing panic",
			"error.stack":         differs.AnyString(),
			"outer":               true,
			"timing.dequeued_at":  differs.RFC3339NanoTime(),
			"timing.finished_at":  differs.RFC3339NanoTime(),
			"timing.scheduled_at": differs.RFC3339NanoTime(),
			"timing.service_time": differs.FloatRange(0, 0.1),
			"timing.total_time":   differs.FloatRange(0, 0.1),
			"timing.wait_time":    differs.FloatRange(0, 0.001),
		}, differs.Custom())
	}()

	defer tracker.EndCall(outer)
	panic("testing panic")
}

func TestTracker_reportLatency(t *testing.T) {
	tracker := &call.Tracker{}
	ctx := context.Background()

	types := []call.Type{call.TypeHTTP, call.TypeGRPC, call.TypeOutbound}
	statuses := []string{"ok", "failed"}

	for _, callType := range types {
		for _, status := range statuses {
			name := string(callType) + "-" + status
			outer := tracker.StartCall(ctx, name, nil)
			outerInfo := tracker.Info(outer)
			outerInfo.Type = callType
			if status != "ok" {
				outerInfo.SetStatus(outer, errors.New(status))
				assert.Assert(t, outerInfo.ErrInfo.RawError != nil)
			}
			tracker.EndCall(outer)
			outerInfo.ReportLatency(ctx)
		}
	}

	metrics, err := prometheus.DefaultGatherer.Gather()
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

			for _, metricFamily := range metrics {
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

func marshalInfo(info *call.Info) logf.F {
	result := logf.F{}
	result.Set("", info)
	return result
}
