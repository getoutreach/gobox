// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Provides helpers for interacting with the logger in
// tests.

package olog

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
)

// TestLogLine is a log line that was captured by the testHandler. This
// log line was emitted by a logger created during a test.
type TestLogLine struct {
	// Level is the log level of the log.
	Level slog.Level

	// Message is the message that was logged.
	Message string

	// Attrs is a map of attributes that were logged. This does not
	// include the time or source attributes since they are generally not
	// stable across runs.
	Attrs map[string]any
}

// testLogCapturer parses the output of a JSON logger and stores it in a
// slice of TestLogLine.
type testLogCapturer struct {
	io.Writer
	logsMu sync.Mutex
	logs   []TestLogLine
}

// GetLogs returns all of the logs that were emitted by loggers created
// using this handler. This drains the logs slice.
func (t *testLogCapturer) GetLogs() []TestLogLine {
	t.logsMu.Lock()
	defer t.logsMu.Unlock()

	// copy the logs slice so that we can drain it.
	out := make([]TestLogLine, len(t.logs))
	copy(out, t.logs)
	t.logs = make([]TestLogLine, 0)

	return out
}

// Write implements io.Writer and parses the provided log line as a
// TestLogLine and stores it in the logs slice.
func (t *testLogCapturer) Write(p []byte) (n int, err error) {
	var out map[string]any
	if err := json.Unmarshal(p, &out); err != nil {
		return 0, err
	}

	// turn into a TestLogLine
	ll := TestLogLine{Attrs: make(map[string]any)}
	for k, v := range out {
		switch k {
		case "level":
			if err := ll.Level.UnmarshalText([]byte(v.(string))); err != nil {
				return 0, fmt.Errorf("failed to parse log level: %w", err)
			}
		case "msg":
			ll.Message = v.(string)
		case "time", "source": // Ignored fields.
		default:
			// everything else goes into attrs.
			ll.Attrs[k] = v
		}
	}

	// Add the log line to the logs.
	t.logsMu.Lock()
	t.logs = append(t.logs, ll)
	t.logsMu.Unlock()

	return len(p), nil
}

// TestCaptureLogs returns a testLogCapturer that can be used to capture
// all output from loggers created after this call.
//
// Note: This should only ever be used during tests and is not
// thread-safe.
//
// Note (parallel tests): This will not work with parallel tests due to
// the usage of globals in this package.
//
//nolint:revive // Why: We don't want this to be passed around.
func NewTestCapturer(t *testing.T) *testLogCapturer {
	orig := defaultOut
	tc := &testLogCapturer{logs: make([]TestLogLine, 0)}
	defaultOut = tc

	// reset the output
	t.Cleanup(func() {
		defaultOut = orig
	})

	return tc
}
