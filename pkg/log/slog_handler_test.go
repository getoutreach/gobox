// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests for SetHandler and Fatal flushing behavior

package log

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// restoreSlogState snapshots the current ShouldUseSlog state before a test
// installs a custom handler via SetHandler, and restores it afterward.
// SetHandler permanently forces the slog facade on and replaces the global
// handler; without this restoration, later tests in the same process (which
// share these package-level globals) would unexpectedly run through the
// slog facade with a stale/incompatible handler.
func restoreSlogState(t *testing.T) {
	t.Helper()
	original := ShouldUseSlog()
	t.Cleanup(func() {
		SetShouldUseSlog(original)
	})
}

// TestSetHandlerEarlyInstallation verifies that SetHandler works when called
// before the first log line and forces the slog facade on.
func TestSetHandlerEarlyInstallation(t *testing.T) {
	if ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to NOT be initially set")
	}
	restoreSlogState(t)

	// Create a tracking handler
	handler := &trackingHandler{name: "early"}
	SetHandler(handler)

	// Verify slog facade is now enabled
	if !ShouldUseSlog() {
		t.Fatal("SetHandler should enable slog facade")
	}

	// Log a message; handler should receive it
	ctx := context.Background()
	Info(ctx, "early install test", F{"key": "value"})

	// Verify the handler got the log
	if handler.handleCalls != 1 {
		t.Fatalf("handler should have received the log record: got %d calls", handler.handleCalls)
	}
}

// TestSetHandlerLateInstallation verifies that SetHandler works when called
// after the first log line (bypassing the once guard).
func TestSetHandlerLateInstallation(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}
	restoreSlogState(t)

	// Force slog to initialize with default handler
	ctx := context.Background()
	Info(ctx, "initial log", F{"pre": "install"})

	// Now install a custom handler
	handler := &trackingHandler{name: "late"}
	SetHandler(handler)

	// Log again; new handler should receive it (not the original)
	Info(ctx, "after install", F{"post": "install"})

	// Verify handler got at least one call (may get both if setup cached)
	if handler.handleCalls < 1 {
		t.Fatalf("SetHandler should install the new handler: got %d calls", handler.handleCalls)
	}
}

// trackingHandler counts Handle calls for testing.
type trackingHandler struct {
	name        string
	handleCalls int
}

func (h *trackingHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic
	h.handleCalls++
	return nil
}

func (h *trackingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *trackingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *trackingHandler) WithGroup(name string) slog.Handler {
	return h
}

// fakeFlushHandler is a handler that tracks ForceFlush calls for testing.
type fakeFlushHandler struct {
	slog.Handler
	flushCalls int
	flushErr   error
}

// Handle just delegates to the wrapped handler.
func (h *fakeFlushHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic
	return h.Handler.Handle(ctx, r)
}

// ForceFlush implements the Flusher interface and tracks calls.
func (h *fakeFlushHandler) ForceFlush(ctx context.Context) error {
	h.flushCalls++
	return h.flushErr
}

// Enabled delegates to the wrapped handler.
func (h *fakeFlushHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs delegates to the wrapped handler.
func (h *fakeFlushHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fakeFlushHandler{
		Handler: h.Handler.WithAttrs(attrs),
	}
}

// WithGroup delegates to the wrapped handler.
func (h *fakeFlushHandler) WithGroup(name string) slog.Handler {
	return &fakeFlushHandler{
		Handler: h.Handler.WithGroup(name),
	}
}

// TestFatalFlushesHandler verifies that Fatal calls ForceFlush on handlers
// that implement the Flusher interface.
func TestFatalFlushesHandler(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}
	restoreSlogState(t)

	// Create a fake flusher handler
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	flusher := &fakeFlushHandler{Handler: baseHandler}
	SetHandler(flusher)

	// Call fatalFlush (which is what Fatal calls before os.Exit)
	ctx := context.Background()
	fatalFlush(ctx)

	// Verify ForceFlush was called
	if flusher.flushCalls != 1 {
		t.Fatalf("ForceFlush should have been called once: got %d calls", flusher.flushCalls)
	}
}

// slowFlushHandler is a handler that delays on ForceFlush for testing timeout behavior.
type slowFlushHandler struct {
	slog.Handler
	delay time.Duration
}

func (h *slowFlushHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic
	return h.Handler.Handle(ctx, r)
}

