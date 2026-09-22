// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/getoutreach/gobox/pkg/log"
)

// emittableMetricBase is the base struct for all emittable metrics.
type emittableMetricBase struct {
	name MetricName
}

// observableMetricBase is the base struct for all observable metrics.
type observableMetricBase struct {
	name MetricName
}

// Float64HistogramMetric represents a float64 histogram metric.
type Float64HistogramMetric struct {
	emittableMetricBase
	metric metric.Float64Histogram
}

// lastReportedError keeps track of the last time an error was reported for a full emit buffer.
var lastReportedError atomic.Pointer[time.Time]

// ReportFullEmitBuffer logs a warning if the emit buffer for the given metric is full (at most once per minute).
func ReportFullEmitBuffer(ctx context.Context, name MetricName) {
	for range 5 {
		now := time.Now()
		lastReported := lastReportedError.Load()
		if lastReported != nil && now.Sub(*lastReported) <= time.Minute {
			return
		}

		if swapped := lastReportedError.CompareAndSwap(lastReported, &now); swapped {
			break
		}
	}

	log.Warn(ctx, "Full emit buffer for OTLP metrics", log.F{"metric_name": name})
}

// Record records a value for the float64 histogram metric. It enqueues the recording work item to be processed asynchronously.
func (m *Float64HistogramMetric) Record(ctx context.Context, incr float64, options ...metric.RecordOption) {
	// Prepare the work item to emit the metric from the worker pool.
	recordWorkItem := emitWorkItem{
		emit: func(ctx context.Context) {
			m.metric.Record(ctx, incr, options...)
		},
	}

	// Enqueue without blocking the caller and if not successful (queue full or service not started), report it.
	if !singletonMetricsOutput.trySend(recordWorkItem) {
		ReportFullEmitBuffer(ctx, m.name)
	}
}

// Observer is a wrapper around the OpenTelemetry metric.Observer interface.
type Observer struct {
	metric.Observer
}

// Float64ObservableCounterMetric represents a float64 observable counter metric.
type Float64ObservableCounterMetric struct {
	observableMetricBase
	metric metric.Float64ObservableCounter
}

// Record records a value for the float64 observable counter metric. It invokes the observation immediately.
func (m *Float64ObservableCounterMetric) Record(o Observer, incr float64, options ...metric.ObserveOption) {
	o.ObserveFloat64(m.metric, incr, options...)
}

// Int64ObservableCounterMetric represents an int64 observable counter metric.
type Int64ObservableCounterMetric struct {
	observableMetricBase
	metric metric.Int64ObservableCounter
}

// Record records a value for the int64 observable counter metric. It invokes the observation immediately.
func (m *Int64ObservableCounterMetric) Record(o Observer, incr int64, options ...metric.ObserveOption) {
	o.ObserveInt64(m.metric, incr, options...)
}

// Float64ObservableGaugeMetric represents a float64 observable gauge metric.
type Float64ObservableGaugeMetric struct {
	observableMetricBase
	metric metric.Float64ObservableGauge
}

// Record records a value for the float64 observable gauge metric. It invokes the observation immediately.
func (m *Float64ObservableGaugeMetric) Record(o Observer, incr float64, options ...metric.ObserveOption) {
	o.ObserveFloat64(m.metric, incr, options...)
}
