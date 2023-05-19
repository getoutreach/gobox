// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the call tracker implementation used by trace.

// Package call helps support tracking latency and other metrics for calls.
package call

import (
	"context"
	"sync"
	"time"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/metrics"
)

// Type tracks the call type.
type Type string

const (
	// TypeHTTP is a constant that denotes the call type being an HTTP
	// request.
	TypeHTTP Type = "http"

	// TypeGRPC is a constant that denotes the call type being a gRPC
	// request.
	TypeGRPC Type = "grpc"

	// TypeOutbound is a constant that denotes the call type being an
	// outbound request.
	TypeOutbound Type = "outbound"
)

// Info tracks information about an ongoing synchronous call.
type Info struct {
	Name string
	Type Type
	Kind metrics.CallKind
	Args []logf.Marshaler

	events.Times
	events.Durations

	ErrInfo *events.ErrorInfo

	mu sync.Mutex
}

// Start initializes info with the start time and some name.
func (info *Info) Start(ctx context.Context, name string) {
	info.Name = name
	if info.Kind == "" {
		info.Kind = metrics.CallKindInternal
	}
	info.Times.Started = time.Now()
}

// End records the finished time and updates durations.
func (info *Info) End(ctx context.Context) {
	info.Times.Finished = time.Now()
	info.Durations = *info.Times.Durations()
}

// ReportLatency reports the call latency via the metrics package based on the
// call Kind.  If the Kind is not one of HTTP, GRPC or Outbound, it does nothing.
func (info *Info) ReportLatency(ctx context.Context) {
	var err error
	if info.ErrInfo != nil {
		err = info.ErrInfo.RawError
	}

	name, kind := app.Info().Name, metrics.WithCallKind(info.Kind)
	switch info.Type {
	case TypeHTTP:
		metrics.ReportHTTPLatency(name, info.Name, info.ServiceSeconds, err, kind)
	case TypeGRPC:
		metrics.ReportGRPCLatency(name, info.Name, info.ServiceSeconds, err, kind)
	case TypeOutbound:
		metrics.ReportOutboundLatency(name, info.Name, info.ServiceSeconds, err, kind)
	default:
		// do not report anything.
	}
}

// AddArgs appends the provided logf.Marshalers to the Args slice.
func (info *Info) AddArgs(ctx context.Context, args ...logf.Marshaler) {
	info.mu.Lock()
	info.Args = append(info.Args, args...)
	info.mu.Unlock()
}

// ApplyOpts applies call Option functions to the call Info object.
// even if args are logf.Marshalers, but there might be some call.Options
// this is done intentionally to preserve compatibility of StartCall API
// and extend it with new functionality
func (info *Info) ApplyOpts(ctx context.Context, args ...logf.Marshaler) {
	for _, a := range args {
		if opt, ok := a.(Option); ok {
			opt(info)
		}
	}
}

// SetStatus updates the ErrInfo field based on the error.
func (info *Info) SetStatus(ctx context.Context, err error) {
	info.ErrInfo = events.NewErrorInfo(err)
}

// MarshalLog addes log.Marshaler support, logging most of the fields.
func (info *Info) MarshalLog(addField func(key string, value interface{})) {
	info.Times.MarshalLog(addField)
	info.Durations.MarshalLog(addField)
	logf.Many(info.Args).MarshalLog(addField)
	info.ErrInfo.MarshalLog(addField)
}

// Tracker helps manage a call info via the context.
type Tracker struct {
}

// StartCall creates a new call Info object and returns a new context
// where tracker.Info(ctx) will return the newly setup call Info object.
func (t *Tracker) StartCall(ctx context.Context, name string, args []logf.Marshaler) context.Context {
	var info Info
	info.Start(ctx, name)
	info.AddArgs(ctx, args...)
	info.ApplyOpts(ctx, args...)
	return context.WithValue(ctx, t, &info)
}

// Info returns the call Info object stashed in the context.
func (t *Tracker) Info(ctx context.Context) *Info {
	if v := ctx.Value(t); v != nil {
		return v.(*Info)
	}
	return nil
}

// EndCall is meant to be called in a defer abc.EndCall(ctx) pattern.
// It checks if there is a panic.  If so, it uses that to update the current
// call Info object.
// It calls info.End(ctx) before returning.
// It rethrows any panic.
func (t *Tracker) EndCall(ctx context.Context) {
	info := t.Info(ctx)
	if r := recover(); r != nil {
		info.ErrInfo = events.NewErrorInfoFromPanic(r)

		// rethrow at end of the function
		defer panic(r)
	}

	info.End(ctx)
}
