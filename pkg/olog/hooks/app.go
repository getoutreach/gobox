// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Implements a hook based pattern for automatically
// adding data to logs before the record is written.

package hooks

import (
	"context"
	"log/slog"

	"github.com/getoutreach/gobox/pkg/app"
)

// AppInfo provides a log hook which extracts and returns the gobox/pkg/app.Data
// as a nested attribute on log record.
func AppInfo(ctx context.Context, r slog.Record) ([]slog.Attr, error) {
	info := app.Info()
	if info == nil {
		return []slog.Attr{}, nil
	}

	return []slog.Attr{slog.Any("app", info)}, nil
}
