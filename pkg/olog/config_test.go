package olog

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "embed"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

func TestConfigFromFile(t *testing.T) {
	c := Config{
		Levels: []struct {
			Address string "yaml:\"address\""
			Level   string "yaml:\"level\""
		}{
			{
				Address: "warnModule",
				Level:   "warn",
			},

			{
				Address: "infoPackage",
				Level:   "INFO",
			},

			{
				Address: "offModule",
				Level:   "OFF",
			},
		},
	}

	dir := t.TempDir()

	configBytes, err := yaml.Marshal(c)
	assert.NilError(t, err)

	err = os.WriteFile(dir+"/olog.yaml", configBytes, 0644)
	assert.NilError(t, err)

	err = ConfigureFromFile(dir + "/olog.yaml")
	assert.NilError(t, err)

	logCapture := NewTestCapturer(t)

	SetDefaultHandler(JSONHandler)
	SetGlobalLevel(slog.LevelDebug)

	loggers := map[slog.Level]struct {
		*slog.Logger
		count int
	}{
		slog.LevelWarn: {
			NewWithHandler(createHandler(globalLevelRegistry, &metadata{
				ModulePath:  "warnModule",
				PackagePath: "warnPackage",
			})),
			2,
		},
		slog.LevelInfo: {
			NewWithHandler(createHandler(globalLevelRegistry, &metadata{
				ModulePath:  "infoModule",
				PackagePath: "infoPackage",
			})),
			3,
		},
		slog.Level(100): {
			NewWithHandler(createHandler(globalLevelRegistry, &metadata{
				ModulePath:  "offModule",
				PackagePath: "offPackage",
			})),
			0,
		},
		slog.LevelDebug: {New(), 4},
	}

	for minLevel, logger := range loggers {
		logger.Debug("debug")
		logger.Info("info")
		logger.Warn("warn")
		logger.Error("error")

		logs := logCapture.GetLogs()
		for _, l := range logs {
			assert.Check(t, l.Level >= minLevel)
		}

		assert.Equal(t, len(logs), logger.count, "expected %d logs > %s; got %v", logger.count, minLevel, logs)
	}
}

//go:embed fixtures/info.yaml
var info string

//go:embed fixtures/warn.yaml
var warn string

func TestPollConfigurationFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	file := t.TempDir() + "log.yaml"

	count := 0

	PollConfigurationFile(ctx, file, time.Millisecond, func(err error) bool {
		defer func() {
			count++
		}()
		if errors.Is(err, os.ErrNotExist) {
			err := os.WriteFile(file, []byte(info), 0644)
			return err == nil
		}

		if count == 1 && err == nil {
			t.Log(globalLevelRegistry.ByAddress)
			assert.Equal(t, *globalLevelRegistry.Get("module"), slog.LevelInfo)
			err := os.WriteFile(file, []byte(warn), 0644)
			return err == nil
		}
		if count == 2 && err == nil {
			assert.Equal(t, *globalLevelRegistry.Get("package"), slog.LevelWarn)
			cancel()
		}
		assert.NilError(t, err)
		return err == nil
	})
	assert.Equal(t, count, 3)
}
