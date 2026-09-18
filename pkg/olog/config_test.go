package olog

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "embed"

	"github.com/pkg/errors"
	"go.yaml.in/yaml/v3"
	"gotest.tools/v3/assert"
)

func TestConfigFromFile(t *testing.T) {
	c := Config{
		Levels: []LevelConfig{
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

	err = os.WriteFile(filepath.Join(dir, "olog.yaml"), configBytes, 0o644)
	assert.NilError(t, err)

	err = ConfigureFromFile(filepath.Join(dir, "olog.yaml"))
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

func TestConfigFromFile_UnknownLevel(t *testing.T) {
	c := Config{
		Levels: []LevelConfig{
			{
				Address: "unknownModule",
				Level:   "INVALID_LEVEL",
			},
		},
	}

	dir := t.TempDir()
	configBytes, err := yaml.Marshal(c)
	assert.NilError(t, err)

	filePath := filepath.Join(dir, "olog.yaml")
	err = os.WriteFile(filePath, configBytes, 0o644)
	assert.NilError(t, err)

	err = ConfigureFromFile(filePath)
	assert.Assert(t, err != nil, "expected an unknown level to be reported")
	assert.Assert(t, errors.Is(err, ErrUnknownLevel))
	assert.Assert(t, strings.Contains(err.Error(), "unknownModule"), "error should name the address: %v", err)

	assert.Assert(t, globalLevelRegistry.Get("unknownModule") == nil)
}

// TestConfigFromFile_UnknownLevelAppliesValidEntries ensures a single
// bad entry does not prevent the remaining entries from being applied.
func TestConfigFromFile_UnknownLevelAppliesValidEntries(t *testing.T) {
	c := Config{
		Levels: []LevelConfig{
			{Address: "badLevelModule", Level: "NOPE"},
			{Address: "goodLevelModule", Level: "ERROR"},
		},
	}

	dir := t.TempDir()
	configBytes, err := yaml.Marshal(c)
	assert.NilError(t, err)

	filePath := filepath.Join(dir, "olog.yaml")
	assert.NilError(t, os.WriteFile(filePath, configBytes, 0o644))

	err = ConfigureFromFile(filePath)
	assert.Assert(t, errors.Is(err, ErrUnknownLevel))

	got := globalLevelRegistry.Get("goodLevelModule")
	assert.Assert(t, got != nil, "valid entry should still be applied")
	assert.Equal(t, *got, slog.LevelError)
	assert.Assert(t, globalLevelRegistry.Get("badLevelModule") == nil)
}

//go:embed fixtures/info.yaml
var info string

//go:embed fixtures/warn.yaml
var warn string

func TestPollConfigurationFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	file := filepath.Join(t.TempDir(), "log.yaml")

	count := 0

	PollConfigurationFile(ctx, file, time.Millisecond, func(err error) bool {
		defer func() {
			count++
		}()
		if errors.Is(err, os.ErrNotExist) {
			err := os.WriteFile(file, []byte(info), 0o644)
			return err == nil
		}

		if count == 1 && err == nil {
			t.Log(globalLevelRegistry.ByAddress)
			assert.Equal(t, *globalLevelRegistry.Get("module"), slog.LevelInfo)
			err := os.WriteFile(file, []byte(warn), 0o644)
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
