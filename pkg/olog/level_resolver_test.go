package olog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
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

// TestLevelResolverConcurrentTextHandler ensures that per-context level
// overrides on the text (charm) handler path are evaluated per record
// and never leak across goroutines sharing one logger.
func TestLevelResolverConcurrentTextHandler(t *testing.T) {
	SetDefaultHandler(TextHandler)
	SetGlobalLevel(slog.LevelInfo)
	t.Cleanup(func() {
		SetLevelResolver(nil)
		SetDefaultHandler(JSONHandler)
		SetGlobalLevel(slog.LevelInfo)
	})

	var buf lockedBuffer
	oldOut := defaultOut
	defaultOut = &buf
	t.Cleanup(func() { defaultOut = oldOut })

	type ctxKey struct{}
	SetLevelResolver(func(ctx context.Context, _ []string, _ slog.Level) (slog.Level, bool) {
		if v, ok := ctx.Value(ctxKey{}).(slog.Level); ok {
			return v, true
		}
		return 0, false
	})

	logger := NewWithHandler(createHandler(newRegistry(),
		&metadata{ModulePath: "resolverModule", PackagePath: "resolverPackage"}))

	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), ctxKey{}, slog.LevelDebug)
			logger.DebugContext(ctx, "on")
		}()
		go func() {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), ctxKey{}, slog.Level(100))
			logger.InfoContext(ctx, "off")
		}()
	}
	wg.Wait()

	out := buf.String()
	if got := strings.Count(out, "on"); got != n {
		t.Errorf("expected %d enabled records, got %d", n, got)
	}
	if strings.Contains(out, "off") {
		t.Error("suppressed records leaked into output")
	}
}

// lockedBuffer is a concurrency-safe io.Writer for test output.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
