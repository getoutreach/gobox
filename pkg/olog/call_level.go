// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Exposes per-call-name level overrides to the trace package.

package olog

import "log/slog"

// CallLevel returns the configured level override for the traced call
// with the given name, if any.
func CallLevel(name string) (slog.Level, bool) {
	return globalLevelRegistry.GetCall(name)
}
