// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Int64CounterMetric represents a int64 counter metric.
type Int64CounterMetric struct {
	emittableMetricBase
	metric metric.Int64Counter
}

// Add records a value for the int64 counter metric. It enqueues the recording work item to be processed asynchronously.
func (m *Int64CounterMetric) Add(ctx context.Context, incr int64, options ...metric.AddOption) {
	// Prepare the work item to emit the metric from the worker pool.
	recordWorkItem := emitWorkItem{
		emit: func(ctx context.Context) {
			m.metric.Add(ctx, incr, options...)
		},
	}

	// Enqueue emitting the metric without blocking the caller.
	singletonMetricsOutput.trySend(ctx, recordWorkItem, m.name)
}

// RegisterInt64Counter registers a synchronous int64 counter instrument.
func RegisterInt64Counter(
	metricName MetricName,
	options ...metric.Int64CounterOption,
) (*Int64CounterMetric, error) {
	result := &Int64CounterMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Int64Counter(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
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

// RegisterInt64ObservableCounter registers an int64 observable counter instrument within the observable group.
func (g *ObservableGroup) RegisterInt64ObservableCounter(
	metricName MetricName,
	options ...metric.Int64ObservableCounterOption,
) *Int64ObservableCounterMetric {
	result := &Int64ObservableCounterMetric{observableMetricBase: observableMetricBase{name: metricName}}

	registrationFn := func(meter metric.Meter) error {
		m, err := meter.Int64ObservableCounter(string(metricName), options...)
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
