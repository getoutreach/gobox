// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Float64GaugeMetric represents a float64 gauge metric.
type Float64GaugeMetric struct {
	emittableMetricBase
	metric metric.Float64Gauge
}

// Record records a value for the float64 gauge metric. It enqueues the recording work item to be processed asynchronously.
func (m *Float64GaugeMetric) Record(ctx context.Context, incr float64, options ...metric.RecordOption) {
	// Prepare the work item to emit the metric from the worker pool.
	recordWorkItem := emitWorkItem{
		emit: func(ctx context.Context) {
			m.metric.Record(ctx, incr, options...)
		},
	}

	// Enqueue emitting the metric without blocking the caller.
	singletonMetricsOutput.trySend(ctx, recordWorkItem, m.name)
}

// RegisterFloat64Gauge registers a synchronous float64 gauge instrument.
func RegisterFloat64Gauge(
	metricName MetricName,
	options ...metric.Float64GaugeOption,
) (*Float64GaugeMetric, error) {
	result := &Float64GaugeMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Float64Gauge(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
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

// RegisterFloat64ObservableGauge registers a float64 observable gauge instrument within the observable group.
func (g *ObservableGroup) RegisterFloat64ObservableGauge(
	metricName MetricName,
	options ...metric.Float64ObservableGaugeOption,
) *Float64ObservableGaugeMetric {
	result := &Float64ObservableGaugeMetric{observableMetricBase: observableMetricBase{name: metricName}}

	registrationFn := func(meter metric.Meter) error {
		m, err := meter.Float64ObservableGauge(string(metricName), options...)
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
