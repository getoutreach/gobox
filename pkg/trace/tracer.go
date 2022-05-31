package trace

import (
	"context"
	"net/http"

	"github.com/getoutreach/gobox/pkg/log"
)

type tracer interface {
	// Deprecated: Use initTracer() instead.
	startTracing(serviceName string) error

	initTracer(ctx context.Context, serviceName string) error

	// Deprecated: Use closeTracer() instead.
	endTracing()

	closeTracer(ctx context.Context)

	startTrace(ctx context.Context, name string) context.Context

	id(ctx context.Context) string

	startSpan(ctx context.Context, name string) context.Context

	startSpanAsync(ctx context.Context, name string) context.Context

	end(ctx context.Context)

	addInfo(ctx context.Context, args ...log.Marshaler)

	spanID(ctx context.Context) string

	// Deprecated: Will be removed with full migration to OpenTelemetry.
	// OpenTelemetry automatically handle adding parentID to traces
	parentID(ctx context.Context) string

	newTransport(http.RoundTripper) http.RoundTripper

	// Deprecated: will be removed in favor of automatic instrumentation
	fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context

	// Deprecated: will be removed in favor of automatic instrumentation
	toHeaders(ctx context.Context) map[string][]string
}
