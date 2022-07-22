package log

import (
	"context"
)

type logContextMarshallerType int

const logContextMarshallerSingleton logContextMarshallerType = iota

type ContextFilter interface {
	// Process prepares the incoming marshallers for use in the nested library call.
	// This method always receives a non-nil marshaller.
	Process(in Marshaler) Marshaler
}

// NewContext creates a new Context to store fields that should be attached to all logs.
// This method is equivalent to NewFilteredContext with the CaptureAll filter.
// CaptureAll filter immediately invokes MarshalLog on all args to reduce risk of hard to
// debug issues and arbitrary code running during logging.
func NewContext(ctx context.Context, args ...Marshaler) context.Context {
	return NewFilteredContext(ctx, CaptureAll{}, args...)
}

// NewFilteredContext creates a new Context to store fields that should be attached to all logs. It allows
// a custom ContextFilter to filter out marshallers (fields) based on caller needs.
// The following built-in filters are available: CaptureAll, CaptureSpecific, PassThough.
func NewFilteredContext(ctx context.Context, filter ContextFilter, args ...Marshaler) context.Context {
	m := fromContext(ctx)
	if m != nil && filter != nil {
		m = filter.Process(m)
	}

	// note: the result of the Process can be nil too
	if m == nil {
		m = Many(args)
	} else if len(args) > 0 {
		// merge inherited with args (while keeping the inherited first, to preserve order of addfield calls)
		tmp := make([]Marshaler, len(args)+1)
		tmp = append(tmp, m)
		tmp = append(tmp, args...)
		m = Many(tmp)
	}
	// else just leave m as is

	return context.WithValue(ctx, logContextMarshallerSingleton, m)
}

// fromContext extracts the context marshaller or nil if none present
func fromContext(ctx context.Context) Marshaler {
	current := ctx.Value(logContextMarshallerSingleton)
	if current == nil {
		return nil
	}
	return current.(Marshaler)
}

// Built-in ContextFilter types

// CaptureAll is a ContextFilter similar to PassThough, except that it immediately captures all the fields that input
// marshaller has to provide into a captured log.F instance and creates the child context with the captured map.
// This filter is useful when invoking async method from an context that has 'marshallers' that provide dynamic values.
// Since the nested method can continue async with the same context chain, CaptureAll makes sure that the nested method
// keeps using same 'captured' state during its async execution.
//
// This is also the default filter.
type CaptureAll struct{}

// Process immediately expands all incoming marshaller values and returns the captured map.
func (CaptureAll) Process(in Marshaler) Marshaler {
	captured := make(F)
	in.MarshalLog(captured.Set)
	return captured
}

// PassThrough is a ContextFilter that passes the input marshaller as is between context-aware method calls.
// Do not use this filter on methods that can execute asynchroniously.
type PassThrough struct{}

// Process returns input marshaller As Is. Do not use PassThrough on async method invocations.
func (PassThrough) Process(in Marshaler) Marshaler {
	return in
}

// IgnoreAll is a ContextFilter that prevents input marshallers from being used in inherited context.
type IgnoreAll struct{}

// Process ignores input and returns nil
func (IgnoreAll) Process(Marshaler) Marshaler {
	return nil
}

// CaptureSpecific in a ContextFilter that captures only specific Keys, ignoring the rest.
type CaptureSpecific struct {
	Keys []string
}

// Process immediately expands the incoming marshaller fields and captures only those in the Keys.
func (f CaptureSpecific) Process(in Marshaler) Marshaler {
	captured := make(F)
	allowed := make(map[string]bool)
	for _, k := range f.Keys {
		allowed[k] = true
	}
	in.MarshalLog(func(field string, value interface{}) {
		if allowed[field] {
			captured.Set(field, value)
		}
	})
	return captured
}
