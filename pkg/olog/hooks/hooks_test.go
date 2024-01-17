package hooks

import (
	"context"
	"log/slog"
	"testing"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/olog"
	"github.com/google/go-cmp/cmp"
)

func TestLogWithHook(t *testing.T) {
	// Force JSON handler for valid unmarshaling used in the TestCapturer
	olog.SetDefaultHandler(olog.JSONHandler)

	logCapture := olog.NewTestCapturer(t)

	// Create logger with test hook
	logger := Logger(func(context.Context, slog.Record) ([]slog.Attr, error) {
		return []slog.Attr{slog.Any("data", map[string]string{"foo": "bar"})}, nil
	})

	logger.Info("should appear")

	// Verify the right messages were logged.
	expected := []olog.TestLogLine{
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
	olog.SetDefaultHandler(olog.JSONHandler)

	logCapture := olog.NewTestCapturer(t)

	// Initialize test app info
	app.SetName("ologHooksTest")

	// Create logger with test hook
	logger := Logger(AppInfo)

	logger.Info("should appear")

	// Verify the right messages were logged.
	expected := []olog.TestLogLine{
		{
			Message: "should appear",
			Level:   slog.LevelInfo,
			Attrs: map[string]any{
				"module":    "github.com/getoutreach/gobox",
				"modulever": "",
				"app": map[string]any{
					"name":         "ologHooksTest",
					"service_name": "ologHooksTest",
				},
			},
		},
	}

	if diff := cmp.Diff(expected, logCapture.GetLogs()); diff != "" {
		t.Fatalf("unexpected log output (-want +got):\n%s", diff)
	}
}
