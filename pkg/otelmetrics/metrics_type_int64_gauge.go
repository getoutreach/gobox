// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Int64GaugeMetric represents an int64 gauge metric.
type Int64GaugeMetric struct {
	emittableMetricBase
	metric metric.Int64Gauge
}

// Record records a value for the int64 gauge metric. It enqueues the recording work item to be processed asynchronously.
func (m *Int64GaugeMetric) Record(ctx context.Context, incr int64, options ...metric.RecordOption) {
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

// RegisterInt64Gauge registers a synchronous int64 gauge instrument.
func RegisterInt64Gauge(
	metricName MetricName,
	options ...metric.Int64GaugeOption,
) (*Int64GaugeMetric, error) {
	result := &Int64GaugeMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Int64Gauge(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
}

// Int64ObservableGaugeMetric represents an int64 observable gauge metric.
type Int64ObservableGaugeMetric struct {
	observableMetricBase
	metric metric.Int64ObservableGauge
}

// Record records a value for the int64 observable gauge metric. It invokes the observation immediately.
func (m *Int64ObservableGaugeMetric) Record(o Observer, incr int64, options ...metric.ObserveOption) {
	o.ObserveInt64(m.metric, incr, options...)
}

// RegisterInt64ObservableGauge registers a int64 observable gauge instrument within the observable group.
func (g *ObservableGroup) RegisterInt64ObservableGauge(
	metricName MetricName,
	options ...metric.Int64ObservableGaugeOption,
) *Int64ObservableGaugeMetric {
	result := &Int64ObservableGaugeMetric{observableMetricBase: observableMetricBase{name: metricName}}

	registrationFn := func(meter metric.Meter) error {
		m, err := meter.Int64ObservableGauge(string(metricName), options...)
		if err != nil {
			return err
		}
		result.metric = m
		g.observables = append(g.observables, m)
		return nil
	}
	g.pending = append(g.pending, registrationFn)

	return result
}
