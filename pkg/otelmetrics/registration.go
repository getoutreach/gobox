// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the registration of OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/metric"
)

// InstrumentRegistrar is called after the meter provider is initialized to actually register the instrument.
type InstrumentRegistrar func(meter metric.Meter) error

// MetricsOutput is the process-wide registry of deferred instrument registrations that metrics are emitted through.
type MetricsOutput struct {
	instrumentRegistrars map[MetricName]InstrumentRegistrar
	emitChannel          chan emitWorkItem
	meter                metric.Meter
	m                    sync.Mutex
}

// singletonMetricsOutput is the single, process-wide instance of MetricsOutput.
var singletonMetricsOutput MetricsOutput

// SingletonMetricsOutput returns the single, process-wide instance of MetricsOutput.
func SingletonMetricsOutput() *MetricsOutput {
	return &singletonMetricsOutput
}

// activate is called on metrics initialization and runs every deferred registrar against meter
// It is called once, by Service.Run, after the meter provider is initialized.
func (m *MetricsOutput) activate(meter metric.Meter, emitChannel chan emitWorkItem) error {
	m.m.Lock()
	defer m.m.Unlock()

	for name, registrar := range m.instrumentRegistrars {
		if registrar == nil {
			continue
		}

		if err := registrar(meter); err != nil {
			return fmt.Errorf("failed to activate metric %s: %w", name, err)
		}

		// Clear the registrar for this metric after successful activation.
		m.instrumentRegistrars[name] = nil
	}

	// Only make the emit queue live once every instrument has been created successfully.
	m.meter = meter
	m.emitChannel = emitChannel

	return nil
}

// Register stores registrationFn to run once the meter is available, or runs
// it immediately if the meter is already active (e.g. a late registration
// after the service has started). Each MetricName may only be registered
// once.
func (m *MetricsOutput) Register(metricName MetricName, registrationFn InstrumentRegistrar) error {
	m.m.Lock()
	defer m.m.Unlock()

	if m.instrumentRegistrars == nil {
		m.instrumentRegistrars = make(map[MetricName]InstrumentRegistrar)
	}

	if _, exists := m.instrumentRegistrars[metricName]; exists {
		return fmt.Errorf("metric %s is already registered", metricName)
	}

	// If the meter is already initialized, immediately register the metric.
	if m.meter != nil {
		if err := registrationFn(m.meter); err != nil {
			delete(m.instrumentRegistrars, metricName)
			return err
		}

		// Mark this metric as successfully registered even though the registrar is now cleared.
		m.instrumentRegistrars[metricName] = nil

		return nil
	}

	m.instrumentRegistrars[metricName] = registrationFn

	return nil
}

// trySend enqueues item onto the bounded emit queue without blocking and returns whether it was successfully enqueued.
func (m *MetricsOutput) trySend(item emitWorkItem) bool {
	m.m.Lock()
	ch := m.emitChannel
	m.m.Unlock()

	// If the emit channel is not yet active, the item cannot be enqueued.
	if ch == nil {
		return false
	}

	select {
	case ch <- item:
		return true
	default:
		return false
	}
}

// ObservableGroup collects a set of observable instruments that are reported together from a single OTel callback.
type ObservableGroup struct {
	groupName   string
	pending     []func(meter metric.Meter) error
	observables []metric.Observable
}

// Prepare finalizes the group once the meter is available and registers collectFn to be called at every collection time.
func (g *ObservableGroup) Prepare(groupName string, collectFn func(context.Context, Observer) error) error {
	g.groupName = groupName
	registerFn := func(meter metric.Meter) error {
		for _, create := range g.pending {
			if err := create(meter); err != nil {
				return err
			}
		}

		wrappedCollectFn := func(ctx context.Context, o metric.Observer) error {
			return collectFn(ctx, Observer{Observer: o})
		}

		_, err := meter.RegisterCallback(wrappedCollectFn, g.observables...)
		return err
	}

	return singletonMetricsOutput.Register(MetricName(groupName), registerFn)
}
