// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides capabilities for forcing sampling on traces, including propagation of forced traces

package trace

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"github.com/honeycombio/beeline-go/sample"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var sampleRateAttribute = attribute.Key("SampleRate")

const (
	fieldForceTrace = "force_trace"
)

type forceTraceContextKeyT int

const (
	forceTraceContextKey forceTraceContextKeyT = iota
)

// forceTracing turn on forceTracing starting with the next span
func forceTracing(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, forceTraceContextKey, true)
	return ctx
}

// isTracingForced returns true if the force-tracing context flag is set.
func isTracingForced(ctx context.Context) bool {
	val, ok := ctx.Value(forceTraceContextKey).(bool)
	return ok && val
}

// defaultSampler is a sampler that tries to provide reasonable
// backwards-compatible behaviors.  It's a sensible default.
//
// This sampler is not just a standard OpenTelemetry sampler.  It includes
// custom behaviors to support the `X-Force-Trace` header, and to propagate the
// sample rate as a tag.  Also, unlike its predecessor, it respects
// OpenTelemetry parent-based sampling.
//
// This sampler accepts one parameter, a sample rate.  This is used both as the
// sample rate when making decisions about new local traces, and as our best
// guess as to the upstream sample rate when we continue from a remote parent.
//
// The provided sample rate is the number of requests seen per requset sampled.
func defaultSampler(sampleRate uint) sdktrace.Sampler {
	parentSampler := sdktrace.ParentBased(
		// new, non-remote trace: use HC deterministic sampler.
		NewHoneycombDeterministicSampler(sampleRate),

		// Assume our parent uses the same sample rate that we do.  It's
		// an odd assumption, but it's the better options aren't fully
		// standardized yet.
		sdktrace.WithRemoteParentSampled(NewSampleRateTaggingSampler(sampleRate)),

		// This hack takes our propagated "sample_rate", probably
		// originating from one of the samplers above, and materializes
		// it as a tag in our outgoing span.
		sdktrace.WithLocalParentSampled(NewSampleRateTaggingSampler(sampleRate)),

		// We leave the non-sampled cases to the default drop behaviors.
	)

	// Wrap it all with support for `X-Force-Trace` headers.
	return NewForceTraceHeaderSampler(parentSampler)
}

var _ sdktrace.Sampler = (*forceTraceHeaderSampler)(nil)

// NewForceTraceHeaderSampler wraps an existing sampler with support for the
// X-Force-Trace header, with the help of the `isTracingForced` context flag.
func NewForceTraceHeaderSampler(delegate sdktrace.Sampler) sdktrace.Sampler {
	return &forceTraceHeaderSampler{
		delegate: delegate,
	}
}

// forceTraceHeaderSampler allows force sample rate to 100% when trace context
// contains field force_trace.  It's used to support the `X-Force-Trace` header,
// and in theory some other ad-hoc trace possibilities.
type forceTraceHeaderSampler struct {
	delegate sdktrace.Sampler
}

// Description returns a description of this `forceTraceHeaderSampler`.
func (s *forceTraceHeaderSampler) Description() string {
	return fmt.Sprintf("forceTraceHeaderSampler{%s}", s.delegate.Description())
}

// ShouldSample returns true if the context-based isTracingForced flag is enabled,
// and delegates to its provided sampler otherwise.
func (s *forceTraceHeaderSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	ctx := p.ParentContext
	psc := trace.SpanContextFromContext(p.ParentContext)

	if isTracingForced(ctx) {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: SetTraceStateSampleRate(psc.TraceState(), 1),
			Attributes: []attribute.KeyValue{
				sampleRateAttribute.Int(1),
			},
		}
	}

	return s.delegate.ShouldSample(p)
}

// NewHoneycombDeterministicSampler returns a sampler that mimics the
// deterministic sampler behavior from Honeycomb's beeline libraries.  This was
// formerly our primary sampler, but now we use it only for new local traces.
//
// Our exact choice of sampling algorithm shouldn't matter in a world where
// every service respects parent-based sampling.  However, we have a lot of
// old services that still have this HC deterministic sampler hard-coded, and
// so using this as our sampler reduces the odds of broken traces.
//
// In the longer term, when every service supports parent-based sampling, we can
// stop caring which sampler we use for this case.
func NewHoneycombDeterministicSampler(sampleRate uint) sdktrace.Sampler {
	sampler, err := sample.NewDeterministicSampler(sampleRate)
	if err != nil {
		panic(err)
	}
	return &honeycombDeterministicSampler{
		sampler:    sampler,
		sampleRate: sampleRate,
	}
}

