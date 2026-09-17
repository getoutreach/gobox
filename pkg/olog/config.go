// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Implements configuration loading and watching for olog.

package olog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// stringLevel is a map of string to slog.Level.
var stringLevel = map[string]slog.Level{
	slog.LevelDebug.String(): slog.LevelDebug,
	slog.LevelInfo.String():  slog.LevelInfo,
	slog.LevelWarn.String():  slog.LevelWarn,
	slog.LevelError.String(): slog.LevelError,
	"OFF":                    slog.Level(100),
}

// LevelConfig is a single address-to-level mapping. The address is
// either a module or a package path.
type LevelConfig struct {
	// Address is a module or package path.
	Address string `yaml:"address"`

	// Level is one of DEBUG, INFO, WARN, ERROR, or OFF.
	Level string `yaml:"level"`
}

// Config is level configuration for olog. The address is either a module
// or a package, and the level is one of DEBUG, INFO, WARN, ERROR, or OFF.
type Config struct {
	Levels []LevelConfig `yaml:"log"`
}

// ConfigureFromFile loads the level configuration from the provided path.
func ConfigureFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w reading %s", err, path)
	}

	c := Config{}
	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return fmt.Errorf("%w unmarshalling %s: \n%s", err, path, string(data))
	}

	for _, configuredLevel := range c.Levels {
		l, ok := stringLevel[strings.ToUpper(configuredLevel.Level)]
		if !ok {
			New().Error("unknown level", "level", configuredLevel.Level, "address", configuredLevel.Address)
			continue
		}
		globalLevelRegistry.Set(l, configuredLevel.Address)
	}

	return nil
}

// PollConfigurationFile watches the level configuration file for changes and reloads it.
//
//   - if [ctx] ends, Poll exits. Otherwise, it blocks until fn returns false
//   - logCfgFilePath is the file to watch
//   - pollInterval controls how frequently PollConfigurationFile polls the file
//   - if Poll encounters an error or successfully loads a config, fn is called. If the errFunc returns false,
//     poller exits
//
// This is useful, for example, to watch a file mounted by a configmap for changes:
// https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#mounted-configmaps-are-updated-automatically
func PollConfigurationFile(ctx context.Context, logCfgFilePath string, pollInterval time.Duration, fn func(err error) bool) {
	lastModTime := time.Time{}
	for ctx.Err() == nil {
		stat, err := os.Stat(logCfgFilePath)

		if err != nil {
			ok := fn(fmt.Errorf("failed to stat %s: %w", logCfgFilePath, err))
			if !ok {
				return
			}
			sleep(ctx, pollInterval)
			continue
		}

		if stat.ModTime().After(lastModTime) {
			lastModTime = stat.ModTime()
			err := ConfigureFromFile(logCfgFilePath)
			ok := fn(err)
			if !ok {
				return
			}
		}

		sleep(ctx, pollInterval)
	}
}

// sleep waits for the provided duration or until the context is done,
// whichever comes first. Inlined (instead of pkg/async.Sleep) to avoid
// an import cycle: async -> log -> olog.
func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
