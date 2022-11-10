// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file integrates the logger with go-logr/logr

// Package adapters integrates the logger with go-logr/logr
package adapters

import (
	"context"
	"encoding/json"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/go-logr/logr"
)

// NewLogr returns a gobox/pkg/log logger that implements the
// logr.Logger interface.
//
// ! This should ONLY be used if the consumer doesn't support calling
// ! gobox/pkg/log directly.
func NewLogrLogger(ctx context.Context) logr.Logger {
	logger := &logrLogger{ctx, []log.Marshaler{}}
	return logr.New(logger)
}

// logrLogger implements logr.Logger
type logrLogger struct {
	ctx context.Context

	// existingMarshalers are marshalers that should
	// be always set when calling any function. This is used
	// to support passing loggers.
	existingMarshalers []log.Marshaler
}

// Init initializes the logger. The gobox logger doesn't need
// to be intialized so this is a NOOP
func (l *logrLogger) Init(ri logr.RuntimeInfo) {}

// Enabled returns if this logger is enabled or not. Because this
// logger does not support different levels this function always
// returns true.
func (l *logrLogger) Enabled(level int) bool {
	return true
}

// listToGoboxF converts a list of arbitrary length to key/value pairs.
// Based on:
// https://github.com/bombsimon/logrusr/blob/6296cfced8667be48746ac95a295359bbb08bb25/logrusr.go#L158
func listToGoboxF(keysAndValues ...interface{}) log.F {
	f := make(log.F)

	// Skip all fields if it's not an even length list.
	if len(keysAndValues)%2 != 0 {
		return f
	}

	for i := 0; i < len(keysAndValues); i += 2 {
		k, v := keysAndValues[i], keysAndValues[i+1]

		if s, ok := k.(string); ok {
			// Try to avoid marshaling known types.
			switch vVal := v.(type) {
			case int, int8, int16, int32, int64,
				uint, uint8, uint16, uint32, uint64,
				float32, float64, complex64, complex128,
				string, bool:
				f[s] = vVal

			case []byte:
				f[s] = string(vVal)

			default:
				//nolint:errcheck // Why: we're OK
				j, _ := json.Marshal(vVal)
				f[s] = string(j)
			}
		}
	}

	return f
}

// Errors wraps log.Error()
func (l *logrLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Error(l.ctx, msg, append(append(l.existingMarshalers, events.Err(err)), listToGoboxF(keysAndValues...))...)
}

// Info wraps log.Info()
func (l *logrLogger) Info(level int, msg string, keysAndValues ...interface{}) {
	log.Info(l.ctx, msg, append(l.existingMarshalers, listToGoboxF(keysAndValues...))...)
}

// WithName sets the name of this logger. Gobox no-ops this because we only
// support app.Name
func (l *logrLogger) WithName(name string) logr.LogSink {
	return l
}

// WithValues returns a copy of the current logger with the provided
// key/value pairs being added to all sub-sequent calls
// of error/info/etc.
func (l *logrLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	newLogger := &logrLogger{
		existingMarshalers: append(l.existingMarshalers, listToGoboxF(keysAndValues...)),
	}
	return newLogger
}
