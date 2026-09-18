// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Provides a log call site in a distinct package so tests can
// verify per-caller-package log level routing.

// Package callerpkg emits logs from its own package path.
package callerpkg

import (
	"context"

	"github.com/getoutreach/gobox/pkg/log"
)

// Path is the package path of this package as seen by the level registry.
const Path = "github.com/getoutreach/gobox/pkg/log/internal/callerpkg"

// EmitDebug emits a DEBUG log from this package.
func EmitDebug(ctx context.Context, message string) {
	log.Debug(ctx, message)
}
