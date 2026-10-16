// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests config-driven per-call level overrides.

package trace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/log/logtest"
	"github.com/getoutreach/gobox/pkg/olog"
	"github.com/getoutreach/gobox/pkg/trace"
	"gotest.tools/v3/assert"
)

func TestCallLevelOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "olog.yaml")
	t.Cleanup(func() {
		assert.NilError(t, os.WriteFile(path, []byte("calls: []\n"), 0o644))
		assert.NilError(t, olog.ConfigureFromFile(path))
	})

	cfg := "calls:\n  - name: quiet\n    level: WARN\n  - name: loud\n    level: INFO\n"
	assert.NilError(t, os.WriteFile(path, []byte(cfg), 0o644))
	assert.NilError(t, olog.ConfigureFromFile(path))

	count := func(name string, opts ...log.Marshaler) int {
		recorder := logtest.NewLogRecorder(t)
		defer recorder.Close()
		ctx := trace.StartCall(t.Context(), name, opts...)
		trace.EndCall(ctx)
		return len(recorder.Entries())
	}

	assert.Equal(t, count("quiet", trace.WithInfoLoggingEnabled()), 0)
	assert.Equal(t, count("loud"), 1)
	assert.Equal(t, count("other"), 0)
	assert.Equal(t, count("other", trace.WithInfoLoggingEnabled()), 1)
}
