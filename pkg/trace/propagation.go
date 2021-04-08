package trace

import (
	"context"
	"net/http"

	"github.com/getoutreach/gobox/pkg/app"
	"github.com/honeycombio/beeline-go/propagation"
	"github.com/honeycombio/beeline-go/trace"
)

// fromHeaders fetches trace info from a headers map
func (t *tracer) fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context {
	header := http.Header(hdrs)

	if t.Honeycomb.Enabled {
		// honeycomb uses X-Honeycomb-Trace header with some serialized state
		beelineHeader := header.Get(propagation.TracePropagationHTTPHeader)
		ctx2, t := trace.NewTrace(ctx, beelineHeader)
		marshalLog(t.AddField, "", app.Info())
		t.GetRootSpan().AddField("name", name)
		ctx = ctx2
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
