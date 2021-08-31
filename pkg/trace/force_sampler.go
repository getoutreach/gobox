package trace

import (
	"context"

	"github.com/honeycombio/beeline-go/sample"
	"github.com/honeycombio/beeline-go/trace"
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
	if t := trace.GetTraceFromContext(ctx); t != nil {
		t.AddField(fieldForceTrace, "true")
	}
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
