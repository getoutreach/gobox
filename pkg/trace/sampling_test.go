//go:build !or_e2e

package trace_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/getoutreach/gobox/pkg/trace"
	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"gotest.tools/v3/assert"
)

var sampleRateAttribute = attribute.Key("SampleRate")

func traceStateEqual(a, b oteltrace.TraceState) bool {
	return a.String() == b.String()
}

func keyValuesEqual(a, b attribute.KeyValue) bool {
	as := attribute.NewSet(a)
	bs := attribute.NewSet(b)
	return as.Equals(&bs)
}

func traceStateLiteral(s string) oteltrace.TraceState {
	ts, err := oteltrace.ParseTraceState(s)
	if err != nil {
		panic(err)
	}
	return ts
}

func TestForceTraceHeaderSampler(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected sdktrace.SamplingResult
	}{
		{
			name: "force trace header set",
			ctx:  trace.ForceTracing(t.Context()),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(1),
				},
				Tracestate: traceStateLiteral("sample_rate=1,ot=th:0"),
			},
		},
		{
			name:     "force trace header not set",
			ctx:      t.Context(),
			expected: sdktrace.SamplingResult{Decision: sdktrace.Drop},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := trace.NewForceTraceHeaderSampler(sdktrace.NeverSample())

			traceID, _ := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
			spanID, _ := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
			parentCtx := oteltrace.ContextWithSpanContext(
				tt.ctx,
				oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
			)
			params := sdktrace.SamplingParameters{ParentContext: parentCtx}

			actual := sampler.ShouldSample(params)
			assert.DeepEqual(t, tt.expected, actual,
				cmp.Comparer(traceStateEqual),
				cmp.Comparer(keyValuesEqual),
			)
		})
	}
}

func TestHoneycombDeterministicSampler(t *testing.T) {
	tests := []struct {
		name          string
		traceID       string
		sampleRate    uint
		expectSampled bool
	}{
		{
			name:          "Sampled at 100%",
			traceID:       "4bf92f3577b34da6a3ce929d0e0effff",
			sampleRate:    1,
			expectSampled: true,
		},
		{
			name:          "Not sampled at 50%",
			traceID:       "4bf92f3577b34da6a3ce929d0e0effff",
			sampleRate:    2,
			expectSampled: false,
		},
		{
			name:          "Sampled at 50%",
			traceID:       "4bf92f3577b34da6a3ce929d0e0efffd",
			sampleRate:    2,
			expectSampled: true,
		},
		{
			name:          "Not sampled at 1/1000000",
			traceID:       "4bf92f3577b34da6a3ce929d0e0efffd",
			sampleRate:    1000000,
			expectSampled: false,
		},
		{
			name:          "Never sampled at rate 0",
			traceID:       "4bf92f3577b34da6a3ce929d0e0effff",
			sampleRate:    0,
			expectSampled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := trace.NewHoneycombDeterministicSampler(tt.sampleRate)

			traceID, err := oteltrace.TraceIDFromHex(tt.traceID)
			assert.NilError(t, err)
			spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
			assert.NilError(t, err)
			parentCtx := oteltrace.ContextWithSpanContext(
				t.Context(),
				oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
					TraceID: traceID,
					SpanID:  spanID,
				}),
			)
			params := sdktrace.SamplingParameters{ParentContext: parentCtx, TraceID: traceID}

			actual := sampler.ShouldSample(params)

			expectSampled := sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(int(tt.sampleRate)),
				},
				Tracestate: traceStateLiteral("sample_rate=" + strconv.FormatUint(uint64(tt.sampleRate), 10)),
			}
			expectNotSampled := sdktrace.SamplingResult{Decision: sdktrace.Drop}

			if tt.expectSampled {
				assert.DeepEqual(t, expectSampled, actual,
					cmp.Comparer(traceStateEqual),
					cmp.Comparer(keyValuesEqual),
				)
			} else {
				assert.DeepEqual(t, expectNotSampled, actual,
					cmp.Comparer(traceStateEqual),
					cmp.Comparer(keyValuesEqual),
				)
			}
		})
	}
}

