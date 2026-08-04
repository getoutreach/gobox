// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for config.go

package cli

import (
	"testing"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/trace"
	"gotest.tools/v3/assert"
)

// TestOverrideConfigLoadersDisabledTelemetry ensures that setting
// Config.Telemetry.Disabled results in a trace.yaml override that reports
// tracing as fully disabled, instead of the default behavior of hardcoding
// OpenTelemetry on.
func TestOverrideConfigLoadersDisabledTelemetry(t *testing.T) {
	originalReader := cfg.DefaultReader()
	defer cfg.SetDefaultReader(originalReader)

	overrideConfigLoaders(&Config{
		Telemetry: TelemetryConfig{Disabled: true},
	})

	var traceConfig trace.Config
	assert.NilError(t, cfg.Load("trace.yaml", &traceConfig))
	assert.DeepEqual(t, traceConfig, trace.Config{})
}

// TestOverrideConfigLoadersEnabledTelemetry ensures the pre-existing
// behavior is unchanged when Disabled is not set: OpenTelemetry is
// hardcoded on regardless of what a mounted trace.yaml contains.
func TestOverrideConfigLoadersEnabledTelemetry(t *testing.T) {
	originalReader := cfg.DefaultReader()
	defer cfg.SetDefaultReader(originalReader)

	overrideConfigLoaders(&Config{
		Telemetry: TelemetryConfig{
			Otel: TelemetryOtelConfig{Dataset: "test-dataset"},
		},
	})

	var traceConfig trace.Config
	assert.NilError(t, cfg.Load("trace.yaml", &traceConfig))
	assert.DeepEqual(t, traceConfig, trace.Config{
		Otel: trace.Otel{
			Enabled:  true,
			Endpoint: "api.honeycomb.io",
			APIKey: cfg.Secret{
				Path: "APIKey",
			},
			Dataset:       "test-dataset",
			SamplePercent: 100,
		},
	})
}
