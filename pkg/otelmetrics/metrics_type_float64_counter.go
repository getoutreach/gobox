// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Float64CounterMetric represents a float64 histogram metric.
type Float64CounterMetric struct {
	emittableMetricBase
	metric metric.Float64Counter
}

// Add records a value for the float64 counter metric. It enqueues the recording work item to be processed asynchronously.
func (m *Float64CounterMetric) Add(ctx context.Context, incr float64, options ...metric.AddOption) {
	// Prepare the work item to emit the metric from the worker pool.
	recordWorkItem := emitWorkItem{
		emit: func(ctx context.Context) {
			m.metric.Add(ctx, incr, options...)
		},
	}

	// Enqueue without blocking the caller and if not successful (queue full or service not started), report it.
	if !singletonMetricsOutput.trySend(recordWorkItem) {
		reportFullEmitBuffer(ctx, m.name)
	}
}

// RegisterFloat64Counter registers a synchronous float64 counter instrument.
func RegisterFloat64Counter(
	metricName MetricName,
	options ...metric.Float64CounterOption,
) (*Float64CounterMetric, error) {
	result := &Float64CounterMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Float64Counter(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
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

// RegisterFloat64ObservableCounter registers a float64 observable counter instrument within the observable group.
func (g *ObservableGroup) RegisterFloat64ObservableCounter(
	metricName MetricName,
	options ...metric.Float64ObservableCounterOption,
) *Float64ObservableCounterMetric {
	result := &Float64ObservableCounterMetric{observableMetricBase: observableMetricBase{name: metricName}}

	registrationFn := func(meter metric.Meter) error {
		m, err := meter.Float64ObservableCounter(string(metricName), options...)
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
