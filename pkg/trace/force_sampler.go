// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides capabilities for forcing sampling on traces, including propagation of forced traces

package trace

import (
	"context"

	"github.com/honeycombio/beeline-go/sample"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	fieldForceTrace = "force_trace"
)

// forceTracing turn on forceTracing starting with the next span
func forceTracing(ctx context.Context) context.Context {
	defaultTracer.setForce(true)

	return ctx
}

// forceSampler allows force sample rate to 100% when trace context contains field force_trace
// and sample at a rate of 1/<given rate> when context contains field sample_trace.
type otelForceSampler struct {
	sampleRate uint
}

// Description provides a description for the sampler
func (s *otelForceSampler) Description() string {
	return "Samples at the specified rate or forces sampling based on the `force_trace` attribute."
}

// ShouldSample makes a determination whether the current trace should be sampled
//
//nolint:gocritic // Why: Required to pass SamplingParameters as a copy
func (s *otelForceSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)
	if defaultTracer.isForce() {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: psc.TraceState(),
		}
	}

	sampler, err := sample.NewDeterministicSampler(s.sampleRate)
	if err != nil {
		panic(err)
	}

	var forceTrace string
	for _, a := range p.Attributes {
		if string(a.Key) == fieldForceTrace {
			forceTrace = a.Value.AsString()
		}
	}

	if forceTrace != "" {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: psc.TraceState(),
		}
	}

	traceID := p.TraceID.String()
	if sampler.Sample(traceID) {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: psc.TraceState(),
		}
	}

	return sdktrace.SamplingResult{
		Decision:   sdktrace.Drop,
		Tracestate: psc.TraceState(),
	}
}

// forceSample creates a new force sampler with the provided sampleRate
func forceSample(sampleRate uint) sdktrace.Sampler {
	return &otelForceSampler{
		sampleRate: sampleRate,
	}
}
