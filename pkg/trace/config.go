package trace

import (
	"github.com/getoutreach/gobox/pkg/cfg"
)

// tracing config goes into trace.yaml
type Config struct {
	Honeycomb `yaml:"Honeycomb"`
}

type Honeycomb struct {
	Enabled       bool       `yaml:"Enabled"`
	APIHost       string     `yaml:"APIHost"`
	Dataset       string     `yaml:"Dataset"`
	SamplePercent float64    `yaml:"SamplePercent"`
	Debug         bool       `yaml:"Debug"`
	Stdout        bool       `yaml:"Stdout"`
	APIKey        cfg.Secret `yaml:"APIKey"`
}

func (c *Config) Load() error {
	return cfg.Load("trace.yaml", c)
}
