package log_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"gotest.tools/v3/assert"
)

// TestSlogErrorKind verifies error.kind end to end through events.Err and the
// slog handler (slogAttrs): exactly one error.kind with the expected value.
func TestSlogErrorKind(t *testing.T) {
	cleanup := setupSlogTest(t)
	defer cleanup()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	ctx := context.Background()

	base := errors.New("base failure")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"default", base, "error"},
		{"explicit", orerr.Kind(base, "timeout"), "timeout"},
		{"explicit wrapped", fmt.Errorf("ctx: %w", orerr.WithInfo(log.F{"a": "b"})(orerr.Kind(base, "timeout"))), "timeout"},
		{"explicit beats Info error.kind", orerr.Kind(orerr.Info(base, log.F{"error.kind": "legacy"}), "timeout"), "timeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Repeat: the old behavior depended on unstable sorting.
			for i := 0; i < 20; i++ {
				buf.Reset()
				log.Error(ctx, "failed", events.Err(tt.err))
				out := buf.String()
				assert.Equal(t, strings.Count(out, "error.kind="), 1, out)
				assert.Assert(t, strings.Contains(out, "error.kind="+tt.want+" ") ||
					strings.Contains(out, "error.kind="+tt.want+"\n") ||
					strings.HasSuffix(out, "error.kind="+tt.want), out)
			}
		})
	}
}
