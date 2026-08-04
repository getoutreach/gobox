// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains config related helpers for
// CLIs and pkg/config.

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/logfile"
	"github.com/getoutreach/gobox/pkg/secrets"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"
)

// Config configures the CLI integration provided by gobox
type Config struct {
	// Telemetry is the configuration for telemetry for the CLI
	Telemetry TelemetryConfig

	// Logger is the logger to use for logging
	Logger logrus.FieldLogger
}

// TelemetryConfig is the configuration for telemetry for the CLI
type TelemetryConfig struct {
	// Disabled intentionally turns off all telemetry for the CLI, regardless of
	// UseDelibird, Otel, or a mounted trace.yaml.
	//
	// This framework normally overrides the mounted trace.yaml with a synthesized
	// config that always enables OpenTelemetry tracing to Honeycomb using an API
	// key compiled into the binary. CLIs that don't have a valid Honeycomb API key
	// (for example, cron jobs built with stencil-golang) should set this instead of
	// relying on trace.yaml, since that file is otherwise ignored here and setting
	// `OpenTelemetry.Enabled: false` in it has no effect, producing continuous
	// "unknown API key" export errors. Takes priority over UseDelibird.
	Disabled bool

	// UseDelibird enables the delibird integration, which logs all output to a
	// file as frames (writes to the terminal) as well as records traces.
	//
	// These files will not be automatically uploaded, a daemon must be running on
	// the user's local machine to upload them. For Outreach, this is uploaded by the
	// orc 'delibird' daemon. For other users, these are kept locally.
	UseDelibird bool

	// Otel is the configuration for telemetry when delibird is not in use.
	Otel TelemetryOtelConfig
}

// useDelibird reports whether delibird-based telemetry should be used.
// Disabled always takes priority over UseDelibird.
func (t TelemetryConfig) useDelibird() bool {
	return !t.Disabled && t.UseDelibird
}

// TelemetryOtelConfig is the configuration for telemetry when delibird is not in use.
type TelemetryOtelConfig struct {
	// HoneycombAPIKey is the honeycomb API key to use for telemetry.
	HoneycombAPIKey cfg.SecretData

	// Dataset is the dataset to send telemetry to.
	Dataset string

	// Debug enables debug logging for telemetry.
	Debug bool
}

// useEmbeddedHoneycombAPIKey configures the secret reader to use the
// embedded honeycomb API key, if it is set.
func useEmbeddedHoneycombAPIKey(honeycombAPIKey cfg.SecretData) {
	// override the secret loader so that we can read specific keys from variables
	// otherwise fallback to the original secret loader, if it was set.
	var fallbackSecretLookup func(context.Context, string) ([]byte, error)
	fallbackSecretLookup = secrets.SetDevLookup(func(ctx context.Context, path string) ([]byte, error) {
		// use the embedded in value
		if path == "APIKey" {
			return []byte(string(honeycombAPIKey)), nil
		}

		// if no fallback, return an error, failed to find :(
		// note: as of this time the secrets logic looks for
		// the path before falling back to the devlookup so this
		// is safe to assume all attempts have failed
		if fallbackSecretLookup == nil {
			return nil, fmt.Errorf("failed to find secret at path '%s', or compiled into binary", path)
		}

		return fallbackSecretLookup(ctx, path)
	})
}

// overrideConfigLoaders fakes certain parts of the config that usually get pulled
// in via mechanisms that don't make sense to use in CLIs.
func overrideConfigLoaders(conf *Config) {
	if !conf.Telemetry.Disabled && !conf.Telemetry.UseDelibird {
		useEmbeddedHoneycombAPIKey(conf.Telemetry.Otel.HoneycombAPIKey)
	}

	fallbackConfigReader := cfg.DefaultReader()
	cfg.SetDefaultReader(func(fileName string) ([]byte, error) {
		// try to use fake file first
		if fileName == "trace.yaml" {
			var traceConfig *trace.Config
			switch {
			case conf.Telemetry.Disabled:
				// Report every tracer as off so gobox never attempts to connect
				// anywhere, instead of hardcoding OpenTelemetry on below.
				traceConfig = &trace.Config{}
			case conf.Telemetry.useDelibird():
				portStr := os.Getenv(logfile.TracePortEnvironmentVariable)
				if portStr == "" {
					return nil, fmt.Errorf("delibird enabled, but %s not set", logfile.TracePortEnvironmentVariable)
				}

				port, err := strconv.Atoi(portStr)
				if err != nil {
					return nil, errors.Wrap(err, "failed to parse trace port")
				}

				traceConfig = &trace.Config{
					LogFile: trace.LogFile{
						Enabled: true,
						Port:    port,
					},
				}
			default:
				traceConfig = &trace.Config{
					Otel: trace.Otel{
						Enabled:  true,
						Endpoint: "api.honeycomb.io",
						APIKey: cfg.Secret{
							Path: "APIKey",
						},
						Debug:         conf.Telemetry.Otel.Debug,
						Dataset:       conf.Telemetry.Otel.Dataset,
						SamplePercent: 100,
					},
				}
			}

			b, err := yaml.Marshal(&traceConfig)
			if err != nil {
				panic(err)
			}

			return b, nil
		}

		// otherwise fallback to default
		return fallbackConfigReader(fileName)
	})
}