var _ sdktrace.Sampler = (*honeycombDeterministicSampler)(nil)

type honeycombDeterministicSampler struct {
	sampler    *sample.DeterministicSampler
	sampleRate uint
}

// Description returns a description of this `honeycombDeterministicSampler`.
func (s *honeycombDeterministicSampler) Description() string {
	return fmt.Sprintf("honeycombDeterministicSampler{1/%d}", s.sampleRate)
}

// ShouldSample returns a "record and sample" decision with an appropriate
// sample_rate entry in its tracestate if the traceID is sampled at the current
// sample rate, and a "drop" decision otherwise.
func (s *honeycombDeterministicSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)

	traceID := p.TraceID.String()
	if s.sampler.Sample(traceID) {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: SetTraceStateSampleRate(psc.TraceState(), s.sampleRate),
			Attributes: []attribute.KeyValue{
				sampleRateAttribute.Int(int(s.sampleRate)),
			},
		}
	}

	return sdktrace.SamplingResult{
		Decision:   sdktrace.Drop,
		Tracestate: psc.TraceState(),
	}
}

var _ sdktrace.Sampler = (*sampleRateTaggingSampler)(nil)

type sampleRateTaggingSampler struct {
	defaultSampleRate uint
}

// NewSampleRateTaggingSampler is a sampler that doesn't actually make sampling
// decisions.  It simply inherits the decision from its parent.  We used it not
// for decision-making, but to ensure all sampled spans include an appropriate
// "SampleRate" tag that Honeycomb can use for sample rate adjustment.
//
// This is a hack.  We hope that one day we can move to an official
// OpenTelemetry head-based sampling mechanism that handles this more
// cleanly. [1] At the time of writing, this part of the standard is in flux and
// not all OpenTelemetry libraries support it.
//
// [1] https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/
func NewSampleRateTaggingSampler(defaultSampleRate uint) sdktrace.Sampler {
	return &sampleRateTaggingSampler{
		defaultSampleRate,
	}
}

// Description returns a description of this `sampleRateTaggingSampler`.
func (s *sampleRateTaggingSampler) Description() string {
	return "sampleRateTaggingSampler"
}

// ShouldSample returns "record and sample" if the parent span context is sampled
// and "drop" otherwise.
//
// In the case where the parent span context is sampled, we also attach a sample
// rate attribute to this span.  This hack is the main purrpose of this sampler.
func (s *sampleRateTaggingSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)
	ts := psc.TraceState()

	if psc.IsSampled() {
		sampleRate, ok := GetTraceStateSampleRate(ts)
		if ok {
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: ts,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(int(sampleRate)),
				},
			}
		} else {
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: ts,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(int(s.defaultSampleRate)),
				},
			}
		}
	} else {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.Drop,
			Tracestate: ts,
		}
	}
}

// sampleRateKey is the key we use to store the sample rate in the tracestate.
//
// This value goes into the tracecontext header [1] that gets sent to downstream
// services.  This could be used to forward sample rate information to other
// services, allowing them to populate their sample rate tags without guessing.
//
// We don't do this currently.  We can get away with guessing for now because
// our sample rates tend to line up across all services.  Hopefully we'll be
// able to move to an official standard before this becomes necessary.
//
// [1] https://www.w3.org/TR/trace-context/
const sampleRateKey = "sample_rate"

// SetTraceStateSampleRate sets the sample rate in the tracestate.
func SetTraceStateSampleRate(ts trace.TraceState, sampleRate uint) trace.TraceState {
	updated, err := ts.Insert(sampleRateKey, fmt.Sprintf("%d", sampleRate))
	if err != nil {
		// This really shouldn't happen.
		log.Warn(context.TODO(), "failed to insert sample rate into tracestate", events.NewErrorInfo(err))
		return ts
	}
	return updated
}

// GetTraceStateSampleRate fetches the sample rate from the tracestate.
//
// The second return value is an `ok` bool that is false if the fetch fails.
func GetTraceStateSampleRate(ts trace.TraceState) (uint, bool) {
	var ret uint
	stringVal := ts.Get(sampleRateKey)
	_, err := fmt.Sscanf(stringVal, "%d", &ret)
	if err != nil {
		return 0, false
	}
	return ret, true
}
