package olog

import (
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestLogLevelByModule ensures that the log-level is able to be
// determined by the module name that a logger was created in.
func TestLogLevelByModule(t *testing.T) {
	lr := newRegistry()
	logCapture := NewTestCapturer(t)

	logger := NewWithHandler(createHandler(lr, &metadata{ModuleName: "testModuleName", PackageName: "testPackageName"}))
	nullLogger := NewWithHandler(createHandler(lr, &metadata{ModuleName: "nullModuleName", PackageName: "nullPackageName"}))

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