func TestSampleRateTaggingSampler(t *testing.T) {
	tests := []struct {
		name          string
		parentSampled bool
		tracestate    oteltrace.TraceState
		expected      sdktrace.SamplingResult
	}{
		{
			name:          "parent sampled and has trace state",
			parentSampled: true,
			tracestate:    traceStateLiteral("sample_rate=123"),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(123),
				},
				Tracestate: traceStateLiteral("sample_rate=123"),
			},
		},
		{
			name:          "parent sampled with threshold only",
			parentSampled: true,
			tracestate:    traceStateLiteral("ot=th:c"),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(4),
				},
				Tracestate: traceStateLiteral("sample_rate=4,ot=th:c"),
			},
		},
		{
			name:          "threshold wins over a disagreeing sample_rate",
			parentSampled: true,
			tracestate:    traceStateLiteral("sample_rate=123,ot=th:c"),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(4),
				},
				Tracestate: traceStateLiteral("sample_rate=4,ot=th:c"),
			},
		},
		{
			name:          "matching sample_rate and threshold are left alone",
			parentSampled: true,
			tracestate:    traceStateLiteral("foo=bar,sample_rate=4,ot=th:c"),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(4),
				},
				Tracestate: traceStateLiteral("foo=bar,sample_rate=4,ot=th:c"),
			},
		},
		{
			name:          "invalid threshold falls back to sample_rate",
			parentSampled: true,
			tracestate:    traceStateLiteral("sample_rate=123,ot=th:zz"),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(123),
				},
				Tracestate: traceStateLiteral("sample_rate=123,ot=th:zz"),
			},
		},
		{
			name:          "parent sampled and has no trace state",
			parentSampled: true,
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(100),
				},
				Tracestate: traceStateLiteral(""),
			},
		},
		{
			name:          "parent not sampled",
			parentSampled: false,
			expected: sdktrace.SamplingResult{
				Decision:   sdktrace.Drop,
				Tracestate: traceStateLiteral(""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := trace.NewSampleRateTaggingSampler(100)

			traceID, _ := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
			spanID, _ := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
			var flags oteltrace.TraceFlags
			parentCtx := oteltrace.ContextWithSpanContext(
				t.Context(),
				oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: flags.WithSampled(tt.parentSampled),
					TraceState: tt.tracestate,
				}),
			)
			params := sdktrace.SamplingParameters{ParentContext: parentCtx}

			actual := sampler.ShouldSample(params)
			assert.DeepEqual(t, tt.expected, actual,
				cmp.Comparer(traceStateEqual),
				cmp.Comparer(keyValuesEqual),
			)
		})
	}
}

func TestSetTraceStateSampleRate(t *testing.T) {
	ts := traceStateLiteral("foo=bar")
	_, ok := trace.GetTraceStateSampleRate(ts)
	assert.Equal(t, ok, false)

	ts2 := trace.SetTraceStateSampleRate(ts, 123)
	sr, ok := trace.GetTraceStateSampleRate(ts2)
	assert.Equal(t, ok, true)
	assert.Equal(t, sr, uint(123))

	ts3 := trace.SetTraceStateSampleRate(ts, 1)
	sr, ok = trace.GetTraceStateSampleRate(ts3)
	assert.Equal(t, ok, true)
	assert.Equal(t, sr, uint(1))
}

func TestForceTraceHeaderSamplerReplacesInheritedThreshold(t *testing.T) {
	sampler := trace.NewForceTraceHeaderSampler(sdktrace.NeverSample())

	traceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	assert.NilError(t, err)
	spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	assert.NilError(t, err)
	parentCtx := oteltrace.ContextWithSpanContext(
		trace.ForceTracing(t.Context()),
		oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceState: traceStateLiteral("sample_rate=100,ot=th:fd70a;rv:abcdef01234567"),
		}),
	)

	actual := sampler.ShouldSample(sdktrace.SamplingParameters{ParentContext: parentCtx})
	expected := sdktrace.SamplingResult{
		Decision: sdktrace.RecordAndSample,
		Attributes: []attribute.KeyValue{
			sampleRateAttribute.Int(1),
		},
		// The threshold becomes 0 (probability 1) and the randomness is kept.
		Tracestate: traceStateLiteral("sample_rate=1,ot=th:0;rv:abcdef01234567"),
	}
	assert.DeepEqual(t, expected, actual,
		cmp.Comparer(traceStateEqual),
		cmp.Comparer(keyValuesEqual),
	)
}

