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

// lastReportedError keeps track of the last time an error was reported for a full emit buffer.
var lastReportedError atomic.Pointer[time.Time]

// reportFullEmitBuffer logs a warning if the emit buffer for the given metric is full (at most once per minute).
func reportFullEmitBuffer(ctx context.Context, name MetricName) {
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

// Observer is a wrapper around the OpenTelemetry metric.Observer interface.
type Observer struct {
	metric.Observer
}
