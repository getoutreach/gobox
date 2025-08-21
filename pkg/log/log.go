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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/callerinfo"
	"github.com/getoutreach/gobox/pkg/log/internal/entries"
	"github.com/getoutreach/gobox/pkg/olog"
	"go.opentelemetry.io/otel/trace"
)

// packageSourceInfoSkips lists the packages that we will skip when calculating caller info
var packageSourceInfoSkips = map[string]any{
	"github.com/getoutreach/gobox/pkg/log":   nil,
	"github.com/getoutreach/gobox/pkg/trace": nil,
}

// nolint:gochecknoglobals // Why: sets up overwritable writers
var (
	// wrap stdout and stderr in sync writers to ensure that writes exceeding
	// PAGE_SIZE (4KB) are not interleaved.

	stdOutLock           = new(sync.RWMutex)
	stdOut     io.Writer = &syncWriter{w: os.Stdout}
	errOut     io.Writer = &syncWriter{w: os.Stderr}

	// once to ensure we only initialize the slog.Logger once.
	once          = sync.Once{}
	slogLock      = sync.Mutex{}
	_, shouldSlog = os.LookupEnv("GOBOX_AS_SLOG_FACADE")
	// log is a structured logger instance.
	log *slog.Logger

	// dbgEntries is essentially a buffer of entries
	dbgEntries = entries.New()
)

// setupSlog initializes a global slogger
func setupSlog() {
	slogLock.Lock()
	defer slogLock.Unlock()
	log = olog.New()
}

// ShouldUseSlog returns true if slog facade should be used, checking both initialization and runtime
func ShouldUseSlog() bool {
	return shouldSlog
}

// SetShouldUseSlog modifies whether calls to log methods will use slog or the
// vintage custom outreach writer.
func SetShouldUseSlog(val bool) {
	slogLock.Lock()
	defer slogLock.Unlock()
	shouldSlog = val
	log = olog.New()
}

// Marshaler is the interface to be implemented by items that can be logged.
//
// The MarshalLog function will be called by the logger with the
// addField function provided. The implementation an add logging
// fields using this function. The field value can itself be another
// Marshaler instance, in which case the field names are concatenated
// with dot to indicate nesting.
type Marshaler = logf.Marshaler

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
	stdOutLock.Lock()
	defer stdOutLock.Unlock()

	if ShouldUseSlog() {
		olog.SetOutput(w)
		// Force re-initialization of the slog logger with the new output
		setupSlog()
	}
	stdOut = w
}

func Output() io.Writer {
	stdOutLock.RLock()
	defer stdOutLock.RUnlock()
	return stdOut
}

func Write(s string) {
	if _, err := fmt.Fprintln(Output(), s); err != nil {
		fmt.Fprintln(errOut, err)
	}
}

// F is a map of fields used for logging:
//
//	log.Info(ctx, "request started", log.F{"start_time": time.Now()})
//
// When logging errors, use events.Err:
//
//	log.Error(ctx, "some failure", events.Err(err))
type F = logf.F

// slogIt produces a slog structured log at the appropriate level.
func slogIt(ctx context.Context, lvl slog.Level, message string, m []Marshaler) {
	once.Do(setupSlog)
	log.LogAttrs(ctx, lvl, message, slogAttrs(m)...)
}

// Debug emits a log at DEBUG level but only if an error or fatal happens
// within 2min of this event
func Debug(ctx context.Context, message string, m ...Marshaler) {
	if ShouldUseSlog() {
		slogIt(ctx, slog.LevelDebug, message, m)
		return
	}
	dbgEntries.Append(format(ctx, message, "DEBUG", time.Now(), app.Info(), m))
}

// Info emits a log at INFO level. This is not filtered and meant for non-debug information.
func Info(ctx context.Context, message string, m ...Marshaler) {
	if ShouldUseSlog() {
		slogIt(ctx, slog.LevelInfo, message, m)
		return
	}
	s := format(ctx, message, "INFO", time.Now(), app.Info(), m)

	Write(s)
}

// Warn emits a log at WARN level. Warn logs are meant to be investigated if they reach high volumes.
func Warn(ctx context.Context, message string, m ...Marshaler) {
	if ShouldUseSlog() {
		slogIt(ctx, slog.LevelWarn, message, m)
		return
	}
	s := format(ctx, message, "WARN", time.Now(), app.Info(), m)

	Write(s)
}

// Error emits a log at ERROR level.  Error logs must be investigated
func Error(ctx context.Context, message string, m ...Marshaler) {
	if ShouldUseSlog() {
		slogIt(ctx, slog.LevelError, message, m)
		return
	}
	dbgEntries.Flush(Write)
	s := format(ctx, message, "ERROR", time.Now(), app.Info(), m)

	Write(s)
}

// Fatal emits a log at FATAL level and exits.  This is for catastrophic unrecoverable errors.
func Fatal(ctx context.Context, message string, m ...Marshaler) {
	if ShouldUseSlog() {
		slogIt(ctx, slog.LevelError, message, m)
		os.Exit(1)
		return
	}
	dbgEntries.Flush(Write)
	s := format(ctx, message, "FATAL", time.Now(), app.Info(), m)

	Write(s)

	os.Exit(1)
}

