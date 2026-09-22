// Copyright 2026 Outreach Corporation. All Rights Reserved.

package otelmetrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// withTestEmitChannel points the process-wide emit queue at a channel the
// test controls, restoring whatever was there before on cleanup.
// Float64HistogramMetric.Record is hardwired to the singleton MetricsOutput
// (it is meant to be a single, process-wide queue), so exercising its
// bounding behavior means reaching into that singleton directly; tests using
// this helper must not run in parallel with each other.
func withTestEmitChannel(t *testing.T, ch chan emitWorkItem) {
	t.Helper()

	singletonMetricsOutput := SingletonMetricsOutput()
	singletonMetricsOutput.m.Lock()
	prev := singletonMetricsOutput.emitChannel
	singletonMetricsOutput.emitChannel = ch
	singletonMetricsOutput.m.Unlock()

	t.Cleanup(func() {
		singletonMetricsOutput.m.Lock()
		singletonMetricsOutput.emitChannel = prev
		singletonMetricsOutput.m.Unlock()
	})
}

func newTestHistogram(t *testing.T) (*Float64HistogramMetric, *sdkmetric.ManualReader) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })

	hist, err := provider.Meter("test").Float64Histogram("test_histogram")
	require.NoError(t, err)

	return &Float64HistogramMetric{
		emittableMetricBase: emittableMetricBase{name: "test_histogram"},
		metric:              hist,
	}, reader
}

func collectHistogramSum(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.Histogram[float64] {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "metric %q is not a float64 histogram", name)
			return hist
		}
	}

	return metricdata.Histogram[float64]{}
}

// TestFloat64HistogramMetricRecordEnqueuesAndWorkerEmits is an end-to-end
// check of the fixed wiring: Record() enqueues a work item that, once a
// worker runs it, actually calls through to the real OTel instrument. This
// is the behavior that was silently broken before the fix - every Record()
// call would report a full buffer and drop the sample, because the shared
// queue was never wired onto the instrument's struct.
func TestFloat64HistogramMetricRecordEnqueuesAndWorkerEmits(t *testing.T) {
	m, reader := newTestHistogram(t)

	ch := make(chan emitWorkItem, 1)
	withTestEmitChannel(t, ch)

	m.Record(context.Background(), 2.5)

	select {
	case item := <-ch:
		item.emit(context.Background())
	default:
		t.Fatal("Record did not enqueue a work item")
	}

	hist := collectHistogramSum(t, reader, "test_histogram")
	require.Len(t, hist.DataPoints, 1)
	assert.Equal(t, uint64(1), hist.DataPoints[0].Count)
	assert.InDelta(t, 2.5, hist.DataPoints[0].Sum, 0.0001)
}

// TestFloat64HistogramMetricRecordDropsWithoutBlockingWhenQueueFull is the
// core "bound the number of outstanding emit events" behavior: once the
// bounded queue is full, Record() must drop the sample and return
// immediately rather than blocking the caller (the old design spawned an
// unbounded goroutine per emission instead; this replaces that with a fixed
// queue that sheds load once full).
func TestFloat64HistogramMetricRecordDropsWithoutBlockingWhenQueueFull(t *testing.T) {
	m, _ := newTestHistogram(t)

	ch := make(chan emitWorkItem, 1)
	ch <- emitWorkItem{emit: func(context.Context) {}} // pre-fill; nothing drains it
	withTestEmitChannel(t, ch)

	done := make(chan struct{})
	go func() {
		m.Record(context.Background(), 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Record blocked instead of dropping the sample once the queue was full")
	}

	assert.Len(t, ch, 1, "the dropped sample must not have been appended past capacity")
}

// TestFloat64HistogramMetricRecordDropsWhenQueueNotActive covers the
// service-disabled / not-yet-started case: with no active queue, Record()
// must no-op rather than panic or block.
func TestFloat64HistogramMetricRecordDropsWhenQueueNotActive(t *testing.T) {
	m, _ := newTestHistogram(t)
	withTestEmitChannel(t, nil)

	done := make(chan struct{})
	go func() {
		m.Record(context.Background(), 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Record blocked with no active queue")
	}
}

// TestMetricsOutputTrySendRespectsBufferCapacity checks the bound is exactly
// the channel's capacity: no more, no less.
func TestMetricsOutputTrySendRespectsBufferCapacity(t *testing.T) {
	var out MetricsOutput
	out.emitChannel = make(chan emitWorkItem, 2)

	item := emitWorkItem{emit: func(context.Context) {}}
	assert.True(t, out.trySend(item))
	assert.True(t, out.trySend(item))
	assert.False(t, out.trySend(item), "the third send must be dropped once the bounded queue is full")
	assert.Len(t, out.emitChannel, 2)
}

// TestMetricsOutputTrySendWithNoChannelReturnsFalse checks the zero-value
// (never-activated) case does not panic on a nil channel.
func TestMetricsOutputTrySendWithNoChannelReturnsFalse(t *testing.T) {
	var out MetricsOutput
	assert.False(t, out.trySend(emitWorkItem{emit: func(context.Context) {}}))
}

// TestServiceEmitRecoversPanics is a regression test for the panic recovery
// that the old fire-and-forget emitAsync helper provided (recovering so a
// bad metric callback could never crash the caller) but which the new
// worker-pool design dropped: a panicking Record callback used to only ever
// crash its own throwaway goroutine; now several instruments share a small
// pool of long-lived workers, so an unrecovered panic there would silently
// stop that worker (or crash the process) for everyone.
func TestServiceEmitRecoversPanics(t *testing.T) {
	s := &Service{}

	assert.NotPanics(t, func() {
		s.emit(context.Background(), emitWorkItem{
			emit: func(context.Context) { panic("boom") },
		})
	})
}
