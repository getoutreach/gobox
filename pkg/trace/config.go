// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements configuration types and helpers for
// configuring tracing.

package trace

import (
	"github.com/getoutreach/gobox/pkg/cfg"
)

// tracing config goes into trace.yaml
type Config struct {
	Otel       `yaml:"OpenTelemetry"`
	LogFile    `yaml:"LogFile"`
	GlobalTags `yaml:"GlobalTags,omitempty"`
}

type GlobalTags struct {
	DevEmail string `yaml:"DevEmail,omitempty"`
}

func (g *GlobalTags) MarshalLog(addField func(key string, v interface{})) {
	if g.DevEmail != "" {
		addField("dev.email", g.DevEmail)
	}
}

// Otel is the configuration for OpenTelemetry based tracing
type Otel struct {
	// Enabled determines whether to turn on tracing
	Enabled bool `yaml:"Enabled"`
	// Endpoint for the tracing backend
	Endpoint string `yaml:"Endpoint"`
	// AdditionalEndpoint another backend for tracing
	AdditionalEndpoint string `yaml:"AdditionalEndpoint"`
	// Dataset the honeycomb grouping of traces
	Dataset string `yaml:"Dataset"`
	// SamplePercent the rate at which to sample
	SamplePercent float64 `yaml:"SamplePercent"`
	// Debug allows printing debug statements for traces
	Debug bool `yaml:"Debug"`
	// Stdout also outputs traces to stdout
	Stdout bool `yaml:"Stdout"`
	// APIKey used for authentication with the backend at Endpoint
	APIKey cfg.Secret `yaml:"APIKey"`
	// AdditionalAPIKey used for authentication with the backend at AdditionalEndpoint
	AdditionalAPIKey cfg.Secret `yaml:"AdditionalAPIKey"`
}

// LogFile is the configuration for log file based tracing
type LogFile struct {
	// Port is the port used by the the logfile trace server
	Port int `yaml:"Port"`
}

func (c *Config) Load() error {
	return cfg.Load("trace.yaml", c)
}
