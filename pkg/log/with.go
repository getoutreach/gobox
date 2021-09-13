package log

import "context"

// With creates a logger that captures the marshaler arguments.
//
// All methods exposed by the logger automatically add the provided marshalers.
func With(m ...Marshaler) logger { //nolint: revive // logger is intentionally hidden.
	return logger{m}
}

// logger is intentionally not exported as this prevents logger from
// being tacked on to structs or passed as args to functions.  That
// pattern is not encouraged.
type logger struct {
	m []Marshaler
}

// Debug emits a log at DEBUG level but only if an error or fatal happens
// within 2min of this event
func (l logger) Debug(ctx context.Context, message string, m ...Marshaler) {
	Debug(ctx, message, append(m, l.m...)...)
}

// Info emits a log at INFO level. This is not filtered and meant for non-debug information.
func (l logger) Info(ctx context.Context, message string, m ...Marshaler) {
	Info(ctx, message, append(m, l.m...)...)
}

// Warn emits a log at WARN level. Warn logs are meant to be investigated if they reach high volumes.
func (l logger) Warn(ctx context.Context, message string, m ...Marshaler) {
	Warn(ctx, message, append(m, l.m...)...)
}

// Error emits a log at ERROR level.  Error logs must be investigated
func (l logger) Error(ctx context.Context, message string, m ...Marshaler) {
	Error(ctx, message, append(m, l.m...)...)
}

// Fatal emits a lot at FATAL level.  This is for catastrophic unrecoverable errors.
func (l logger) Fatal(ctx context.Context, message string, m ...Marshaler) {
	Fatal(ctx, message, append(m, l.m...)...)
}
