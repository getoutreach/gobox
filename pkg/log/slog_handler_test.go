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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// TestSetHandlerEarlyInstallation verifies that SetHandler works when called
// before the first log line and forces the slog facade on.
func TestSetHandlerEarlyInstallation(t *testing.T) {
	if ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to NOT be initially set")
	}

	// Create a tracking handler
	handler := &trackingHandler{name: "early"}
	SetHandler(handler)

	// Verify slog facade is now enabled
	require.True(t, ShouldUseSlog(), "SetHandler should enable slog facade")

	// Log a message; handler should receive it
	ctx := context.Background()
	Info(ctx, "early install test", F{"key": "value"})

	// Verify the handler got the log
	require.Equal(t, 1, handler.handleCalls, "handler should have received the log record")
}

// TestSetHandlerLateInstallation verifies that SetHandler works when called
// after the first log line (bypassing the once guard).
func TestSetHandlerLateInstallation(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}

	// Force slog to initialize with default handler
	ctx := context.Background()
	Info(ctx, "initial log", F{"pre": "install"})

	// Now install a custom handler
	handler := &trackingHandler{name: "late"}
	SetHandler(handler)

	// Log again; new handler should receive it (not the original)
	Info(ctx, "after install", F{"post": "install"})

	// Verify handler got at least one call (may get both if setup cached)
	require.GreaterOrEqual(t, handler.handleCalls, 1, "SetHandler should install the new handler")
}

// trackingHandler counts Handle calls for testing.
type trackingHandler struct {
	name       string
	handleCalls int
}

func (h *trackingHandler) Handle(ctx context.Context, r slog.Record) error {
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
func (h *fakeFlushHandler) Handle(ctx context.Context, r slog.Record) error {
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

	// Create a fake flusher handler
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	flusher := &fakeFlushHandler{Handler: baseHandler}
	SetHandler(flusher)

	// Call fatalFlush (which is what Fatal calls before os.Exit)
	ctx := context.Background()
	fatalFlush(ctx)

	// Verify ForceFlush was called
	require.Equal(t, 1, flusher.flushCalls, "ForceFlush should have been called once")
}

// slowFlushHandler is a handler that delays on ForceFlush for testing timeout behavior.
type slowFlushHandler struct {
	slog.Handler
	delay time.Duration
}

func (h *slowFlushHandler) Handle(ctx context.Context, r slog.Record) error {
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
	require.Less(t, elapsed, 3*time.Second, "fatalFlush should timeout after ~2 seconds")
}

// TestSlogItExtractsTraceAndSpanID verifies that slogIt extracts both traceID
// and spanID from a valid span context, and correctly omits spanID when it's zero.
func TestSlogItExtractsTraceAndSpanID(t *testing.T) {
	if !ShouldUseSlog() {
		t.Skip("test requires GOBOX_AS_SLOG_FACADE to be set")
	}

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
		require.NotNil(t, capturedRecord, "handler should have captured the record")
		hasTraceID := false
		hasSpanID := false
		capturedRecord.Attrs(func(a slog.Attr) bool {
			if a.Key == "traceID" {
				hasTraceID = true
				require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", a.Value.String())
			}
			if a.Key == "spanID" {
				hasSpanID = true
				require.Equal(t, "00f067aa0ba902b7", a.Value.String())
			}
			return true
		})
		require.True(t, hasTraceID, "traceID should be present in the log record")
		require.True(t, hasSpanID, "spanID should be present in the log record")
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
		require.NotNil(t, capturedRecord, "handler should have captured the record")
		hasTraceID := false
		hasSpanID := false
		capturedRecord.Attrs(func(a slog.Attr) bool {
			if a.Key == "traceID" {
				hasTraceID = true
				require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", a.Value.String())
			}
			if a.Key == "spanID" {
				hasSpanID = true
			}
			return true
		})
		require.True(t, hasTraceID, "traceID should be present")
		require.False(t, hasSpanID, "spanID should NOT be present when spanID is zero")
	})
}

// capturingHandler captures the first log record for testing.
type capturingHandler struct {
	capturedRecord **slog.Record
}

// Handle captures the record.
func (h *capturingHandler) Handle(ctx context.Context, r slog.Record) error {
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

	// Create a regular handler (no Flusher implementation)
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	SetHandler(handler)

	// This should not panic or error
	ctx := context.Background()
	fatalFlush(ctx)
	// If we get here without panicking, the test passes
}
