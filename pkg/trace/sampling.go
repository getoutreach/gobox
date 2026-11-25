// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements the default trace sampler: OpenTelemetry probability
// sampling for new traces, parent-based sampling with sample rate propagation,
// and the X-Force-Trace override.

package trace

import (
	"context"
	"crypto/sha1" //nolint:gosec // Why: non-cryptographic hash kept for Honeycomb beeline compatibility
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/getoutreach/gobox/pkg/events"
	"github.com/getoutreach/gobox/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/x"
	"go.opentelemetry.io/otel/trace"
)

//nolint:gochecknoglobals // Why: read-only attribute key
var sampleRateAttribute = attribute.Key("SampleRate")

const (
	fieldForceTrace = "force_trace"
)

type forceTraceContextKeyT int

const (
	forceTraceContextKey forceTraceContextKeyT = iota
)

// forceTracing turn on forceTracing starting with the next span.
func forceTracing(ctx context.Context) context.Context {
	return context.WithValue(ctx, forceTraceContextKey, true)
}

// isTracingForced returns true if the force-tracing context flag is set.
func isTracingForced(ctx context.Context) bool {
	val, ok := ctx.Value(forceTraceContextKey).(bool)
	return ok && val
}

// sampleRate converts a sample percentage into the number of requests seen per
// request sampled.  Percentages above 50 truncate to 1, which keeps every trace.
func sampleRate(percent float64) uint {
	return uint(100 / percent)
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
// sample rate when making decisions about new local traces, and as our last
// resort guess as to the upstream sample rate when we continue from a remote
// parent that did not tell us its rate.
//
// The provided sample rate is the number of requests seen per request sampled.
func defaultSampler(sampleRate uint) sdktrace.Sampler {
	parentSampler := sdktrace.ParentBased(
		// New, non-remote trace: use the OpenTelemetry threshold-based
		// probability sampler, which records its decision as `ot=th:...` in
		// the tracestate.
		NewProbabilitySampler(sampleRate),

		// Read the upstream sample rate from the tracestate: the
		// OpenTelemetry `th` threshold first, then our legacy `sample_rate`
		// key.  If neither is present, assume the calling service used the
		// same sample rate we're configured with.
		sdktrace.WithRemoteParentSampled(NewSampleRateTaggingSampler(sampleRate)),

		// Materialize the propagated sample rate, probably originating
		// from one of the samplers above, as a tag in our outgoing span.
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
//
//nolint:gocritic // Why: required by otel
func (s *forceTraceHeaderSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	ctx := p.ParentContext
	psc := trace.SpanContextFromContext(p.ParentContext)

	if isTracingForced(ctx) {
		// A forced trace is kept with probability 1, which OpenTelemetry
		// encodes as `th:0`.  This replaces any threshold inherited from
		// the parent, so downstream services derive a sample rate of 1.
		ts := setTraceStateThreshold(psc.TraceState(), "0")
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: SetTraceStateSampleRate(ts, 1),
			Attributes: []attribute.KeyValue{
				sampleRateAttribute.Int(1),
			},
		}
	}

	return s.delegate.ShouldSample(p)
}

// NewProbabilitySampler returns a sampler for new local traces that uses the
// OpenTelemetry threshold-based probability sampling algorithm [1].
//
// The sample rate is the number of requests seen per request sampled.  A
// sample rate of 0 never samples.
//
// Sampled decisions carry the OpenTelemetry `ot=th:...` threshold in the
// tracestate, plus our legacy `sample_rate` tracestate key and `SampleRate`
// span attribute, both derived from that threshold.  The legacy key lets
// services that don't understand `th` yet keep reading the sample rate.
//
// This sampler doesn't look at the parent's sampling decision.  Use it as the
// root sampler of a ParentBased sampler.
//
// [1] https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/
func NewProbabilitySampler(sampleRate uint) sdktrace.Sampler {
	probability := 0.0
	if sampleRate > 0 {
		probability = 1 / float64(sampleRate)
	}
	return &probabilitySampler{
		delegate:   x.ProbabilitySampler(probability),
		sampleRate: sampleRate,
	}
}

var _ sdktrace.Sampler = (*probabilitySampler)(nil)

type probabilitySampler struct {
	delegate   sdktrace.Sampler
	sampleRate uint
}

// Description returns a description of this `probabilitySampler`.
func (s *probabilitySampler) Description() string {
	return fmt.Sprintf("probabilitySampler{1/%d}", s.sampleRate)
}

// ShouldSample delegates the decision to the OpenTelemetry probability sampler
// and, when sampled, adds the legacy sample rate to the tracestate and span.
//
//nolint:gocritic // Why: required by otel
func (s *probabilitySampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	res := s.delegate.ShouldSample(p)
	if res.Decision != sdktrace.RecordAndSample {
		return res
	}

	// Derive the rate from the threshold actually written, so this service and
	// every downstream reader of `th` agree on it.  The configured rate is a
	// fallback that should never be needed.
	rate, ok := sampleRateFromThreshold(res.Tracestate)
	if !ok {
		rate = s.sampleRate
	}

	res.Tracestate = SetTraceStateSampleRate(res.Tracestate, rate)
	res.Attributes = append(res.Attributes,
		//nolint:gosec // Why: sample rate is far below MaxInt
		sampleRateAttribute.Int(int(rate)),
	)
	return res
}

// NewHoneycombDeterministicSampler returns a sampler that mimics the
// deterministic sampler behavior from Honeycomb's beeline libraries.
//
// A sample rate of 0 never samples.
//
// Deprecated: the default sampler now uses NewProbabilitySampler, which
// implements the OpenTelemetry standard.  This sampler doesn't write the
// OpenTelemetry `th` threshold, so downstream services fall back to the legacy
// `sample_rate` tracestate key.
func NewHoneycombDeterministicSampler(sampleRate uint) sdktrace.Sampler {
	var upperBound uint32
	if sampleRate > 0 {
		//nolint:gosec // Why: matches beeline, which truncates the rate to 32 bits
		upperBound = math.MaxUint32 / uint32(sampleRate)
	}
	return &honeycombDeterministicSampler{
		sampleRate: sampleRate,
		upperBound: upperBound,
	}
}

var _ sdktrace.Sampler = (*honeycombDeterministicSampler)(nil)

type honeycombDeterministicSampler struct {
	sampleRate uint
	upperBound uint32
}

// sample reproduces beeline-go's DeterministicSampler.Sample: the first four
// bytes of the SHA-1 hash of the determinant, read as a big-endian uint32,
// must not exceed MaxUint32 / sampleRate.
func (s *honeycombDeterministicSampler) sample(determinant string) bool {
	switch s.sampleRate {
	case 0:
		return false
	case 1:
		return true
	}
	sum := sha1.Sum([]byte(determinant)) //nolint:gosec // Why: compatibility hash, not security
	return binary.BigEndian.Uint32(sum[:4]) <= s.upperBound
}

// Description returns a description of this `honeycombDeterministicSampler`.
func (s *honeycombDeterministicSampler) Description() string {
	return fmt.Sprintf("honeycombDeterministicSampler{1/%d}", s.sampleRate)
}

// ShouldSample returns a "record and sample" decision with an appropriate
// sample_rate entry in its tracestate if the traceID is sampled at the current
//
// sample rate, and a "drop" decision otherwise.
//
//nolint:gocritic // Why: required by otel
func (s *honeycombDeterministicSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)

	traceID := p.TraceID.String()
	if s.sample(traceID) {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: SetTraceStateSampleRate(psc.TraceState(), s.sampleRate),
			Attributes: []attribute.KeyValue{
				//nolint: gosec // Why: sample rate is guaranteed not to be < 0
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
// decisions.  It simply inherits the decision from its parent.  We use it not
// for decision-making, but to ensure all sampled spans include an appropriate
// "SampleRate" tag, which is used for sample rate adjustment.
//
// The sample rate comes from the parent's tracestate, in order of preference:
//
//  1. The OpenTelemetry `ot=th:...` threshold [1].  When this is present, the
//     legacy `sample_rate` key is rewritten to match, so downstream services
//     that only understand `sample_rate` see the same rate.
//  2. Our legacy `sample_rate` key.
//  3. The provided default sample rate, as a guess.
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
//
//nolint:gocritic // Why: required by otel
func (s *sampleRateTaggingSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	psc := trace.SpanContextFromContext(p.ParentContext)
	ts := psc.TraceState()

	if psc.IsSampled() {
		if sampleRate, ok := sampleRateFromThreshold(ts); ok {
			if legacy, ok := GetTraceStateSampleRate(ts); !ok || legacy != sampleRate {
				ts = SetTraceStateSampleRate(ts, sampleRate)
			}
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: ts,
				Attributes: []attribute.KeyValue{
					//nolint:gosec // Why: sample rate is far below MaxInt
					sampleRateAttribute.Int(int(sampleRate)),
				},
			}
		}

		sampleRate, ok := GetTraceStateSampleRate(ts)
		if ok {
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: ts,
				Attributes: []attribute.KeyValue{
					//nolint: gosec // Why: sample rate is guaranteed not to be < 0
					sampleRateAttribute.Int(int(sampleRate)),
				},
			}
		}

		// Sampled but no "sample_rate" in the tracestate.
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: ts,
			Attributes: []attribute.KeyValue{
				//nolint: gosec // Why: sample rate is guaranteed not to be < 0
				sampleRateAttribute.Int(int(s.defaultSampleRate)),
			},
		}
	}

	// Not sampled.
	return sdktrace.SamplingResult{
		Decision:   sdktrace.Drop,
		Tracestate: ts,
	}
}

// sampleRateKey is the key we use to store the sample rate in the tracestate.
//
// This value goes into the tracecontext header [1] that gets sent to downstream
// services, allowing them to populate their sample rate tags without guessing.
//
// This is our legacy, non-standard key.  The OpenTelemetry standard is the `th`
// sub-key of the `ot` tracestate entry [2].  We keep writing both until every
// service that reads `sample_rate` also understands `th`.
//
// [1] https://www.w3.org/TR/trace-context/
// [2] https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/
const sampleRateKey = "sample_rate"

// OpenTelemetry tracestate probability sampling encoding.  See
// https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/
const (
	// otTraceStateKey is the vendor key of the OpenTelemetry tracestate entry.
	otTraceStateKey = "ot"

	// otThresholdSubKey is the sub-key holding the rejection threshold.
	otThresholdSubKey = "th"

	// otThresholdDigits is the maximum number of hex digits in a threshold,
	// which covers 56 bits.
	otThresholdDigits = 14

	// otMaxAdjustedCount is 2^56, the size of the randomness space.
	otMaxAdjustedCount = uint64(1) << 56
)

// sampleRateFromThreshold returns the sample rate (requests seen per request
// sampled) encoded by the OpenTelemetry `th` threshold in the tracestate.
//
// The second return value is false if there is no valid threshold.
func sampleRateFromThreshold(ts trace.TraceState) (uint, bool) {
	th, ok := otSubKey(ts.Get(otTraceStateKey), otThresholdSubKey)
	if !ok || th == "" || len(th) > otThresholdDigits {
		return 0, false
	}

	// Thresholds are written with trailing zeros removed.  Pad them back out
	// to the full 56 bits before parsing.
	threshold, err := strconv.ParseUint(th+strings.Repeat("0", otThresholdDigits-len(th)), 16, 64)
	if err != nil || threshold >= otMaxAdjustedCount {
		return 0, false
	}

	// The sampling probability is (2^56 - threshold) / 2^56, so the sample
	// rate is its inverse.
	rate := math.Round(float64(otMaxAdjustedCount) / float64(otMaxAdjustedCount-threshold))
	return uint(rate), true
}

// setTraceStateThreshold sets the `th` sub-key of the OpenTelemetry tracestate
// entry, preserving its other sub-keys (such as the `rv` randomness).  The
// threshold is placed first, as the OpenTelemetry SDK does.
func setTraceStateThreshold(ts trace.TraceState, th string) trace.TraceState {
	subKeys := []string{otThresholdSubKey + ":" + th}
	for kv := range strings.SplitSeq(ts.Get(otTraceStateKey), ";") {
		if kv == "" || strings.HasPrefix(kv, otThresholdSubKey+":") {
			continue
		}
		subKeys = append(subKeys, kv)
	}

	updated, err := ts.Insert(otTraceStateKey, strings.Join(subKeys, ";"))
	if err != nil {
		// The existing `ot` entry was malformed; leave the tracestate as is.
		log.Warn(context.Background(), "failed to insert sampling threshold into tracestate", events.NewErrorInfo(err))
		return ts
	}
	return updated
}

// otSubKey returns the value of a sub-key in an OpenTelemetry tracestate entry
// value, which has the form `key1:value1;key2:value2`.
func otSubKey(ot, key string) (string, bool) {
	for kv := range strings.SplitSeq(ot, ";") {
		if value, found := strings.CutPrefix(kv, key+":"); found {
			return value, true
		}
	}
	return "", false
}

// SetTraceStateSampleRate sets the sample rate in the tracestate.
func SetTraceStateSampleRate(ts trace.TraceState, sampleRate uint) trace.TraceState {
	updated, err := ts.Insert(sampleRateKey, strconv.FormatUint(uint64(sampleRate), 10))
	if err != nil {
		// This really shouldn't happen.
		log.Warn(context.Background(), "failed to insert sample rate into tracestate", events.NewErrorInfo(err))
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
