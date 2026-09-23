// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Float64HistogramMetric represents a float64 histogram metric.
type Float64HistogramMetric struct {
	emittableMetricBase
	metric metric.Float64Histogram
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
		reportFullEmitBuffer(ctx, m.name)
	}
}

// RegisterFloat64Histogram registers a synchronous float64 histogram instrument.
func RegisterFloat64Histogram(
	metricName MetricName,
	options ...metric.Float64HistogramOption,
) (*Float64HistogramMetric, error) {
	result := &Float64HistogramMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Float64Histogram(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
}

// Float64Histogram doesn't have an observable option
