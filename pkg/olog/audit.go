// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Contains a strongly-typed logger for audit logs using
// slog.
//
// Note: This is an EXAMPLE of an extension to the slog.Logger, it is
// not meant to actually be used.

package olog

import "log/slog"

// Audit returns an audit logger using the underlying slog.Logger for
// writing audit logs.
//
// Example usage (chaining):
//
//	olog.Audit(logger).Log(&AuditLog{...})
//
// Example usage (storing):
//
//	alog := olog.Audit(logger)
//	alog.Log(&AuditLog{...})
func Audit(log *slog.Logger) *AuditLogger {
	return &AuditLogger{log}
}

// AuditLogger implements a wrapper ontop of the provided slog.Logger
// that writes structured audit logs.
type AuditLogger struct{ *slog.Logger }

// Log writes an audit log to the underlying logger.
func (a *AuditLogger) Log(entry *AuditLog) {
	// write the audit log to the logger.
	// TODO(jaredallard): Convert the AuditLog into slog.Attr.
	a.Info("audit log", "audit", entry)
}

// AuditLog contains information used for auditing changes to specific
// resources.
//
// While AuditLog can be logged using any slog.Logger, it is strongly
// recommended that AuditLogger be used instead to ensure that logs are
// handled appropriately.
type AuditLog struct {
	// Fields for an audit log go here.
}

// LogValue implements slog.LogValuer for AuditLog. This allows logging
// an AuditLog using a slog.Logger.
func (a *AuditLog) LogValue() slog.Value {
	return slog.GroupValue()
}
