// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Float64UpDownMetric represents a float64 UpDown metric.
type Float64UpDownMetric struct {
	emittableMetricBase
	metric metric.Float64UpDownCounter
}

// Add adds a value for the float64 UpDown metric. It enqueues the recording work item to be processed asynchronously.
func (m *Float64UpDownMetric) Add(ctx context.Context, incr float64, options ...metric.AddOption) {
	// Prepare the work item to emit the metric from the worker pool.
	recordWorkItem := emitWorkItem{
		emit: func(ctx context.Context) {
			m.metric.Add(ctx, incr, options...)
		},
	}

	// Enqueue emitting the metric without blocking the caller.
	singletonMetricsOutput.trySend(ctx, recordWorkItem, m.name)
}

// RegisterFloat64UpDown registers a synchronous float64 UpDown instrument.
func RegisterFloat64UpDown(
	metricName MetricName,
	options ...metric.Float64UpDownCounterOption,
) (*Float64UpDownMetric, error) {
	result := &Float64UpDownMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Float64UpDownCounter(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
}

// Float64ObservableUpDownMetric represents a float64 observable UpDown metric.
type Float64ObservableUpDownMetric struct {
	observableMetricBase
	metric metric.Float64ObservableUpDownCounter
}

// Add adds a value for the float64 observable UpDown metric. It invokes the observation immediately.
func (m *Float64ObservableUpDownMetric) Add(o Observer, incr float64, options ...metric.ObserveOption) {
	o.ObserveFloat64(m.metric, incr, options...)
}

// RegisterFloat64ObservableUpDown registers a float64 observable UpDown instrument within the observable group.
func (g *ObservableGroup) RegisterFloat64ObservableUpDown(
	metricName MetricName,
	options ...metric.Float64ObservableUpDownCounterOption,
) *Float64ObservableUpDownMetric {
	result := &Float64ObservableUpDownMetric{observableMetricBase: observableMetricBase{name: metricName}}

	registrationFn := func(meter metric.Meter) error {
		m, err := meter.Float64ObservableUpDownCounter(string(metricName), options...)
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