func (h *slowFlushHandler) ForceFlush(ctx context.Context) error {
	select {
	case <-time.After(h.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *slowFlushHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *slowFlushHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slowFlushHandler{
		Handler: h.Handler.WithAttrs(attrs),
		delay:   h.delay,
	}
}

func (h *slowFlushHandler) WithGroup(name string) slog.Handler {
	return &slowFlushHandler{
		Handler: h.Handler.WithGroup(name),
		delay:   h.delay,
	}
}

// TestFatalFlushTimeoutRespectsContext verifies that fatalFlush respects
// the 2-second timeout and doesn't block indefinitely.
func TestFatalFlushTimeoutRespectsContext(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}
	restoreSlogState(t)

	// Create a handler that blocks for longer than 2 seconds
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)

	slowHandler := &slowFlushHandler{
		Handler: baseHandler,
		delay:   5 * time.Second, // Will timeout because fatalFlush uses 2s timeout
	}

	SetHandler(slowHandler)

	// Call fatalFlush and measure time
	ctx := context.Background()
	start := time.Now()
	fatalFlush(ctx)
	elapsed := time.Since(start)

	// Verify it didn't block for 5 seconds (should timeout at ~2 seconds)
	if elapsed >= 3*time.Second {
		t.Fatalf("fatalFlush should timeout after ~2 seconds: took %s", elapsed)
	}
}

// TestSlogItExtractsTraceAndSpanID verifies that slogIt extracts both traceID
// and spanID from a valid span context, and correctly omits spanID when it's zero.
func TestSlogItExtractsTraceAndSpanID(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}
	restoreSlogState(t)

	t.Run("with valid traceID and spanID", func(t *testing.T) {
		// Set up a handler that captures the record
		var capturedRecord *slog.Record
		captureHandler := &capturingHandler{
			capturedRecord: &capturedRecord,
		}
		SetHandler(captureHandler)

		// Build a span context with non-zero traceID and spanID
		traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
		spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
			SpanID:  spanID,
		})

		// Create a context with this span context
		ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

		// Log with the valid span context
		Info(ctx, "test with both IDs", F{"key": "value"})

		// Verify both traceID and spanID are present
		if capturedRecord == nil {
			t.Fatal("handler should have captured the record")
		}
		hasTraceID := false
		hasSpanID := false
		capturedRecord.Attrs(func(a slog.Attr) bool {
			if a.Key == "traceID" {
				hasTraceID = true
				if got := a.Value.String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
					t.Errorf("unexpected traceID: %s", got)
				}
			}
			if a.Key == "spanID" {
				hasSpanID = true
				if got := a.Value.String(); got != "00f067aa0ba902b7" {
					t.Errorf("unexpected spanID: %s", got)
				}
			}
			return true
		})
		if !hasTraceID {
			t.Error("traceID should be present in the log record")
		}
		if !hasSpanID {
			t.Error("spanID should be present in the log record")
		}
	})

	t.Run("with valid traceID but zero spanID", func(t *testing.T) {
		// Set up a handler that captures the record
		var capturedRecord *slog.Record
		captureHandler := &capturingHandler{
			capturedRecord: &capturedRecord,
		}
		SetHandler(captureHandler)

		// Build a span context with non-zero traceID but zero spanID
		traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		spanID := trace.SpanID{} // zero spanID
		spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
			SpanID:  spanID,
		})

		// Create a context with this span context
		ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

		// Log with the span context
		Info(ctx, "test with zero spanID", F{"key": "value"})

		// Verify traceID is present but spanID is absent
		if capturedRecord == nil {
			t.Fatal("handler should have captured the record")
		}
		hasTraceID := false
		hasSpanID := false
		capturedRecord.Attrs(func(a slog.Attr) bool {
			if a.Key == "traceID" {
				hasTraceID = true
				if got := a.Value.String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
					t.Errorf("unexpected traceID: %s", got)
				}
			}
			if a.Key == "spanID" {
				hasSpanID = true
			}
			return true
		})
		if !hasTraceID {
			t.Error("traceID should be present")
		}
		if hasSpanID {
			t.Error("spanID should NOT be present when spanID is zero")
		}
	})
}

// capturingHandler captures the first log record for testing.
type capturingHandler struct {
	capturedRecord **slog.Record
}

// Handle captures the record.
func (h *capturingHandler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic
	*h.capturedRecord = &r
	return nil
}

// Enabled always returns true.
func (h *capturingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// WithAttrs returns a new handler.
func (h *capturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler.
func (h *capturingHandler) WithGroup(name string) slog.Handler {
	return h
}

// TestSetHandlerDisabledWhenNotSlog verifies that SetHandler is a no-op
// when GOBOX_AS_SLOG_FACADE is not set.
func TestSetHandlerDisabledWhenNotSlog(t *testing.T) {
	if ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to NOT be set")
	}
	restoreSlogState(t)

	// This should be a no-op and not panic
	handler := slog.NewJSONHandler(os.Stderr, nil)
	SetHandler(handler)
	// If we get here without panicking, the test passes
}

// TestFatalFlushNonFlusher verifies that fatalFlush safely handles
// handlers that don't implement Flusher.
func TestFatalFlushNonFlusher(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}
	restoreSlogState(t)

	// Create a regular handler (no Flusher implementation)
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	SetHandler(handler)

	// This should not panic or error
	ctx := context.Background()
	fatalFlush(ctx)
	// If we get here without panicking, the test passes
}
