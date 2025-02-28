// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides a standard means for go logging

// Package log implements standard go logging
//
// For logging:
//
//	log.Info(ctx, "message", log.F{field: 42})
//	log.Error(...)
//	log.Debug(...)
//	log.Fatal(...)
//
// By default, log.Debug is not emitted but instead it is cached. If
// a higher event arrives within a couple of minutes of the debug log,
// the cached debug log is emitted (with the correct older timestamp).
//
// # Guidance on what type of log to use
//
// Please see the confluence page for logging guidance:
// https://outreach-io.atlassian.net/wiki/spaces/EN/pages/699695766/Logging+Tracing+and+Metrics
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/log/internal/entries"
	"github.com/getoutreach/gobox/pkg/olog"
)

// nolint:gochecknoglobals // Why: sets up overwritable writers
var (
	slogger = olog.New()

	dbgEntries = entries.New()
)

// Marshaler is the interface to be implemented by items that can be logged.
//
// The MarshalLog function will be called by the logger with the
// addField function provided. The implementation an add logging
// fields using this function. The field value can itself be another
// Marshaler instance, in which case the field names are concatenated
// with dot to indicate nesting.
type Marshaler = logf.Marshaler

// SetOutput can be used to set the output for the module
// Note: this function should not be used in production code outside of service startup.
// SetOutput can be used for tests that need to redirect or filter logs
func SetOutput(w io.Writer) {
	olog.SetOutput(w)
}

// F is a map of fields used for logging:
//
//	log.Info(ctx, "request started", log.F{"start_time": time.Now()})
//
// When logging errors, use events.Err:
//
//	log.Error(ctx, "some failure", events.Err(err))
type F = logf.F

// Debug emits a log at DEBUG level
func Debug(ctx context.Context, message string, m ...Marshaler) {
	attrs := format(app.Info(), m)
	slogger.LogAttrs(ctx, slog.LevelDebug, message, attrs...)
}

func Info(ctx context.Context, message string, m ...Marshaler) {
	attrs := format(app.Info(), m)
	slogger.LogAttrs(ctx, slog.LevelInfo, message, attrs...)
}

// Warn emits a log at WARN level. Warn logs are meant to be investigated if they reach high volumes.
func Warn(ctx context.Context, message string, m ...Marshaler) {
	attrs := format(app.Info(), m)
	slogger.LogAttrs(ctx, slog.LevelWarn, message, attrs...)
}

// Error emits a log at ERROR level.  Error logs must be investigated
func Error(ctx context.Context, message string, m ...Marshaler) {
	attrs := format(app.Info(), m)
	slogger.LogAttrs(ctx, slog.LevelError, message, attrs...)
}

// Fatal emits a log at FATAL level and exits.  This is for catastrophic unrecoverable errors.
// Deprecated: don't use this. os.Exit means you cannot clean up any resources or ensure any buffers are flushed.
func Fatal(ctx context.Context, message string, m ...Marshaler) {
	attrs := format(app.Info(), m)
	// todo: make a custom level for Fatal
	slogger.LogAttrs(ctx, slog.LevelError, message, attrs...)

	os.Exit(1)
}

// format formats fields into a slog.Attr slice
func format(appInfo Marshaler, mm Many) []slog.Attr {
	entry := F{}

	// todo: can we extract appInfo from env at export time
	appInfo.MarshalLog(entry.Set)
	mm.MarshalLog(entry.Set)

	if entry["level"] == "FATAL" {
		generateFatalFields(entry)
	}

	if len(entry) == 0 {
		return nil
	}

	return marshalToKeyValue(entry)
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

// nolint:gocyclo // Why: It's a big case statement that's hard to split.
func marshalToKeyValue(arg Marshaler) []slog.Attr {
	res := []slog.Attr{}

	logf.Marshal("", arg, func(key string, value any) {
		switch v := value.(type) {
		case bool:
			res = append(res, slog.Bool(key, v))
		case int:
			res = append(res, slog.Int(key, v))
		case int8:
			res = append(res, slog.Int64(key, int64(v)))
		case int16:
			res = append(res, slog.Int64(key, int64(v)))
		case int32:
			res = append(res, slog.Int64(key, int64(v)))
		case int64:
			res = append(res, slog.Int64(key, v))
		case uint8:
			res = append(res, slog.Int64(key, int64(v)))
		case uint16:
			res = append(res, slog.Int64(key, int64(v)))
		case uint32:
			res = append(res, slog.Int64(key, int64(v)))
			// We can't guarantee that uint64 or uint can be safely casted
			// to int64.  We let them fall through to be strings.  :/
		case float32:
			res = append(res, slog.Float64(key, float64(v)))
		case float64:
			res = append(res, slog.Float64(key, v))
		case string:
			res = append(res, slog.String(key, v))
		case time.Duration:
			res = append(res, slog.Duration(key, v))
		case time.Time:
			// This is a compromise.  OTel seems to
			// prefer UNIX epoch milliseconds, while
			// Honeycomb says it accepts UNIX epoch
			// seconds.  Honeycomb also has a function to
			// convert RFC3339 timestamps to epoch.
			//
			// We figure RFC3339 is unambiguously a
			// timestamp and expect most systems can
			// deal with it accordingly.  Magic ints
			// or floats without units attached would
			// be harder to interpret.
			res = append(res, slog.String(key, v.Format(time.RFC3339Nano)))
		default:
			res = append(res, slog.String(key, fmt.Sprintf("%v", v)))
		}
	})

	return res
}
