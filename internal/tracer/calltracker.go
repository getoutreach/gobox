// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the call tracking implementation.

package tracer

import (
	"context"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/internal/logf"
)

// WithCallTracking adds the call tracking tracer.
func WithCallTracking() Option {
	return WithProvider(&callTracker{})
}

type callTracker struct {
	Tracer *Tracer
	call.Tracker
}

func (c *callTracker) Init(ctx context.Context, t *Tracer, serviceName string) error {
	c.Tracer = t
	return nil
}

func (c *callTracker) Close(ctx context.Context) {
}

func (c *callTracker) StartTrace(ctx context.Context, name string, headers map[string][]string) context.Context {
	return ctx
}

func (c *callTracker) EndTrace(ctx context.Context) {}

func (c *callTracker) AddTraceInfo(ctx context.Context, info logf.Marshaler) {}

func (c *callTracker) StartSpan(ctx context.Context, name string, spanType SpanType, args logf.Marshaler) context.Context {
	if !spanType.IsCall() {
		return ctx
	}
	return c.Tracker.StartCall(ctx, name, []logf.Marshaler{args})
}

func (c *callTracker) EndSpan(ctx context.Context, spanType SpanType) {
	if spanType.IsCall() {
		c.Tracker.EndCall(ctx)
	}
}

func (c *callTracker) AddSpanInfo(ctx context.Context, spanType SpanType, arg logf.Marshaler) {
	if spanType.IsCall() {
		if info := c.Tracker.Info(ctx); info != nil {
			info.AddArgs(ctx, arg)
		}
	}
}

func (c *callTracker) SetSpanStatus(ctx context.Context, spanType SpanType, err error) {
	if spanType.IsCall() {
		if info := c.Tracker.Info(ctx); info != nil {
			info.SetStatus(ctx, err)
		}
	}
}

func (c *callTracker) CurrentInfo(ctx context.Context, info *Info) {
	info.Call = c.Tracker.Info(ctx)
}

func (c *callTracker) CurrentHeaders(ctx context.Context, headers map[string][]string) {}
