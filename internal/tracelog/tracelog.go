// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the singleton trace implementation
// with logging added.

// Package tracelog provides the singleton tracer for use with /pkg/trace.
package tracelog

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/internal/logf"
	"github.com/getoutreach/gobox/internal/tracer"

	// TODO: remove this dependency.
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/getoutreach/gobox/pkg/orerr"
	"github.com/getoutreach/gobox/pkg/statuscodes"
)

// nolint:gochecknoglobals Why: we need a singleton default tracer for /pkg/trace.
var Tracer = tracer.New(
	tracer.WithCallTracking(),
	tracer.WithHoneycomb(),
	tracer.WithProvider(&latencyReporter{}),
	tracer.WithProvider(&callLogger{}),
)

// base implments a provider that can be embedded in other providers.
type base struct{}

func (base) StartTrace(ctx context.Context, name string, headers map[string][]string) context.Context {
	return ctx
}

func (base) StartSpan(ctx context.Context, name string, spanType tracer.SpanType, args logf.Marshaler) context.Context {
	return ctx
}

func (base) Close(ctx context.Context)                                                      {}
func (base) EndTrace(ctx context.Context)                                                   {}
func (base) AddTraceInfo(ctx context.Context, info logf.Marshaler)                          {}
func (base) EndSpan(ctx context.Context, spanType tracer.SpanType)                          {}
func (base) AddSpanInfo(ctx context.Context, spanType tracer.SpanType, info logf.Marshaler) {}
func (base) SetSpanStatus(ctx context.Context, spanType tracer.SpanType, err error)         {}
func (base) CurrentInfo(ctx context.Context, info *tracer.Info)                             {}
func (base) CurrentHeaders(ctx context.Context, headers map[string][]string)                {}

type callLogger struct {
	Tracer *tracer.Tracer
	base
}

func (c *callLogger) Init(ctx context.Context, t *tracer.Tracer, name string) error {
	c.Tracer = t
	return nil
}

func (c *callLogger) StartSpan(ctx context.Context, name string, spanType tracer.SpanType, args logf.Marshaler) context.Context {
	if c.Tracer == nil || !spanType.IsCall() {
		return ctx
	}

	// TODO: distinguish between inbound and outbound calls
	log.Debug(ctx, fmt.Sprintf("calling: %s", name), args)
	return ctx
}

func (c *callLogger) EndSpan(ctx context.Context, spanType tracer.SpanType) {
	if c.Tracer == nil || !spanType.IsCall() {
		return
	}

	info := c.Tracer.Info(ctx)
	if info.Call == nil {
		return
	}

	if info.Call.ErrInfo == nil {
		log.Info(ctx, info.Call.Name, info, traceEventMarker{})
		return
	}

	switch category := orerr.ExtractErrorStatusCategory(info.Call.ErrInfo.RawError); category {
	case statuscodes.CategoryClientError:
		log.Warn(ctx, info.Call.Name, info, traceEventMarker{})
	case statuscodes.CategoryServerError:
		log.Error(ctx, info.Call.Name, info, traceEventMarker{})
	case statuscodes.CategoryOK: // just in case if someone will return non-nil error on success
		log.Info(ctx, info.Call.Name, info, traceEventMarker{})
	}
}

type traceEventMarker struct{}

func (traceEventMarker) MarshalLog(addField func(k string, v interface{})) {
	addField("event_name", "trace")
}

type latencyReporter struct {
	Tracer *tracer.Tracer
	base
}

func (l *latencyReporter) Init(ctx context.Context, t *tracer.Tracer, name string) error {
	l.Tracer = t
	return nil
}

func (l *latencyReporter) EndSpan(ctx context.Context, spanType tracer.SpanType) {
	if l.Tracer == nil || !spanType.IsCall() {
		return
	}

	if info := l.Tracer.Info(ctx); info.Call != nil {
		// TODO: hooke up to metrics directly here and remove
		// it from internal/call
		info.Call.ReportLatency(ctx)
	}
}
