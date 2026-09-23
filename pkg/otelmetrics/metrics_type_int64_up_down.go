// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Int64UpDownMetric represents a Int64 UpDown metric.
type Int64UpDownMetric struct {
	emittableMetricBase
	metric metric.Int64UpDownCounter
}

// Add adds a value for the Int64 UpDown metric. It enqueues the recording work item to be processed asynchronously.
func (m *Int64UpDownMetric) Add(ctx context.Context, incr int64, options ...metric.AddOption) {
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

// RegisterInt64UpDown registers a synchronous Int64 UpDown instrument.
func RegisterInt64UpDown(
	metricName MetricName,
	options ...metric.Int64UpDownCounterOption,
) (*Int64UpDownMetric, error) {
	result := &Int64UpDownMetric{
		emittableMetricBase: emittableMetricBase{name: metricName},
	}

	registrationFn := func(meter metric.Meter) (err error) {
		result.metric, err = meter.Int64UpDownCounter(string(metricName), options...)
		return err
	}

	return result, singletonMetricsOutput.Register(metricName, registrationFn)
}

// Int64ObservableUpDownMetric represents a Int64 observable UpDown metric.
type Int64ObservableUpDownMetric struct {
	observableMetricBase
	metric metric.Int64ObservableUpDownCounter
}

// Add adds a value for the Int64 observable UpDown metric. It invokes the observation immediately.
func (m *Int64ObservableUpDownMetric) Add(o Observer, incr int64, options ...metric.ObserveOption) {
	o.ObserveInt64(m.metric, incr, options...)
}

// RegisterInt64ObservableUpDown registers a Int64 observable UpDown instrument within the observable group.
func (g *ObservableGroup) RegisterInt64ObservableUpDown(
	metricName MetricName,
	options ...metric.Int64ObservableUpDownCounterOption,
) *Int64ObservableUpDownMetric {
	result := &Int64ObservableUpDownMetric{observableMetricBase: observableMetricBase{name: metricName}}

	registrationFn := func(meter metric.Meter) error {
		m, err := meter.Int64ObservableUpDownCounter(string(metricName), options...)
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
