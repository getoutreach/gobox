// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file implements the OTLP push metrics service as an
// async.Runner for proper lifecycle management of OpenTelemetry push metrics.

// Package otelmetrics provides an OTLP push metrics service for OpenTelemetry.
// It stands up an OpenTelemetry meter provider that periodically pushes
// metrics over OTLP/gRPC to the in-cluster OTel gateway, using delta
// temporality (required by the downstream ClickHouse histogram backend).
//
// See: https://outreach-io.atlassian.net/wiki/spaces/Observabil/pages/5171413429/OTLP+Push+Integration
package otelmetrics

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
)

// MetricName represents the name of a metric, must be unique within the binary.
type MetricName string

// emitWorkItem represents a unit of work to emit metrics.
type emitWorkItem struct {
	emit func(ctx context.Context)
}

// Service implements async.Runner for OTLP push metrics lifecycle management.
// Instruments register themselves against the process-wide MetricsOutput
// (typically from an init() function in the package that defines them - see
// e.g. internal/searchindexer/metrics/e2e.go and kafkawriter.go, which import
// this package as a library); Service just brings the meter provider and the
// emit worker pool to life and activates those deferred registrations.
type Service struct {
	cfg           Config
	meterProvider *sdkmetric.MeterProvider

	// ready is closed once instrument activation completes successfully and metrics recording is live.
	// Async observable instruments only appear in a reader's Collect() output once a value has been observed through them.
	ready chan struct{}
}

// singletonService holds the single instance of the OTLP push metrics service.
var singletonService atomic.Pointer[Service]

// Option represents a functional option for configuring the Service.
type Option func(*Service)

// WithMeterProvider sets a custom meter provider for the service.
// Note: for tests only!
func WithMeterProvider(meterProvider *sdkmetric.MeterProvider) Option {
	return func(s *Service) {
		s.meterProvider = meterProvider
	}
}

// NewService creates a new OTLP push metrics service. It copies the config to
// avoid mutation issues and applies defaults.
func NewService(cfg *Config, serviceName string, opts ...Option) *Service {
	service := &Service{
		cfg:   cfg.WithDefaults(serviceName),
		ready: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(service)
	}

	if singletonService.Load() != nil {
		panic("OTLP push metrics service already initialized")
	}

	singletonService.Store(service)

	return service
}

// Ready returns a channel that is closed once instrument activation has completed successfully and metrics recording is live.
func (s *Service) Ready() <-chan struct{} {
	return s.ready
}

// SingletonService returns the single instance of the OTLP push metrics service.
// It panics if the service has not been initialized yet.
func SingletonService() *Service {
	if singletonService.Load() == nil {
		panic("OTLP push metrics service not initialized yet")
	}

	return singletonService.Load()
}

// Run implements async.Runner - initializes OTel and blocks until the context is cancelled.
// A failure to activate the registered instruments is considered fatal.
func (s *Service) Run(ctx context.Context) error {
	if !s.cfg.Enabled {
		log.Info(ctx, "OTLP push metrics disabled, skipping initialization")
		<-ctx.Done()
		return nil
	}

	logData := log.F{
		"otel.endpoint":       s.cfg.Endpoint,
		"otel.push_interval":  s.cfg.PushInterval.String(),
		"otel.export_timeout": s.cfg.ExportTimeout.String(),
	}
	log.Info(ctx, "Initializing OTLP push metrics service", logData)

	// Log OTel errors instead of silently dropping them. Use a background
	// context because the handler may fire after Run's context is cancelled
	// during shutdown.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Error(ctx, "OTel metrics error", events.NewErrorInfo(err))
	}))

	if err := s.initialize(ctx); err != nil {
		// Log but do not fail startup - metrics are non-critical.
		log.Error(ctx, "Failed to initialize OTLP push metrics, continuing without", events.NewErrorInfo(err))
		return err
	}

	log.Info(ctx, "OTLP push metrics service initialized successfully")

	// The queue is bounded (EmitBufferSize) so a burst of Record() calls can never grow memory without limit.
	// Once full, Record() drops samples instead of blocking the caller or spawning unbounded goroutines.
	// Note: we don't close the emit channel ourselves to prevent writers from panicking.
	emitChannel := make(chan emitWorkItem, s.cfg.EmitBufferSize)

	meter := s.meterProvider.Meter(s.cfg.ServiceName)

	// activate actually registers all deferred metrics and enables recording of data.
	if err := singletonMetricsOutput.activate(meter, emitChannel); err != nil {
		return errors.Wrap(err, "failed to activate OTLP metric instruments")
	}

	log.Info(ctx, "OTLP metric instruments activated")
	close(s.ready)

	for i := 0; i < max(s.cfg.PushWorkerCount, 1); i++ {
		go s.worker(ctx, emitChannel)
	}

	<-ctx.Done()

	return nil
}

