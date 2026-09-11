// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Tests the LogTracedCalls behavior, which logs calls belonging to an exported trace

package trace

import (
	"context"
	"testing"

	"github.com/getoutreach/gobox/pkg/log"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestShouldLogTracedCalls(t *testing.T) {
	tests := []struct {
		name           string
		logTracedCalls bool
		samplePercent  float64
		want           bool
	}{
		{"off by default", false, 0.25, false},
		{"on when sampling is selective", true, 0.25, true},
		{"ignored when everything is sampled", true, 100, false},
		{"on just below full sampling", true, 99.9, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{LogTracedCalls: tt.logTracedCalls}
			config.Otel.SamplePercent = tt.samplePercent

			if got := shouldLogTracedCalls(config); got != tt.want {
				t.Errorf("shouldLogTracedCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

// sampledSpanContext returns a span context with the sampled flag set.
func sampledSpanContext() oteltrace.SpanContext {
	return oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:     oteltrace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		TraceFlags: oteltrace.FlagsSampled,
	})
}

// TestLogTracedCall checks that calls in an exported trace are promoted to info
// logging, and that explicit choices are not overridden.
func TestLogTracedCall(t *testing.T) {
	tests := []struct {
		name              string
		logTracedCalls    bool
		logCallsByDefault bool
		sampled           bool
		opts              []log.Marshaler
		want              bool
	}{
		{
			name:           "promotes a sampled call",
			logTracedCalls: true,
			sampled:        true,
			want:           true,
		},
		{
			name:           "leaves an unsampled call alone",
			logTracedCalls: true,
			sampled:        false,
			want:           false,
		},
		{
			name:           "does nothing when not configured",
			logTracedCalls: false,
			sampled:        true,
			want:           false,
		},
		{
			name:           "does not override an explicit disable",
			logTracedCalls: true,
			sampled:        true,
			opts:           []log.Marshaler{WithInfoLoggingDisabled()},
			want:           false,
		},
		{
			name:           "leaves an explicit enable enabled",
			logTracedCalls: true,
			sampled:        true,
			opts:           []log.Marshaler{WithInfoLoggingEnabled()},
			want:           true,
		},
		{
			name:              "leaves logging on when it is the default",
			logTracedCalls:    true,
			logCallsByDefault: true,
			sampled:           true,
			want:              true,
		},
		{
			name:           "promotes a call the resolver sampled out",
			logTracedCalls: true,
			sampled:        true,
			opts:           []log.Marshaler{ResolvedLogging(InfoLoggingSampledOut)},
			want:           true,
		},
		{
			name:              "promotes a sampled out call over an enabled default",
			logTracedCalls:    true,
			logCallsByDefault: true,
			sampled:           true,
			opts:              []log.Marshaler{ResolvedLogging(InfoLoggingSampledOut)},
			want:              true,
		},
		{
			name:           "leaves a sampled out call off when the trace is not exported",
			logTracedCalls: true,
			sampled:        false,
			opts:           []log.Marshaler{ResolvedLogging(InfoLoggingSampledOut)},
			want:           false,
		},
		{
			name:           "does not promote a call the resolver disabled",
			logTracedCalls: true,
			sampled:        true,
			opts:           []log.Marshaler{ResolvedLogging(InfoLoggingDisabled)},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreTracedCalls, restoreByDefault := logTracedCalls, logCallsByDefault
			logTracedCalls, logCallsByDefault = tt.logTracedCalls, tt.logCallsByDefault
			t.Cleanup(func() {
				logTracedCalls, logCallsByDefault = restoreTracedCalls, restoreByDefault
			})

			ctx := context.Background()
			if tt.sampled {
				ctx = oteltrace.ContextWithSpanContext(ctx, sampledSpanContext())
			}

			// Mirrors StartCall without needing a tracer.
			opts := append([]log.Marshaler{withDefaultInfoLogging(logCallsByDefault)}, tt.opts...)
			ctx = callTracker.StartCall(ctx, "test", opts)

			logTracedCall(ctx)

			if got := IsInfoLoggingEnabled(ctx); got != tt.want {
				t.Errorf("IsInfoLoggingEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTraceInfoExportedField checks that honeycomb.trace_exported is only added
// for exported traces.
func TestTraceInfoExportedField(t *testing.T) {
	unsampled := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: oteltrace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:  oteltrace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
	})

	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{"no span", context.Background(), false},
		{"unsampled span", oteltrace.ContextWithSpanContext(context.Background(), unsampled), false},
		{"sampled span", oteltrace.ContextWithSpanContext(context.Background(), sampledSpanContext()), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := map[string]interface{}{}
			IDs(tt.ctx).MarshalLog(func(field string, value interface{}) {
				fields[field] = value
			})

			got, ok := fields["honeycomb.trace_exported"]
			if ok != tt.want {
				t.Fatalf("honeycomb.trace_exported present = %v, want %v", ok, tt.want)
			}
			if ok && got != true {
				t.Errorf("honeycomb.trace_exported = %v, want true", got)
			}
		})
	}
}

// TestReevaluateLoggingOutranksTracedCalls checks that a resolver decision made
// during the call wins over the promotion.
func TestReevaluateLoggingOutranksTracedCalls(t *testing.T) {
	tests := []struct {
		name     string
		resolved InfoLoggingResolved
		want     bool
	}{
		{"resolver disables a promoted call", InfoLoggingDisabled, false},
		{"resolver enables it", InfoLoggingEnabled, true},
		{"sampled out stays promotable, so the promotion stands", InfoLoggingSampledOut, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := logTracedCalls
			logTracedCalls = true
			t.Cleanup(func() { logTracedCalls = restore })

			ctx := oteltrace.ContextWithSpanContext(context.Background(), sampledSpanContext())
			ctx = callTracker.StartCall(ctx, "test", []log.Marshaler{withDefaultInfoLogging(false)})

			logTracedCall(ctx)
			ReevaluateLogging(ctx, func(context.Context, string) InfoLoggingResolved {
				return tt.resolved
			})

			if got := IsInfoLoggingEnabled(ctx); got != tt.want {
				t.Errorf("IsInfoLoggingEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
