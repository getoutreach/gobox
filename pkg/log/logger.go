package log

import "context"

// Logger struct encapsulates logging impl in one place. Its ptr can be passed aroudn to libraries that need to perform
// logging, instead of using the static log.Info/Other methods, to ensure the libraries include the fields from
// the calling component into the lib + add their own fields on top.
// Use New method below to create it.
type Logger struct {
	m []Marshaler
}

// New creates a new Logger with fields. The Logger field can be passed around to libraries that perform logging
// on behalf of the parent/caller, making sure their logs incorporate caller's fields.
//
// Implementation note: returning a pointer of the Logger here to enable future expansion of the Logger.
func New(m ...Marshaler) *Logger {
	return &Logger{m}
}

// Debug emits a log at DEBUG level but only if an error or fatal happens
// within 2min of this event.
func (l *Logger) Debug(ctx context.Context, message string, m ...Marshaler) {
	Debug(ctx, message, append(m, l.m...)...)
}

// Info emits a log at INFO level. This is not filtered and meant for non-debug information.
func (l *Logger) Info(ctx context.Context, message string, m ...Marshaler) {
	Info(ctx, message, append(m, l.m...)...)
}

// Warn emits a log at WARN level. Warn logs are meant to be investigated if they reach high volumes.
func (l *Logger) Warn(ctx context.Context, message string, m ...Marshaler) {
	Warn(ctx, message, append(m, l.m...)...)
}

// Error emits a log at ERROR level. Error logs must be investigated.
func (l *Logger) Error(ctx context.Context, message string, m ...Marshaler) {
	Error(ctx, message, append(m, l.m...)...)
}

// Fatal emits a lot at FATAL level.  This is for catastrophic unrecoverable errors.
func (l *Logger) Fatal(ctx context.Context, message string, m ...Marshaler) {
	Fatal(ctx, message, append(m, l.m...)...)
}

// With creates a child Logger implementation with extra fields.
// Imprtant: it captures the marshallers of the current logger (instead of keeping parent ref).
func (l *Logger) With(m ...Marshaler) *Logger {
	// ensure new slice of marshallers (append does not guarantee that)
	mClone := make([]Marshaler, len(m)+len(l.m))
	mClone = append(mClone, l.m...)
	mClone = append(mClone, m...)
	return New(mClone...)
}