// Close implements async.Runner - gracefully shuts down the meter provider, flushing any pending metrics.
func (s *Service) Close(ctx context.Context) error {
	if s.meterProvider == nil {
		return nil
	}

	log.Info(ctx, "Shutting down OTLP push metrics service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.meterProvider.Shutdown(shutdownCtx); err != nil {
		log.Error(ctx, "Error shutting down OTel meter provider", events.NewErrorInfo(err))
		return err
	}

	log.Info(ctx, "OTLP push metrics service shut down successfully")

	return nil
}

// initialize sets up the OTel meter provider and calls instrument registrars.
func (s *Service) initialize(ctx context.Context) error {
	if s.meterProvider != nil {
		return nil
	}

	exporterOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(s.cfg.Endpoint),
		otlpmetricgrpc.WithTimeout(s.cfg.ExportTimeout),
		// In-cluster pushes to the OTel gateway are plaintext gRPC.
		otlpmetricgrpc.WithInsecure(),
		// Override temporality to delta aggregation. This is required by the
		// downstream ClickHouse histogram backend; cumulative histograms are
		// prohibitively expensive there.
		// See: https://outreach-io.atlassian.net/wiki/spaces/Observabil/pages/5171413429/OTLP+Push+Integration
		otlpmetricgrpc.WithTemporalitySelector(deltaTemporalitySelector),
	}

	exporter, err := otlpmetricgrpc.New(ctx, exporterOpts...)
	if err != nil {
		return errors.Wrap(err, "failed to create OTLP metric exporter")
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(s.cfg.ServiceName),
		),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create metrics resource")
	}

	s.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter,
				sdkmetric.WithInterval(s.cfg.PushInterval),
			),
		),
	)

	// Register as the global meter provider so any package using otel.Meter
	// picks it up.
	otel.SetMeterProvider(s.meterProvider)

	return nil
}

// worker processes emit work items from the channel until ctx is cancelled.
func (s *Service) worker(ctx context.Context, emitChannel <-chan emitWorkItem) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-emitChannel:
			s.emit(ctx, item)
		}
	}
}

// emit runs a single work item, recovering panics so a bug in one metric callback can never take down the worker.
func (s *Service) emit(ctx context.Context, item emitWorkItem) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(ctx, "panic while emitting otel metric", nil, log.F{"recover": r})
		}
	}()
	item.emit(ctx)
}

// deltaTemporalitySelector selects delta temporality for all instrument kinds
// except UpDownCounters, which the OTLP spec requires to stay
// cumulative to remain meaningful.
func deltaTemporalitySelector(ik sdkmetric.InstrumentKind) metricdata.Temporality {
	switch ik { //nolint:exhaustive // Why: default covers all other kinds.
	case sdkmetric.InstrumentKindUpDownCounter,
		sdkmetric.InstrumentKindObservableUpDownCounter:
		return metricdata.CumulativeTemporality
	default:
		return metricdata.DeltaTemporality
	}
}
