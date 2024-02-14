// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains configuration for the ometrics package.

package ometrics

import "time"

// Config is the configuration for a meter provider created by this
// package. This is meant to be used by InitializeMeterProvider.
type Config struct {
	// Collector contains configuration for the collector exporter. This
	// is only valid when using the ExporterTypeCollector.
	Collector CollectorConfig
}

// CollectorConfig contains configuration for creating a
// ExporterTypeCollector exporter through InitializeMeterProvider.
type CollectorConfig struct {
	// Interval is the time at which metrics should be read and
	// subsequently pushed to the collector.
	Interval time.Duration
}

// Option is a function that sets a configuration value.
type Option func(c *Config)

// WithConfig sets the configuration for a meter provider replacing all
// default values.
func WithConfig(c Config) Option {
	return func(cfg *Config) {
		*cfg = c
	}
}
