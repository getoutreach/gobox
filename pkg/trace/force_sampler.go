package trace

import (
	"context"

	"github.com/honeycombio/beeline-go/sample"
	"github.com/honeycombio/beeline-go/trace"
)

type samplerHook func(map[string]interface{}) (bool, int)

const (
	fieldTraceID    = "trace.trace_id"
	fieldForceTrace = "force_trace"
)

// forceSampler allows force sample rate to 100% when trace context contains field force_trace
func forceSampler(sampleRate uint) samplerHook {
	sampler, err := sample.NewDeterministicSampler(sampleRate)
	if err != nil {
		panic(err)
	}

	return func(fields map[string]interface{}) (bool, int) {
		if _, ok := fields[fieldForceTrace]; ok {
			return true, 1
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
