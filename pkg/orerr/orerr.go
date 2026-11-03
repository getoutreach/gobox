// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements outreach specific error utilities.

// Package orerr implements outreach specific error utilities.
package orerr

import (
	"errors"

	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

// A SentinelError is a constant which ought to be compared using errors.Is.
type SentinelError string

// Error returns s as a string.
func (s SentinelError) Error() string {
	return string(s)
}

// ShutdownError incidates the process is shutting down. An inner
// error may be provided via Err.
type ShutdownError struct {
	Err error
}

// Error implements the err interface.
func (e ShutdownError) Error() string {
	return "process has shutdown"
}

// Unwrap returns the inner error.
func (e ShutdownError) Unwrap() error {
	return e.Err
}

// LimitExceededError indicates some limit has exceeded. The actual
// limit that has exceeded is indicated via the Kind field. An inner
// error may be provided via Err.
type LimitExceededError struct {
	// Kind refers to the kind of rate whose limit has been exceeded.
	Kind string

	Err error
}

// Error implements the err interface.
func (e LimitExceededError) Error() string {
	return e.Kind + " limit exceeded"
}

// Unwrap returns the inner error.
func (e LimitExceededError) Unwrap() error {
	return e.Err
}

// New creates a new error wrapping all the error options onto it.
//
// For convenience, if err is nil, this function returns nil.
func New(err error, opts ...ErrOption) error {
	if err == nil {
		return nil
	}

	for _, opt := range opts {
		err = opt(err)
	}
	return err
}

// ErrOption just wraps an error.
type ErrOption func(err error) error

// WithInfo attaches the provided logs to the error.
//
// It is a functional option for use with New.
func WithInfo(info ...log.Marshaler) ErrOption {
	return func(err error) error {
		return Info(err, info...)
	}
}

// WithKind sets the error kind of the error, used as the value of the
// error.kind log/trace attribute (see observability/semconv/model/error/attributes.yaml)
// instead of the default "error".
//
// The kind should be a stable, low-cardinality identifier (e.g. "timeout" or
// "validation"), never an error message, ID or other unbounded value. gobox
// does not define kind values; they are owned by the caller. An empty kind is
// a no-op. If several kinds are attached, the outermost (last applied) wins.
// If the error also has a log.Marshaler emitting its own error.kind, the kind
// set here wins.
//
// It is a functional option for use with New.
func WithKind(kind string) ErrOption {
	return func(err error) error {
		return Kind(err, kind)
	}
}

// WithRetry marks the error as retryable.
//
// It is a functional option for use with New.
func WithRetry() ErrOption {
	return func(err error) error {
		return Retryable(err)
	}
}

// WithMeta attaches the provided grpc metadata to the error.
//
// It is a functional option for use with New.
func WithMeta(meta map[string]string) ErrOption {
	return func(err error) error {
		return Meta(err, meta)
	}
}

// WithStatus calls NewErrorStatus with the given code.
//
// It is a functional option for use with New.
func WithStatus(code statuscodes.StatusCode) ErrOption {
	return func(err error) error {
		return NewErrorStatus(err, code)
	}
}

// WithDetails calls NewErrorDetails with the given error details.
//
// It is a functional option for use with New.
func WithDetails(details ...ErrDetail) ErrOption {
	return func(err error) error {
		return NewErrDetails(err, details...)
	}
}

// Info adds extra logging info to an error.
func Info(err error, info ...log.Marshaler) error {
	return withInfo{err, log.Many(info)}
}

// withInfo just embeds error and log.Marshaler, so both interface are
// satisfied.
type withInfo struct {
	error
	log.Marshaler
}

// Unwrap returns the underlying error.
// This method is required by errors.Unwrap.
func (e withInfo) Unwrap() error {
	return e.error
}

// Kind attaches an error kind to err; see WithKind for the semantics of kind.
//
// It returns nil if err is nil, and err unchanged if kind is empty. The
// returned error implements Unwrap, and ErrorKind() string (which
// events.NewErrorInfo uses for error.kind).
func Kind(err error, kind string) error {
	if err == nil || kind == "" {
		return err
	}
	return &withKind{err: err, kind: kind}
}

// withKind attaches a kind to an error. It is transparent: Error() is the
// wrapped error's message.
type withKind struct {
	err  error
	kind string
}

// Error proxies the Error call to the underlying error.
func (e *withKind) Error() string { return e.err.Error() }

// Unwrap returns the underlying error.
// This method is required by errors.Unwrap.
func (e *withKind) Unwrap() error { return e.err }

// ErrorKind returns the kind attached to the error.
func (e *withKind) ErrorKind() string { return e.kind }

// MarshalLog proxies the MarshalLog call to the underlying error if it is a log.Marshaler.
func (e *withKind) MarshalLog(addField func(field string, value interface{})) {
	var m log.Marshaler
	if errors.As(e.err, &m) {
		m.MarshalLog(addField)
	}
}

type retryable struct {
	err error
}

// Retryable wraps err in a type which denotes that the originating process is retryable.
func Retryable(err error) *retryable { //nolint: revive // Why: retryable is meant to be hidden
	return &retryable{err}
}

// IsRetryable returns whether or not err is retryable.
func IsRetryable(err error) bool {
	var re *retryable
	return errors.As(err, &re)
}

// Error proxies the Error call to the underlying error.
func (r *retryable) Error() string {
	return r.err.Error()
}

// MarshalLog proxies the MarshalLog call to the underlying error if it is a log.Marshaler.
func (r *retryable) MarshalLog(addField func(field string, value interface{})) {
	var m log.Marshaler
	if errors.As(r.err, &m) {
		m.MarshalLog(addField)
	}
}

// Unwrap returns the underlying error.
// This method is required by errors.Unwrap.
func (r *retryable) Unwrap() error {
	return r.err
}

// IsOneOf returns true if the supplied error is identical to an error supplied
// in the remaining function error arguments
func IsOneOf(err error, errs ...error) bool {
	for _, e := range errs {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}

// Meta adds grpc metadata to an error.
func Meta(err error, meta map[string]string) error {
	return &withMeta{error: err, meta: meta}
}

type withMeta struct {
	error
	meta map[string]string
}

// Unwrap returns the underlying error.
// This method is required by errors.Unwrap.
func (e *withMeta) Unwrap() error {
	return e.error
}

// ExtractErrorMetadata returns any embedded grpc metadata in
func ExtractErrorMetadata(err error) map[string]string {
	var m *withMeta
	if errors.As(err, &m) {
		return m.meta
	}

	return map[string]string{}
}
