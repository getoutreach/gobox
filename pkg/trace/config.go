package trace

import (
	"github.com/getoutreach/gobox/pkg/cfg"
)

// tracing config goes into trace.yaml
type Config struct {
	Otel       `yaml:"OpenTelemetry"`
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

func (c *Config) Load() error {
	return cfg.Load("trace.yaml", c)
}
