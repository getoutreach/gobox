package trace

import (
	"context"

	"github.com/honeycombio/beeline-go/sample"
	"github.com/honeycombio/beeline-go/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type samplerHook func(map[string]interface{}) (bool, int)

const (
	fieldTraceID     = "trace.trace_id"
	fieldForceTrace  = "force_trace"
	fieldSampleTrace = "sample_trace"
)

// forceSampler allows force sample rate to 100% when trace context contains field force_trace
// and sample at a rate of 1/<given rate> when context contains field sample_trace.
func forceSampler(sampleRate uint) samplerHook {
	sampler, err := sample.NewDeterministicSampler(sampleRate)
	if err != nil {
		panic(err)
	}

	return func(fields map[string]interface{}) (bool, int) {
		if _, ok := fields[fieldForceTrace]; ok {
			return true, 1
		}

		if rawRate, ok := fields[fieldSampleTrace]; ok {
			if rate, ok := rawRate.(uint); ok {
				return true, int(rate)
			}
		}

		if traceID, ok := fields[fieldTraceID].(string); ok {
			return sampler.Sample(traceID), sampler.GetSampleRate()
		}
		return false, 0
	}
}

func forceTracing(ctx context.Context) context.Context {
	defaultTracer.setForce(true)

	return ctx
}

// sampleAt sets the sample rate for a given trace explicitly.
// The rate specifies how many samples were looked at for each
// accepted sample.  That is, the actual sampling  = 1/rate.
func sampleAt(ctx context.Context, rate uint) context.Context {
	if t := trace.GetTraceFromContext(ctx); t != nil {
		t.AddField(fieldSampleTrace, rate)
	}
	return ctx
}

type otelForceSampler struct {
	sampleRate uint
}

func (s *otelForceSampler) Description() string {
	return "Samples at the specified rate or forces sampling based on the `force_trace` attribute."
}

//nolint:gocritic // Why: Required to pass SamplingParameters as a copy
func (s *otelForceSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := oteltrace.SpanContextFromContext(p.ParentContext)
	sampler, err := sample.NewDeterministicSampler(s.sampleRate)
	if err != nil {
		panic(err)
	}

	var forceTrace bool
	for _, a := range p.Attributes {
		if string(a.Key) == fieldForceTrace {
			forceTrace = a.Value.AsBool()
		}
	}

	if forceTrace {
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

func forceSample(sampleRate uint) sdktrace.Sampler {
	return &otelForceSampler{
		sampleRate: sampleRate,
	}
}
