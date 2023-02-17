// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file integrates the logger with retryablehttp.LeveledLogger
package adapters

import (
	"context"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/hashicorp/go-retryablehttp"
)

// NewLeveledLogger returns a gobox/pkg/log logger that implements the
// retryablehttp.LeveledLogger interface.
func NewLeveledLogger(ctx context.Context) retryablehttp.LeveledLogger {
	return &leveledLogger{ctx}
}

// leveledLogger implements retryablehttp.LeveledLogger
type leveledLogger struct {
	ctx context.Context
}

// Debug wraps log.Debug()
func (l leveledLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Debug(l.ctx, msg, listToGoboxF(keysAndValues...))
}

// Error wraps log.Error()
func (l leveledLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Error(l.ctx, msg, listToGoboxF(keysAndValues...))
}

// Info wraps log.Info()
func (l leveledLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Info(l.ctx, msg, listToGoboxF(keysAndValues...))
}

// Warn wraps log.Warn()
func (l leveledLogger) Warn(msg string, keysAndValues ...interface{}) {
	log.Warn(l.ctx, msg, listToGoboxF(keysAndValues...))
}
