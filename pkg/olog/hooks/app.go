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
// nolint:gocritic // Why: this signature is inline with the slog pkg handler interface
func AppInfo(ctx context.Context, r slog.Record) ([]slog.Attr, error) {
	info := app.Info()
	if info == nil {
		return []slog.Attr{}, nil
	}

	return []slog.Attr{
		// Manually assign the LogValue to an attribute. The slog.Group
		// func doesn't really take an already existing GroupValue as
		// Values are more meant to be used directly in log calls.
		{Key: "app", Value: info.LogValue()},
	}, nil
}
