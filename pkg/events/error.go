// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides capabilities for handling error events

package events

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/getoutreach/gobox/pkg/caller"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/pkg/errors"
)

// ErrorInfo tracks the error info for logging purposes.
// Log tags are omitted to do custom MarshalLog handling on Stack.
type ErrorInfo struct {
	RawError error
	Kind     string
	Error    string
	Message  string
	Stack    []string
	Cause    log.Marshaler
	Custom   log.Marshaler
}

// LoggableError is the minimum required to make an error loggable.
func LoggableError(err error) log.Marshaler {
	return &loggableError{error: err}
}

// loggableError is a wrapper around an error that provides a custom MarshalLog
type loggableError struct {
	error
}

// MarshalLog implements log.Marshaler
func (l *loggableError) MarshalLog(addField func(field string, value interface{})) {
	addField("error", l.Error())
}

func (e *ErrorInfo) MarshalLog(addField func(key string, value interface{})) {
	if e == nil {
		return
	}
	addField("error.kind", e.Kind)
	addField("error.error", e.Error)
	addField("error.message", e.Message)
	if len(e.Stack) > 0 {
		addField("error.stack", strings.Join(e.Stack, "\n\t"))
	}
	if e.Cause != nil {
		addField("error.cause", e.Cause)
	}
	if e.Custom != nil {
		e.Custom.MarshalLog(addField)
	}
}

func (e *ErrorInfo) pureStack() bool {
	return e.Message == "" && len(e.Stack) > 0 && e.Custom == nil
}

// nestedErrorInfo is about the same as ErrorInfo but it does not tack
// on the error. prefix.
// Log tags are omitted to do custom MarshalLog handling on Stack.
type nestedErrorInfo struct {
	RawError error
	Kind     string
	Message  string
	Stack    []string
	Cause    *nestedErrorInfo
	Custom   log.Marshaler
}

func (n *nestedErrorInfo) MarshalLog(addField func(key string, value interface{})) {
	if n == nil {
		return
	}
	addField("kind", n.Kind)
	addField("message", n.Message)
	if len(n.Stack) > 0 {
		addField("stack", strings.Join(n.Stack, "\n\t"))
	}
	if n.Cause != nil {
		addField("cause", n.Cause)
	}
	if n.Custom != nil {
		n.Custom.MarshalLog(addField)
	}
}

func (n *nestedErrorInfo) pureMessage() bool {
	return n.Message != "" && len(n.Stack) == 0 && n.Custom == nil
}

func (n *nestedErrorInfo) pureStack() bool {
	return n.Message == "" && len(n.Stack) > 0 && n.Custom == nil
}

// NewErrorInfoFromPanic converts the panic result into an appropriate
// error info for logging
func NewErrorInfoFromPanic(r interface{}) *ErrorInfo {
	if r == nil {
		return nil
	}

	if err, ok := r.(error); ok {
		return NewErrorInfo(err)
	}

	result := NewErrorInfo(errors.Errorf("%v", r))
	result.Kind = "panic"
	return result
}

// Err is a convenience method for logging. It lazily yields the result of
// NewErrorInfo when logged, and caches it for future use.
func Err(err error) *LazyErrInfo {
	return &LazyErrInfo{err: err}
}

// LazyErrInfo holds an unserialized error and marshals it on-demand.
type LazyErrInfo struct {
	err  error
	info *ErrorInfo
	once sync.Once
}

func (l *LazyErrInfo) ErrorInfo() *ErrorInfo {
	l.once.Do(func() {
		l.info = NewErrorInfo(l.err)
	})
	return l.info
}

func (l *LazyErrInfo) MarshalLog(addField func(field string, value interface{})) {
	l.ErrorInfo().MarshalLog(addField)
}

// NewErrorInfo converts an error into ErrorInfo meant for logging.
//
// In the case of errors wrapped with github.com/pkg/errors.Wrap, NewErrorInfo
// will attempt to collapse (message, stack) pairs within the error stack into
// A single level of the error.
func NewErrorInfo(err error) *ErrorInfo {
	if err == nil {
		return nil
	}
	custom, _ := err.(log.Marshaler) //nolint:errorlint // Why: causes unwrap support which needs to be thought about
	info := ErrorInfo{
		RawError: err,
		Kind:     "error",
		Error:    err.Error(),
		Message:  errMessage(err),
		Stack:    errStack(err),
		Custom:   custom,
	}
	// obtain nested error, collapsing upward if possible.
	if cause := nestInfo(errors.Unwrap(err)); cause != nil {
		if info.pureStack() && cause.pureMessage() {
			info.Message = cause.Message
			info.Cause = cause.Cause
		} else {
			info.Cause = cause
		}
	}
	return &info
}

func nestInfo(err error) *nestedErrorInfo {
	if err == nil {
		return nil
	}
	custom, _ := err.(log.Marshaler) //nolint:errorlint // Why: causes unwrap support which needs to be thought about
	info := nestedErrorInfo{
		RawError: err,
		Kind:     "cause",
		Message:  errMessage(err),
		Stack:    errStack(err),
		Custom:   custom,
	}
	// obtain nested error, collapsing upward if possible.
	if cause := nestInfo(errors.Unwrap(err)); cause != nil {
		if info.pureStack() && cause.pureMessage() {
			info.Message = cause.Message
			info.Cause = cause.Cause
		} else {
			info.Cause = cause
		}
	}
	return &info
}

func errMessage(err error) string {
	full := err.Error()
	var sub string
	if err = errors.Unwrap(err); err != nil {
		sub = err.Error()
	}
	return strings.TrimSuffix(strings.TrimSuffix(full, sub), ": ")
}

func errStack(err error) []string {
	// github.com/pkg/errors implements the Tracer interface
	// https://godoc.org/github.com/pkg/errors#hdr-Retrieving_the_stack_trace_of_an_error_or_wrapper
	type tracer interface {
		StackTrace() errors.StackTrace
	}

	t, ok := err.(tracer) //nolint:errorlint // Why: causes unwrap support which needs to be thought about
	if !ok {
		return nil
	}

	var b strings.Builder
	var stack []string

	for _, frame := range t.StackTrace() {
		if n, err := writeFrame(&b, frame); err == nil && n > 0 {
			stack = append(stack, b.String())
			b.Reset()
		}
	}
	return trimRuntime(stack)
}

func trimRuntime(stack []string) []string {
	for i := range stack {
		end := len(stack) - 1 - i
		if !strings.HasPrefix(stack[end], "runtime.") {
			return stack[:end]
		}
	}
	return stack
}

func writeFrame(w io.Writer, frame errors.Frame) (n int, err error) {
	// due to being acquired by runtime.Callers, frame = (pc + 1)
	file, line, name := caller.FileLineNameForPC(uintptr(frame) - 1)
	return fmt.Fprintf(w, "%s:%d `%s`", file, line, name)
}
