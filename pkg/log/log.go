// Package log implements standard go logging
//
//
// For logging:
//
//    log.Info(ctx, "message", log.F{field: 42})
//    log.Error(...)
//    log.Debug(...)
//    log.Fatal(...)
//
// By default, log.Debug is not emitted but instead it is cached. If
// a higher event arrives within a couple of minutes of the debug log,
// the cached debug log is emitted (with the correct older timestamp).
//
// Guidance on what type of log to use
//
// Please see the confluence page for logging guidance:
// https://outreach-io.atlassian.net/wiki/spaces/EN/pages/699695766/Logging+Tracing+and+Metrics
//
package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/log/internal/entries"
)

// nolint:gochecknoglobals
var (
	// wrap stdout and stderr in sync writers to ensure that writes exceeding
	// PAGE_SIZE (4KB) are not interleaved.

	stdOut io.Writer = &syncWriter{w: os.Stdout}
	errOut io.Writer = &syncWriter{w: os.Stderr}

	dbgEntries = entries.New()
)

// Marshaler is the interface to be implemented by items that can be logged.
//
// The MarshalLog function will be called by the logger with the
// addField function provided. The implementation an add logging
// fields using this function. The field value can itself be another
// Marshaler instance, in which case the field names are concatenated
// with dot to indicate nesting.
type Marshaler interface {
	MarshalLog(addField func(field string, value interface{}))
}

type syncWriter struct {
	sync.Mutex
	w io.Writer
}

func (sw *syncWriter) Write(b []byte) (int, error) {
	sw.Lock()
	defer sw.Unlock()

	return sw.w.Write(b)
}

// SetOutput can be used to set the output for the module
// Note: this function should not be used in production code outside of service startup.
// SetOutput can be used for tests that need to redirect or filter logs
func SetOutput(w io.Writer) {
	stdOut = w
}

func Output() io.Writer {
	return stdOut
}

func Write(s string) {
	if _, err := fmt.Fprintln(stdOut, s); err != nil {
		fmt.Fprintln(errOut, err)
	}
}

// F is a map of fields used for logging:
//     log.Info(ctx, "request started", log.F{"start_time": time.Now()})
//
// When logging orgs, use events.Org:
//     ctx = log.WithOrg(ctx, events.Org{Org: "boo", GUID: "hoo"})
//
// When logging errors, use events.Err:
//     log.Error(ctx, "some failure", events.Err(err))
type F map[string]interface{}

// Set writes the field value into F.  If the value is a
// log.Marshaler, it recursively marshals that value into F.
func (f F) Set(field string, value interface{}) {
	if m, ok := value.(Marshaler); ok {
		m.MarshalLog(func(inner string, val interface{}) {
			f.Set(field+"."+inner, val)
		})
	} else if value != nil {
		if f["level"] == "FATAL" && strings.HasPrefix(field, "error.") {
			// if this is a FATAL, make room for the root call stack
			field = "error.cause." + field[6:]
		}
		f[field] = value
	}
}

// MarshalLog implements the Marshaler interface for F
func (f F) MarshalLog(addField func(field string, value interface{})) {
	for k, v := range f {
		addField(k, v)
	}
}

// Debug emits a log at DEBUG level but only if an error or fatal happens
// within 2min of this event
func Debug(ctx context.Context, message string, m ...Marshaler) {
	dbgEntries.Append(format(message, "DEBUG", time.Now(), app.Info(), m))
}

// Info emits a log at INFO level. This is not filtered and meant for non-debug information.
func Info(ctx context.Context, message string, m ...Marshaler) {
	s := format(message, "INFO", time.Now(), app.Info(), m)

	Write(s)
}

// Warn emits a log at WARN level. Warn logs are meant to be investigated if they reach high volumes.
func Warn(ctx context.Context, message string, m ...Marshaler) {
	s := format(message, "WARN", time.Now(), app.Info(), m)

	Write(s)
}

// Error emits a log at ERROR level.  Error logs must be investigated
func Error(ctx context.Context, message string, m ...Marshaler) {
	dbgEntries.Flush(Write)
	s := format(message, "ERROR", time.Now(), app.Info(), m)

	Write(s)
}

// Fatal emits a log at FATAL level and exits.  This is for catastrophic unrecoverable errors.
func Fatal(ctx context.Context, message string, m ...Marshaler) {
	dbgEntries.Flush(Write)
	s := format(message, "FATAL", time.Now(), app.Info(), m)

	Write(s)

	os.Exit(1)
}

func format(msg, level string, ts time.Time, fromContext Marshaler, mm Many) string {
	entry := F{"message": msg, "level": level, "@timestamp": ts.Format(time.RFC3339Nano)}

	fromContext.MarshalLog(entry.Set)
	mm.MarshalLog(entry.Set)

	if entry["level"] == "FATAL" {
		generateFatalFields(entry)
	}

	if len(entry) == 0 {
		return ""
	}

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(entry); err != nil {
		log.Fatal(err)
	}

	return strings.TrimSpace(b.String())
}

// Flush writes out all debug logs
func Flush(ctx context.Context) {
	dbgEntries.Flush(Write)
}

// Purge clears all debug logs without writing them out. This is useful to clear logs
// from a successful tests that we don't want output during a subsequent test
func Purge(ctx context.Context) {
	dbgEntries.Purge()
}

func generateFatalFields(entry F) {
	entry["error.kind"] = "fatal"
	if s, ok := entry["error.cause.error"].(string); ok {
		entry["error.error"] = "fatal occurred: " + s
	} else {
		entry["error.error"] = "fatal occurred"
	}
	entry["error.message"] = "fatal occurred"
	entry["error.stack"] = string(debug.Stack())
}
