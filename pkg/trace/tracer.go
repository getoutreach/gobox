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

	// Deprecated: Will be removed with full migration to OpenTelemetry
	// Gets the parent id. The second return value represents if a parentID should be present.
	// Not all protocol support a parentID
	parentID(ctx context.Context) string

	newTransport(http.RoundTripper) http.RoundTripper

	// Deprecated: Will be removed with full migration to OpenTelemetry
	fromHeaders(ctx context.Context, hdrs map[string][]string, name string) context.Context

	// Deprecated: Will be removed with full migration to OpenTelemetry
	toHeaders(ctx context.Context) map[string][]string
}
