package trace

import (
	"context"
	"net/http"

	"github.com/getoutreach/gobox/pkg/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type tracer interface {
	registerSpanProcessor(s sdktrace.SpanProcessor)
	// Deprecated: Use initTracer() instead.
	startTracing(serviceName string) error

	initTracer(ctx context.Context, serviceName string) error

	// Deprecated: Use closeTracer() instead.
	endTracing()

	closeTracer(ctx context.Context)

	// Deprecated: Use startSpan() instead.
	startTrace(ctx context.Context, name string) context.Context

	id(ctx context.Context) string

	startSpan(ctx context.Context, name string, opts ...SpanStartOption) context.Context

	// Deprecated: Use startSpan() instead.
	startSpanAsync(ctx context.Context, name string) context.Context

	end(ctx context.Context)

	addInfo(ctx context.Context, args ...log.Marshaler)

	spanID(ctx context.Context) string

	// Deprecated: Will be removed with full migration to OpenTelemetry.
	// OpenTelemetry automatically handle adding parentID to traces
	parentID(ctx context.Context) string

	newTransport(http.RoundTripper) http.RoundTripper

	newHandler(handler http.Handler, operation string) http.Handler

	isForce() bool

	setForce(force bool)

	toHeaders(ctx context.Context) map[string][]string

	contextFromHeaders(ctx context.Context, hdrs map[string][]string) context.Context

	// fromHeaders is similar to contextFromHeaders + it starts a new span
	fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context

	// isSampled is used to determine if the given trace ID should be sampled based on the
	// current sample rate
	isSampled(traceID string) bool
}
