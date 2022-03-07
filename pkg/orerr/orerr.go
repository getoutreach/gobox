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

type retryable struct {
	err error
}

// Retryable wraps err in a type which denotes that the originating process is retryable.
func Retryable(err error) *retryable { // nolint
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
