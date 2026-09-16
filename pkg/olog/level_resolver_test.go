package olog

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestLevelResolverOverrides ensures an installed LevelResolver can
// both enable logs below the registry level and suppress logs at or
// above it, using the record context.
func TestLevelResolverOverrides(t *testing.T) {
	SetDefaultHandler(JSONHandler)
	SetGlobalLevel(slog.LevelInfo)
	t.Cleanup(func() {
		SetLevelResolver(nil)
		SetGlobalLevel(slog.LevelInfo)
	})

	lr := newRegistry()
	logCapture := NewTestCapturer(t)

	logger := NewWithHandler(createHandler(lr, &metadata{ModulePath: "resolverModule", PackagePath: "resolverPackage"}))

	type ctxKey struct{}

	SetLevelResolver(func(ctx context.Context, addrs []string, registryLevel slog.Level) (slog.Level, bool) {
		if len(addrs) == 0 || addrs[0] != "resolverPackage" {
			t.Errorf("unexpected addresses: %v", addrs)
		}
		if v, ok := ctx.Value(ctxKey{}).(slog.Level); ok {
			return v, true
		}
		return 0, false
	})

	// No override in ctx: falls back to registry (info default).
	logger.Debug("fallback debug suppressed")
	logger.Info("fallback info appears")

	// Resolver enables debug for this ctx.
	debugCtx := context.WithValue(context.Background(), ctxKey{}, slog.LevelDebug)
	logger.DebugContext(debugCtx, "resolved debug appears")

	// Resolver disables info for this ctx.
	offCtx := context.WithValue(context.Background(), ctxKey{}, slog.Level(100))
	logger.InfoContext(offCtx, "resolved info suppressed")

	expected := []TestLogLine{
		{Message: "fallback info appears", Level: slog.LevelInfo,
			Attrs: map[string]any{"module": "resolverModule", "modulever": ""}},
		{Message: "resolved debug appears", Level: slog.LevelDebug,
			Attrs: map[string]any{"module": "resolverModule", "modulever": ""}},
	}

	if diff := cmp.Diff(expected, logCapture.GetLogs()); diff != "" {
		t.Fatalf("unexpected log output (-want +got):\n%s", diff)
	}
}
