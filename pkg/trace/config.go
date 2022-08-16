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
	Enabled            bool       `yaml:"Enabled"`
	Endpoint           string     `yaml:"Endpoint"`
	AdditionalEndpoint string     `yaml:"AdditionalEndpoint"`
	Dataset            string     `yaml:"Dataset"`
	SamplePercent      float64    `yaml:"SamplePercent"`
	Debug              bool       `yaml:"Debug"`
	Stdout             bool       `yaml:"Stdout"`
	APIKey             cfg.Secret `yaml:"APIKey"`
	AdditionalAPIKey   cfg.Secret `yaml:"AdditionalAPIKey"`
}

func (c *Config) Load() error {
	return cfg.Load("trace.yaml", c)
}
