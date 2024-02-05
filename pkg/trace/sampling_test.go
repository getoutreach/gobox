//go:build !or_e2e

package trace_test

import (
	"context"
	"fmt"
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
			ctx:  trace.ForceTracing(context.Background()),
			expected: sdktrace.SamplingResult{
				Decision: sdktrace.RecordAndSample,
				Attributes: []attribute.KeyValue{
					sampleRateAttribute.Int(1),
				},
				Tracestate: traceStateLiteral("sample_rate=1"),
			},
		},
		{
			name:     "force trace header not set",
			ctx:      context.Background(),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := trace.NewHoneycombDeterministicSampler(tt.sampleRate)

			traceID, err := oteltrace.TraceIDFromHex(tt.traceID)
			assert.NilError(t, err)
			spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
			assert.NilError(t, err)
			parentCtx := oteltrace.ContextWithSpanContext(
				context.Background(),
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
				Tracestate: traceStateLiteral("sample_rate=" + fmt.Sprintf("%d", tt.sampleRate)),
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
				context.Background(),
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
