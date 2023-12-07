package olog

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestLogLevelByModule ensures that the log-level is able to be
// determined by the module name that a logger was created in.
func TestLogLevelByModule(t *testing.T) {
	lr := newRegistry()
	out := &bytes.Buffer{}

	logger := new(lr, out, &metadata{ModuleName: "testModuleName", PackageName: "testPackageName"}, nil)
	nullLogger := new(lr, out, &metadata{ModuleName: "nullModuleName", PackageName: "nullPackageName"}, nil)

	// Effectively disable logging for the null logger.
	lr.Set(slog.Level(100), "nullModuleName")

	nullLogger.Info("should not appear")
	logger.Info("should appear")

	// Verify the right messages were logged.
	//
	// TODO(jaredallard): This will have a helper to make it easier to
	// work with.
	expected := []string{
		`{"time":"2023-10-31T00:00:00Z","level":"INFO","source":{"function":"github.com/getoutreach/gobox/pkg/olog.TestLogLevelByModule","file":"/home/jaredallard/Code/getoutreach/gobox/pkg/olog/olog_test.go","line":43},"msg":"should appear"}`,
	}
	expectedStr := strings.Join(expected, "\n") + "\n"

	if diff := cmp.Diff(expectedStr, out.String()); diff != "" {
		t.Log("Got:\n", out.String())
		t.Fatalf("unexpected output (-want +got):\n%s", diff)
	}
}
