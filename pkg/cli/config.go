// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains config related helpers for
// CLIs and pkg/config.

package cli

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/secrets"
	"github.com/getoutreach/gobox/pkg/trace"
	"gopkg.in/yaml.v3"
)

// overrideConfigLoaders fakes certain parts of the config that usually get pulled
// in via mechanisms that don't make sense to use in CLIs.
func overrideConfigLoaders(honeycombAPIKey, dataset string, tracingDebug bool) {
	// override the secret loader so that we can read specific keys from variables
	// otherwise fallback to the original secret loader, if it was set.
	var fallbackSecretLookup func(context.Context, string) ([]byte, error)
	fallbackSecretLookup = secrets.SetDevLookup(func(ctx context.Context, path string) ([]byte, error) {
		// use the embedded in value
		if path == "APIKey" {
			return []byte(honeycombAPIKey), nil
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

	fallbackConfigReader := cfg.DefaultReader()
	cfg.SetDefaultReader(func(fileName string) ([]byte, error) {
		if fileName == "trace.yaml" {
			traceConfig := &trace.Config{
				Otel: trace.Otel{
					Enabled:  true,
					Endpoint: "api.honeycomb.io",
					APIKey: cfg.Secret{
						Path: "APIKey",
					},
					Debug:         tracingDebug,
					Dataset:       dataset,
					SamplePercent: 100,
				},
			}
			b, err := yaml.Marshal(&traceConfig)
			if err != nil {
				panic(err)
			}
			return b, nil
		}

		return fallbackConfigReader(fileName)
	})
}
