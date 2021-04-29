package trace

import (
	"context"
	"net/http"

	"github.com/honeycombio/beeline-go/propagation"
	"github.com/honeycombio/beeline-go/trace"
)

const (
	// Header that enforces the tracing for particular request
	HeaderForceTracing = "X-Force-Trace"
)

// fromHeaders fetches trace info from a headers map
func (t *tracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	header := http.Header(hdrs)

	if t.Honeycomb.Enabled {
		//nolint:errcheck // Why: we don't report errors in our current interface
		prop, _ := propagation.UnmarshalHoneycombTraceContext(header.Get(propagation.TracePropagationHTTPHeader))
		ctx = t.startHoneycombTrace(ctx, name, prop)
	}

	if _, exists := header[HeaderForceTracing]; exists {
		ctx = ForceTracing(ctx)
	}
	return ctx
}

// toHeaders writes the current trace context into a headers map
func (t *tracer) toHeaders(ctx context.Context) map[string][]string {
	result := http.Header{}

	if t.Honeycomb.Enabled {
		if span := trace.GetSpanFromContext(ctx); span != nil {
			result.Set(propagation.TracePropagationHTTPHeader, span.SerializeHeaders())
		}
	}

	return result
}
