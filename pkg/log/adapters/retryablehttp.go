// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file integrates the logger with retryablehttp.LeveledLogger
package adapters

import (
	"context"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/hashicorp/go-retryablehttp"
)

// NewRetryableHTTPLogger returns a gobox/pkg/log logger that implements the
// retryablehttp.LeveledLogger interface.
func NewRetryableHTTPLogger(ctx context.Context) retryablehttp.LeveledLogger {
	return &retryableHTTPLogger{ctx}
}

// retryableHTTPLogger implements retryablehttp.LeveledLogger
type retryableHTTPLogger struct {
	ctx context.Context
}

// Debug wraps log.Debug()
func (l retryableHTTPLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Debug(l.ctx, msg, listToGoboxF(keysAndValues...))
}

// Error wraps log.Error()
func (l retryableHTTPLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Error(l.ctx, msg, listToGoboxF(keysAndValues...))
}

// Info wraps log.Info()
func (l retryableHTTPLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Info(l.ctx, msg, listToGoboxF(keysAndValues...))
}

// Warn wraps log.Warn()
func (l retryableHTTPLogger) Warn(msg string, keysAndValues ...interface{}) {
	log.Warn(l.ctx, msg, listToGoboxF(keysAndValues...))
}
