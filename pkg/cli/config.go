// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains config related helpers for
// CLIs and pkg/config.

package cli

import (
	"os"
	"strconv"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/cli/logfile"
	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// overrideConfigLoaders fakes certain parts of the config that usually get pulled
// in via mechanisms that don't make sense to use in CLIs.
func overrideConfigLoaders() {
	fallbackConfigReader := cfg.DefaultReader()

	cfg.SetDefaultReader(func(fileName string) ([]byte, error) {
		// try to use fake file first
		if fileName == "trace.yaml" {
			portStr := os.Getenv(logfile.TracePortEnvironmentVariable)
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse trace port")
			}

			traceConfig := &trace.Config{
				LogFile: trace.LogFile{
					Port: port,
				},
			}

			b, err := yaml.Marshal(&traceConfig)
			if err != nil {
				panic(err)
			}

			return b, nil
		}

		// otherwise fallback to default
		bytes, err := fallbackConfigReader(fileName)
		return bytes, err
	})
}
