package olog

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/undefinedlabs/go-mpatch"
	"gotest.tools/v3/assert"
)

// fakeTime patches time.Now to return a fixed time. Automatically calls
// t.Cleanup to unpatch time.Now.
func fakeTime(t *testing.T) {
	p, err := mpatch.PatchMethod(time.Now, func() time.Time {
		return time.Date(2023, 10, 31, 00, 00, 00, 0, time.UTC)
	})
	assert.NilError(t, err, "failed to patch time")

	t.Cleanup(func() {
		assert.NilError(t, p.Unpatch(), "failed to unpatch time")
	})
}

// TestLogLevelByModule ensures that the log-level is able to be
// determined by the module name that a logger was created in.
func TestLogLevelByModule(t *testing.T) {
	fakeTime(t)

	lr := newRegistry()
	out := &bytes.Buffer{}

	logger := newTestLogger(lr, out, "moduleNameGoesHere", "packageNameGoesHere")
	nullLogger := newTestLogger(lr, out, "nullModuleName", "nullPackageName")

	// Effectively disable logging for the null logger.
	lr.Set(slog.Level(100), "nullModuleName")

	nullLogger.Info("test message")
	logger.Info("test message")

	// Verify that the message was logged.
	expected := []string{
		`{"time":"2023-10-31T00:00:00Z","level":"INFO","source":{"function":"github.com/getoutreach/gobox/pkg/olog.TestModuleLogger","file":"/home/jaredallard/Code/getoutreach/gobox/pkg/olog/olog_test.go","line":41},"msg":"test message"}`,
	}
	expectedStr := strings.Join(expected, "\n") + "\n"

	if diff := cmp.Diff(expectedStr, out.String()); diff != "" {
		t.Log("Got:\n", out.String())
		t.Fatalf("unexpected output (-want +got):\n%s", diff)
	}
}
