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

func forceTracing(ctx context.Context) context.Context {
	defaultTracer.setForce(true)

	return ctx
}

// forceSampler allows force sample rate to 100% when trace context contains field force_trace
// and sample at a rate of 1/<given rate> when context contains field sample_trace.
type otelForceSampler struct {
	sampler *sample.DeterministicSampler
}

func (s *otelForceSampler) Description() string {
	return "Samples at the specified rate or forces sampling based on the `force_trace` attribute."
}

//nolint:gocritic // Why: Required to pass SamplingParameters as a copy
func (s *otelForceSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)
	if defaultTracer.isForce() {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: psc.TraceState(),
		}
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

	// if not forced, use deterministic hash of the trace ID and the current rate to decide
	if s.isSampled(p.TraceID.String()) {
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

// isSampled is used to determine if the given trace ID should be sampled based on the
// current sample rate.
//
// Please keep the sampling logic deterministic because this method is used to correlate sampling between different
// services at Outreach. For example, if a producer of the trace headers generates traceID1 and a remote service
// wants to link the traceID1 to its own trace (!= traceID1), then it should only do so if traceID1 was sampled.
// Failure to do so causes honeycomb to show a trace link to a 'dead trace' - and it is not possible to filter those
// out, rendering the the cross-trace linking experience useless.
func (s *otelForceSampler) isSampled(traceID string) bool {
	return s.sampler.Sample(traceID)
}

// newSampler creates a new deterministic sampler for the given rate or panics if the rate is 0.
func newSampler(sampleRate uint) *otelForceSampler {
	sampler, err := sample.NewDeterministicSampler(sampleRate)
	if err != nil {
		panic(err)
	}

	return &otelForceSampler{
		sampler: sampler,
	}
}

var _ sdktrace.Sampler = &otelForceSampler{}