func format(ctx context.Context, msg, level string, ts time.Time, appInfo Marshaler, mm Many) string {
	entry := F{"message": msg, "level": level, "@timestamp": ts.Format(time.RFC3339Nano)}

	appInfo.MarshalLog(entry.Set)
	mm.MarshalLog(entry.Set)

	// cannot use gobox/trace due to circular import. just copy paste for simplicity
	if span := trace.SpanFromContext(ctx); span != nil && span.SpanContext().TraceID().IsValid() {
		entry.Set("traceID", span.SpanContext().TraceID().String())
	}

	addSource(entry)

	if entry["level"] == "FATAL" {
		generateFatalFields(entry)
	}

	if len(entry) == 0 {
		return ""
	}

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(entry); err != nil {
		// at this point we need to report the serialization error.
		// do it in a JSON object so parsers have a better chance of understanding it
		err = json.NewEncoder(&b).Encode(map[string]string{
			"message": fmt.Sprintf(
				"gobox/log: failed to JSON encode log entry %s; err=%v",
				msg,
				err,
			),
			"level":      "ERROR",
			"@timestamp": ts.Format(time.RFC3339Nano),
		})
		if err != nil {
			return ""
		}
	}

	return strings.TrimSpace(b.String())
}

func addSource(entry F) {
	// Attempt to map the caller of the log function into the "module" field for identifying if a service or a module
	// that the service is using is sending logs (costing money).
	// Skip 3 levels to start, and we may go further below (to skip log.With, other wrappers, etc.):
	// 1. addSource
	// 2. format
	// 3. log[Info/Error/etc.]
	skips := uint16(3)
	for {
		ci, err := callerinfo.GetCallerInfo(skips)
		if err != nil {
			entry["module"] = "error"
			break
		}

		// Specifically skip some internal packages (in the fixed map above) -- callers to these are responsible
		// for their logging, the skipped packages are just doing what they're told to do by the caller.
		if _, has := packageSourceInfoSkips[ci.Package]; has {
			skips++
			continue
		}

		if ci.Module != "" {
			entry["module"] = ci.Module
			if ci.ModuleVersion != "" {
				entry["modulever"] = ci.ModuleVersion
			}
		}
		break
	}
}

// Flush writes out all debug logs
func Flush(_ context.Context) {
	dbgEntries.Flush(Write)
}

// Purge clears all debug logs without writing them out. This is useful to clear logs
// from a successful tests that we don't want output during a subsequent test
func Purge(_ context.Context) {
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

// slogAttrs converts a logf.Many into a slice of slog.Attr
// nolint:gocyclo // Why: It's a big case statement that's hard to split.
func slogAttrs(arg logf.Many) []slog.Attr {
	// maps are unsorted, so we create a slice we can sort
	type keyValue struct {
		key   string
		value any
	}

	var kvs []keyValue

	logf.Marshal("", arg, func(key string, value any) {
		kvs = append(kvs, keyValue{key: key, value: value})
	})

	// Sort by key to ensure consistent ordering
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].key < kvs[j].key
	})

	res := make([]slog.Attr, 0, len(kvs))

	for _, kv := range kvs {
		switch v := kv.value.(type) {
		case bool:
			res = append(res, slog.Bool(kv.key, v))
		case int:
			res = append(res, slog.Int(kv.key, v))
		case int8:
			res = append(res, slog.Int64(kv.key, int64(v)))
		case int16:
			res = append(res, slog.Int64(kv.key, int64(v)))
		case int32:
			res = append(res, slog.Int64(kv.key, int64(v)))
		case int64:
			res = append(res, slog.Int64(kv.key, v))
		case uint8:
			res = append(res, slog.Int64(kv.key, int64(v)))
		case uint16:
			res = append(res, slog.Int64(kv.key, int64(v)))
		case uint32:
			res = append(res, slog.Int64(kv.key, int64(v)))
			// We can't guarantee that uint64 or uint can be safely casted
			// to int64.  We let them fall through to be strings.  :/
		case float32:
			f64 := float64(v)
			// Handle special float values that cause JSON encoding issues
			if math.IsInf(f64, 0) || math.IsNaN(f64) {
				res = append(res, slog.String(kv.key, fmt.Sprintf("%v", v)))
			} else {
				res = append(res, slog.Float64(kv.key, f64))
			}
		case float64:
			// Handle special float values that cause JSON encoding issues
			if math.IsInf(v, 0) || math.IsNaN(v) {
				res = append(res, slog.String(kv.key, fmt.Sprintf("%v", v)))
			} else {
				res = append(res, slog.Float64(kv.key, v))
			}
		case string:
			res = append(res, slog.String(kv.key, v))
		case time.Duration:
			res = append(res, slog.Duration(kv.key, v))
		case time.Time:
			res = append(res, slog.String(kv.key, v.Format(time.RFC3339)))
		case slog.Value:
			res = append(res, slog.Attr{Key: kv.key, Value: v})
		case error:
			// Handle errors explicitly - convert to string representation
			res = append(res, slog.String(kv.key, v.Error()))
		case Marshaler:
			// Handle log.Marshaler with recursion - create nested attributes
			nestedAttrs := slogAttrs([]Marshaler{v})
			if len(nestedAttrs) == 0 {
				// If marshaler produces no attributes, use string representation
				res = append(res, slog.String(kv.key, fmt.Sprintf("%v", v)))
			} else {
				// If marshaler produces multiple attributes, create a group
				res = append(
					res,
					slog.Attr{
						Key:   kv.key,
						Value: slog.GroupValue(nestedAttrs...),
					})
			}
		default:
			res = append(res, slog.String(kv.key, fmt.Sprintf("%v", v)))
		}
	}

	return res
}