func TestProbabilitySampler(t *testing.T) {
	// The OpenTelemetry algorithm takes its randomness from the last 56 bits of
	// the trace ID, so these two IDs sit at the top and bottom of that range.
	const (
		alwaysKeptTraceID    = "4bf92f3577b34da6a3ffffffffffffff"
		alwaysDroppedTraceID = "4bf92f3577b34da6a300000000000000"
	)

	tests := []struct {
		name       string
		traceID    string
		sampleRate uint
		// expectedTh is empty when the trace should be dropped.
		expectedTh string
	}{
		{name: "rate 1 keeps everything", traceID: alwaysDroppedTraceID, sampleRate: 1, expectedTh: "0"},
		{name: "rate 2 kept", traceID: alwaysKeptTraceID, sampleRate: 2, expectedTh: "8"},
		{name: "rate 4 kept", traceID: alwaysKeptTraceID, sampleRate: 4, expectedTh: "c"},
		{name: "rate 4 dropped", traceID: alwaysDroppedTraceID, sampleRate: 4},
		{name: "rate 100 kept", traceID: alwaysKeptTraceID, sampleRate: 100, expectedTh: "fd70a"},
		{name: "rate 400 kept", traceID: alwaysKeptTraceID, sampleRate: 400, expectedTh: "ff5c29"},
		{name: "rate 0 never samples", traceID: alwaysKeptTraceID, sampleRate: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := trace.NewProbabilitySampler(tt.sampleRate)

			traceID, err := oteltrace.TraceIDFromHex(tt.traceID)
			assert.NilError(t, err)
			params := sdktrace.SamplingParameters{ParentContext: t.Context(), TraceID: traceID}

			actual := sampler.ShouldSample(params)

			if tt.expectedTh == "" {
				assert.Equal(t, actual.Decision, sdktrace.Drop)
				assert.Equal(t, len(actual.Attributes), 0)
				return
			}

			rate := strconv.FormatUint(uint64(tt.sampleRate), 10)
			expected := sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(int(tt.sampleRate)),
				},
				Tracestate: traceStateLiteral("sample_rate=" + rate + ",ot=th:" + tt.expectedTh),
			}
			assert.DeepEqual(t, expected, actual,
				cmp.Comparer(traceStateEqual),
				cmp.Comparer(keyValuesEqual),
			)

			// A downstream service reading only the threshold must arrive at
			// the same sample rate.
			child := trace.NewSampleRateTaggingSampler(999)
			spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
			assert.NilError(t, err)
			var flags oteltrace.TraceFlags
			childCtx := oteltrace.ContextWithRemoteSpanContext(
				t.Context(),
				oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: flags.WithSampled(true),
					TraceState: traceStateLiteral("ot=th:" + tt.expectedTh),
				}),
			)
			childResult := child.ShouldSample(sdktrace.SamplingParameters{ParentContext: childCtx})
			assert.DeepEqual(t, childResult.Attributes, expected.Attributes, cmp.Comparer(keyValuesEqual))
		})
	}
}

func TestDefaultSamplerRespectsParentDecision(t *testing.T) {
	// The root sampler at rate 1000000 would almost always drop this trace,
	// but a sampled remote parent must still be followed.
	sampler := trace.NewForceTraceHeaderSampler(sdktrace.ParentBased(
		trace.NewProbabilitySampler(1000000),
		sdktrace.WithRemoteParentSampled(trace.NewSampleRateTaggingSampler(1000000)),
	))

	traceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a300000000000000")
	assert.NilError(t, err)
	spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	assert.NilError(t, err)
	var flags oteltrace.TraceFlags
	parentCtx := oteltrace.ContextWithRemoteSpanContext(
		t.Context(),
		oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: flags.WithSampled(true),
			TraceState: traceStateLiteral("ot=th:fd70a"),
		}),
	)

	actual := sampler.ShouldSample(sdktrace.SamplingParameters{ParentContext: parentCtx, TraceID: traceID})
	assert.Equal(t, actual.Decision, sdktrace.RecordAndSample)
	assert.DeepEqual(t, actual.Attributes, []attribute.KeyValue{sampleRateAttribute.Int(100)}, cmp.Comparer(keyValuesEqual))
}
