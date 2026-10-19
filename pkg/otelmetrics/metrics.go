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

// lastReportedChannelNotInitializedError keeps track of the last time an error was reported for an uninitialized emit channel.
var lastReportedChannelNotInitializedError atomic.Pointer[time.Time]

// lastReportedFullBufferError keeps track of the last time an error was reported for a full emit buffer.
var lastReportedFullBufferError atomic.Pointer[time.Time]

// reportChannelNotInitialized logs a warning if the emit channel for the given metric is not yet initialized.
func reportChannelNotInitialized(ctx context.Context, name MetricName) {
	now := time.Now()
	lastReported := lastReportedChannelNotInitializedError.Load()
	if lastReported != nil && now.Sub(*lastReported) <= 60*time.Minute {
		return
	}

	if swapped := lastReportedChannelNotInitializedError.CompareAndSwap(lastReported, &now); swapped {
		log.Warn(ctx, "Emit channel not initialized for OTLP metrics", log.F{"metric_name": name})
	}
}

// reportFullEmitBuffer logs a warning if the emit buffer for the given metric is full (at most once per minute).
func reportFullEmitBuffer(ctx context.Context, name MetricName) {
	now := time.Now()
	lastReported := lastReportedFullBufferError.Load()
	if lastReported != nil && now.Sub(*lastReported) <= time.Minute {
		return
	}

	if swapped := lastReportedFullBufferError.CompareAndSwap(lastReported, &now); swapped {
		log.Warn(ctx, "Full emit buffer for OTLP metrics", log.F{"metric_name": name})
	}
}

// Observer is a wrapper around the OpenTelemetry metric.Observer interface.
type Observer struct {
	metric.Observer
}
