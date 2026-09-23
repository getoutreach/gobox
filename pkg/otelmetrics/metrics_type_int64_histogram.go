// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Int64HistogramMetric represents a int64 histogram metric.
type Int64HistogramMetric struct {
	emittableMetricBase
	metric metric.Int64Histogram
}

// Record records a value for the int64 histogram metric. It enqueues the recording work item to be processed asynchronously.
func (m *Int64HistogramMetric) Record(ctx context.Context, incr int64, options ...metric.RecordOption) {
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

// RegisterInt64Histogram registers a synchronous int64 histogram instrument.
func RegisterInt64Histogram(
	metricName MetricName,
	options ...metric.Int64HistogramOption,
) (*Int64HistogramMetric, error) {
	result := &Int64HistogramMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Int64Histogram(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
}

// Int64Histogram doesn't have an observable option
