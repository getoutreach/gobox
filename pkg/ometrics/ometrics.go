// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains the main implementation of the ometrics
// package.

// Package ometrics implements a small wrapper around working with the
// otel metrics package. It does not provide any wrappers around the
// core types provided by otel, instead provides a way to instantiate
// them instead.
package ometrics

import (
	"context"
	"fmt"
	"time"

	"github.com/getoutreach/gobox/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// ExporterType denotes the type of exporter to use for metrics.
type ExporterType int

// Contains the different types of exporters that can be used.
const (
	// ExporterTypePrometheus exports metrics in the prometheus format. It
	// is the caller's responsibility to expose the metrics via an HTTP
	// endpoint (usually through promhttp). Example implementation:
	// https://github.com/open-telemetry/opentelemetry-go/blob/main/example/prometheus/main.go#L99
	ExporterTypePrometheus ExporterType = iota

	// ExporterTypeCollector exports metrics to the otel collector. This
	// is akin to the "push" model of metrics, for those familiar with
	// the prometheus model.
	ExporterTypeCollector
)

// InitializeMeterProvider initializes the global meter provider to be
// backed by the provided exporter.
func InitializeMeterProvider(ctx context.Context, t ExporterType, opts ...Option) error {
	c := Config{
		Collector: CollectorConfig{
			// Mirror the default interval, which is 60s.
			Interval: 60 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(&c)
	}

	info := app.Info()

	// QUESTION(jaredallard): Do we want to allow exposing other global attributes?
	resources := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(info.Name),
		semconv.ServiceVersionKey.String(info.Version),
	)

	// Create a reader based on the provided exporter. This is confusingly
	// named. In order for "reader" to make sense, think of this as being
	// consumed by an external source (e.g., Prometheus) by default in the
	// design.
	var reader sdkmetric.Reader
	var err error
	switch t {
	case ExporterTypePrometheus:
		reader, err = prometheus.New()
	case ExporterTypeCollector:
		// TODO(jaredallard): We'll want to plumb in gRPC configuration as
		// well as potentially the periodic read config here.
		var exporter *otlpmetricgrpc.Exporter
		exporter, err = otlpmetricgrpc.New(ctx)
		if err == nil {
			reader = sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(c.Collector.Interval))
		}
	default:
		return fmt.Errorf("exporter type provided is unknown")
	}
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create a meter provider with the provided reader and default
	// resources created earlier.
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resources),
		sdkmetric.WithReader(reader),
	)

	// This configures the global meter provider to be backed by the
	// exporter we just created. This allows users to call otel methods as
	// usual and have them "just work".
	otel.SetMeterProvider(mp)

	return nil
}
