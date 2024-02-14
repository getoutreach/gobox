package olog

import (
	"context"
	"log/slog"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/google/go-cmp/cmp"
)

// TestLogLevelByModule ensures that the log-level is able to be
// determined by the module name that a logger was created in.
func TestLogLevelByModule(t *testing.T) {
	// Force JSON handler for valid unmarshaling used in the TestCapturer
	SetDefaultHandler(JSONHandler)

	lr := newRegistry()
	logCapture := NewTestCapturer(t)

	logger := NewWithHandler(createHandler(lr, &metadata{ModulePath: "testModuleName", PackagePath: "testPackageName"}))
	nullLogger := NewWithHandler(createHandler(lr, &metadata{ModulePath: "nullModuleName", PackagePath: "nullPackageName"}))

	// Effectively disable logging for the null logger.
	lr.Set(slog.Level(100), "nullModuleName")

	nullLogger.Info("should not appear")
	logger.Info("should appear")

	// Verify the right messages were logged.
	expected := []TestLogLine{
		{Message: "should appear", Level: slog.LevelInfo, Attrs: map[string]any{"module": "testModuleName", "modulever": ""}},
	}

	if diff := cmp.Diff(expected, logCapture.GetLogs()); diff != "" {
		t.Fatalf("unexpected log output (-want +got):\n%s", diff)
	}
}

func TestLogWithHook(t *testing.T) {
	// Force JSON handler for valid unmarshaling used in the TestCapturer
	SetDefaultHandler(JSONHandler)

	logCapture := NewTestCapturer(t)

	// Create logger with test hook
	logger := NewWithHooks(func(context.Context, slog.Record) ([]slog.Attr, error) {
		return []slog.Attr{slog.Any("data", map[string]string{"foo": "bar"})}, nil
	})

	logger.Info("should appear")

	// Verify the right messages were logged.
	expected := []TestLogLine{
		{
			Message: "should appear",
			Level:   slog.LevelInfo,
			Attrs: map[string]any{
				"module":    "github.com/getoutreach/gobox",
				"modulever": "",
				"data":      map[string]any{"foo": "bar"},
			},
		},
	}

	if diff := cmp.Diff(expected, logCapture.GetLogs()); diff != "" {
		t.Fatalf("unexpected log output (-want +got):\n%s", diff)
	}
}

func TestLogWithAppInfoHook(t *testing.T) {
	// Force JSON handler for valid unmarshaling used in the TestCapturer
	SetDefaultHandler(JSONHandler)

	logCapture := NewTestCapturer(t)

	// Initialize test app info
	app.SetName("ologHooksTest")

	// Due to different environments during testing, capturing expected
	// app version directly from Info(). Shouldn't affect test integrity
	// as we mostly want to test the output structure.
	expAppInfo := map[string]any{
		"name":         "ologHooksTest",
		"service_name": "ologHooksTest",
	}
	if version := app.Info().Version; version != "" {
		expAppInfo["version"] = version
	}
	if namespace := app.Info().Namespace; namespace != "" {
		expAppInfo["deployment.namespace"] = namespace
	}

	// Create logger with test hook
	logger := NewWithHooks(app.LogHook)

	logger.Info("should appear")

	// Verify the right messages were logged.
	expected := []TestLogLine{
		{
			Message: "should appear",
			Level:   slog.LevelInfo,
			Attrs: map[string]any{
				"module":    "github.com/getoutreach/gobox",
				"modulever": "",
				"app":       expAppInfo,
			},
		},
	}

	if diff := cmp.Diff(expected, logCapture.GetLogs()); diff != "" {
		t.Fatalf("unexpected log output (-want +got):\n%s", diff)
	}
}
