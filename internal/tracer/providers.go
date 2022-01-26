// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the honeycomb trace implementation.

// Package tracer implements the core honeycomb tracer.
package tracer

// Note: this package does not depend on pkg/log or pkg/trace.
import (
	"context"

	"github.com/getoutreach/gobox/internal/logf"
)

// provider specifies the generic provider implementation type.
type provider interface {
	Init(ctx context.Context, t *Tracer, name string) error
	Close(ctx context.Context)

	StartTrace(ctx context.Context, name string, headers map[string][]string) context.Context
	EndTrace(ctx context.Context)
	AddTraceInfo(ctx context.Context, info logf.Marshaler)

	StartSpan(ctx context.Context, name string, spanType SpanType, args logf.Marshaler) context.Context
	EndSpan(ctx context.Context, spanType SpanType)
	AddSpanInfo(ctx context.Context, spanType SpanType, info logf.Marshaler)
	SetSpanStatus(ctx context.Context, spanType SpanType, err error)

	CurrentInfo(ctx context.Context, info *Info)
	CurrentHeaders(ctx context.Context, headers map[string][]string)
}

// providers implements the provider interface over a collection of providers.
type providers []provider

func (px providers) Init(ctx context.Context, t *Tracer, name string) error {
	for idx, p := range px {
		err := p.Init(ctx, t, name)
		if err != nil {
			px[:idx].Close(ctx)
			return err
		}
	}
	return nil
}

func (px providers) Close(ctx context.Context) {
	for _, p := range px {
		defer p.Close(ctx)
	}
}

func (px providers) StartTrace(ctx context.Context, name string, headers map[string][]string) context.Context {
	for _, p := range px {
		ctx = p.StartTrace(ctx, name, headers)
	}
	return ctx
}

func (px providers) EndTrace(ctx context.Context) {
	for _, p := range px {
		p.EndTrace(ctx)
	}
}

func (px providers) AddTraceInfo(ctx context.Context, info logf.Marshaler) {
	for _, p := range px {
		p.AddTraceInfo(ctx, info)
	}
}

func (px providers) StartSpan(ctx context.Context, name string, spanType SpanType, args logf.Marshaler) context.Context {
	for _, p := range px {
		ctx = p.StartSpan(ctx, name, spanType, args)
	}
	return ctx
}

func (px providers) EndSpan(ctx context.Context, spanType SpanType) {
	for idx := range px {
		defer px[len(px)-idx-1].EndSpan(ctx, spanType)
	}
}

func (px providers) AddSpanInfo(ctx context.Context, spanType SpanType, info logf.Marshaler) {
	for _, p := range px {
		p.AddSpanInfo(ctx, spanType, info)
	}
}

func (px providers) SetSpanStatus(ctx context.Context, spanType SpanType, err error) {
	for _, p := range px {
		p.SetSpanStatus(ctx, spanType, err)
	}
}

func (px providers) CurrentInfo(ctx context.Context, info *Info) {
	for _, p := range px {
		p.CurrentInfo(ctx, info)
	}
}

func (px providers) CurrentHeaders(ctx context.Context, headers map[string][]string) {
	for _, p := range px {
		p.CurrentHeaders(ctx, headers)
	}
}

func (px providers) SetPresendHook(hook func(map[string]interface{})) {
	type debug interface {
		SetPresendHook(hook func(map[string]interface{}))
	}
	for _, p := range px {
		if d, ok := p.(debug); ok {
			d.SetPresendHook(hook)
		}
	}
}

func (px providers) SetCurrentSampleRate(ctx context.Context, rate uint) context.Context {
	type debug interface {
		SetCurrentSampleRate(ctx context.Context, rate uint) context.Context
	}
	for _, p := range px {
		if d, ok := p.(debug); ok {
			ctx = d.SetCurrentSampleRate(ctx, rate)
		}
	}
	return ctx
}

func (px providers) ForceTrace(ctx context.Context) context.Context {
	type debug interface {
		ForceTrace(ctx context.Context) context.Context
	}
	for _, p := range px {
		if d, ok := p.(debug); ok {
			ctx = d.ForceTrace(ctx)
		}
	}
	return ctx
}
