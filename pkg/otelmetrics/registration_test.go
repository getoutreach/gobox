// Copyright 2026 Outreach Corporation. All Rights Reserved.

package otelmetrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// withTestMeter points singletonMetricsOutput's meter at meter for the duration of the test.
// Tests using this helper must not run in parallel with each other.
func withTestMeter(t *testing.T, meter metric.Meter) {
	t.Helper()

	singletonMetricsOutput := SingletonMetricsOutput()
	singletonMetricsOutput.m.Lock()
	prev := singletonMetricsOutput.meter
	singletonMetricsOutput.meter = meter
	singletonMetricsOutput.m.Unlock()

	t.Cleanup(func() {
		singletonMetricsOutput.m.Lock()
		singletonMetricsOutput.meter = prev
		singletonMetricsOutput.m.Unlock()
	})
}

// TestMetricsOutput_ActivateRunsDeferredRegistrations verifies that deferred registrations are executed correctly after activation.
func TestMetricsOutput_ActivateRunsDeferredRegistrations(t *testing.T) {
	var out MetricsOutput

	type wired struct {
		instrument string
	}
	result := &wired{}

	err := out.Register("test/deferred", func(meter metric.Meter) error {
		result.instrument = "created"
		return nil
	})
	require.NoError(t, err)

	// Not yet activated: the registration must not have run early.
	assert.Empty(t, result.instrument)

	meter := sdkmetric.NewMeterProvider().Meter("test")
	require.NoError(t, out.activate(meter, make(chan emitWorkItem, 1)))

	// activate() ran the deferred closure; the SAME struct the caller holds
	// must reflect it - not a stale copy.
	assert.Equal(t, "created", result.instrument)
}

// TestMetricsOutput_RegisterAfterActivateRunsImmediately covers the "late registration" path.
func TestMetricsOutput_RegisterAfterActivateRunsImmediately(t *testing.T) {
	var out MetricsOutput

	meter := sdkmetric.NewMeterProvider().Meter("test")
	require.NoError(t, out.activate(meter, make(chan emitWorkItem, 1)))

	ran := false
	err := out.Register("test/late", func(meter metric.Meter) error {
		ran = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, ran)
}

// TestMetricsOutput_RegisterRejectsDuplicateNames guards the "must be unique within the binary" contract on MetricName.
func TestMetricsOutput_RegisterRejectsDuplicateNames(t *testing.T) {
	var out MetricsOutput

	noop := func(metric.Meter) error { return nil }
	require.NoError(t, out.Register("dup", noop))

	err := out.Register("dup", noop)
	assert.Error(t, err)
}

// TestMetricsOutput_ActivateFailurePropagatesButLeavesQueueInactive ensures that if activation fails, the queue remains inactive.
func TestMetricsOutput_ActivateFailurePropagatesButLeavesQueueInactive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var out MetricsOutput

	boom := errors.New("boom")
	require.NoError(t, out.Register("ok", func(metric.Meter) error { return nil }))
	require.NoError(t, out.Register("failing", func(metric.Meter) error { return boom }))

	meter := sdkmetric.NewMeterProvider().Meter("test")
	err := out.activate(meter, make(chan emitWorkItem, 1))
	require.Error(t, err)

	// A partially-failed activation must not leave a live queue behind: a
	// worker pool was never started for it, so anything enqueued would sit
	// there forever. trySend must report the queue as inactive.
	assert.False(t, out.trySend(ctx, emitWorkItem{emit: func(context.Context) {}}, "a"))
}

// TestMetricsOutput_RegisterRejectsDuplicateNameAfterMeterIsActive checks that double-registration guard applies even after activation.
func TestMetricsOutput_RegisterRejectsDuplicateNameAfterMeterIsActive(t *testing.T) {
	var out MetricsOutput

	meter := sdkmetric.NewMeterProvider().Meter("test")
	require.NoError(t, out.activate(meter, make(chan emitWorkItem, 1)))

	require.NoError(t, out.Register("late-dup", func(metric.Meter) error { return nil }))

	err := out.Register("late-dup", func(metric.Meter) error { return nil })
	assert.Error(t, err)
}

// TestMetricsOutput_LateRegistrationFailureCanBeRetried ensures that a failed late registration can be retried.
func TestMetricsOutput_LateRegistrationFailureCanBeRetried(t *testing.T) {
	var out MetricsOutput

	meter := sdkmetric.NewMeterProvider().Meter("test")
	require.NoError(t, out.activate(meter, make(chan emitWorkItem, 1)))

	boom := errors.New("boom")
	err := out.Register("retryable", func(metric.Meter) error { return boom })
	require.ErrorIs(t, err, boom)

	ran := false
	err = out.Register("retryable", func(metric.Meter) error {
		ran = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, ran, "retrying with the same name must not be rejected as a duplicate")
}

// TestObservableGroup_ReportsEveryRegisteredInstrument ensures that all instruments are registerd in order.
//
// ObservableGroup defers each instrument's creation into its own pending
// slice and only touches the singleton once, from Prepare - so the "meter
// still nil at registration time" path (what actually happens in
// production: instruments register from init() before Service.Run ever
// creates a meter, and MetricsOutput.activate() then runs every deferred
// registrar in Go map iteration order) is just as safe: Prepare's single map
// entry runs pending (in order) before RegisterCallback, regardless of where
// in that random iteration it lands.
func TestObservableGroup_ReportsEveryRegisteredInstrument(t *testing.T) {
	const n = 25

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })
	withTestMeter(t, provider.Meter("test"))

	g := &ObservableGroup{}
	counters := make([]*Int64ObservableCounterMetric, n)
	for i := 0; i < n; i++ {
		c := g.RegisterInt64ObservableCounter(MetricName(fmtCounterName(i)))
		counters[i] = c
	}

	collectFn := func(_ context.Context, o Observer) error {
		for i, c := range counters {
			c.Record(o, int64(i))
		}
		return nil
	}
	require.NoError(t, g.Prepare("test_ordered_group", collectFn))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	seen := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok || len(sum.DataPoints) == 0 {
				continue
			}
			seen[m.Name] = sum.DataPoints[0].Value
		}
	}

	require.Len(t, seen, n, "every instrument added to the group must have been created and reported")
	for i := 0; i < n; i++ {
		assert.Equal(t, int64(i), seen[fmtCounterName(i)])
	}
}

func fmtCounterName(i int) string {
	return "test_ordered_counter_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
}
