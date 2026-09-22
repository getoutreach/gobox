// Copyright 2026 Outreach Corporation. All Rights Reserved.
// Description: This file defines the configuration for OpenTelemetry (OTLP push) metrics.

package otelmetrics

import (
	"time"
)

// Config holds configuration for OTLP push metrics.
type Config struct {
	// Enabled controls whether OTLP push metrics are initialized.
	Enabled bool `yaml:"Enabled"`

	// Endpoint is the OTLP gRPC endpoint (host:port), e.g.
	// "otel-gateway-collector.observability.svc.cluster.local:4317".
	Endpoint string `yaml:"Endpoint"`

	// PushInterval is how often metrics are pushed (default: 30s).
	PushInterval time.Duration `yaml:"PushInterval"`

	// ExportTimeout is the maximum time allowed for an export (default: 30s).
	// This provides backpressure - if exports take too long, they are cancelled.
	ExportTimeout time.Duration `yaml:"ExportTimeout"`

	// ServiceName is the service name reported on the metrics resource.
	// Defaults to the app name when empty.
	ServiceName string `yaml:"ServiceName"`

	// PushWorkerCount is the number of concurrent workers pushing metrics (default: 1).
	PushWorkerCount int `yaml:"PushWorkerCount"`

	// EmitBufferSize bounds the number of outstanding (not yet processed) metric emissions.
	EmitBufferSize int `yaml:"EmitBufferSize"`
}

// MarshalLog adds the config to the logger
func (c Config) MarshalLog(addField func(key string, value interface{})) {
	addField("Enabled", c.Enabled)
	addField("Endpoint", c.Endpoint)
	addField("PushInterval", c.PushInterval)
	addField("ExportTimeout", c.ExportTimeout)
	addField("ServiceName", c.ServiceName)
	addField("PushWorkerCount", c.PushWorkerCount)
	addField("EmitBufferSize", c.EmitBufferSize)
}

// DefaultPushWorkerCount is used when Config.PushWorkerCount is unset. At
// least one worker is required to ever drain the emit queue - leaving this at
// zero silently disables all metric recording (every Record() call finds the
// queue "full" because nothing is consuming it).
const DefaultPushWorkerCount = 1

// DefaultEmitBufferSize is used when Config.EmitBufferSize is unset.
const DefaultEmitBufferSize = 1000

// WithDefaults returns a copy of the config with default values applied.
func (c Config) WithDefaults(serviceName string) Config {
	cfg := c
	if cfg.ServiceName == "" {
		cfg.ServiceName = serviceName
	}
	if cfg.PushInterval <= 0 {
		cfg.PushInterval = 30 * time.Second
	}
	if cfg.ExportTimeout == 0 {
		cfg.ExportTimeout = 30 * time.Second
	}
	if cfg.PushWorkerCount <= 0 {
		cfg.PushWorkerCount = DefaultPushWorkerCount
	}
	if cfg.EmitBufferSize <= 0 {
		cfg.EmitBufferSize = DefaultEmitBufferSize
	}
	return cfg
}
