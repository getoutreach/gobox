// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements a RoundTripper which adds tracing

package trace

import (
	"net/http"
)

// NewTransport creates a new transport which propagates the current
// trace context.
//
// Usage:
//
//	client := &http.Client{Transport: trace.NewTransport(nil)}
//	resp, err := client.Get("/ping")
//
// For most cases, use the httpx/pkg/fetch package as it also logs the
// request, updates latency metrics and adds traces with full info
//
// Note: the request context must be derived from StartSpan.
func NewTransport(old http.RoundTripper) http.RoundTripper {
	if defaultTracer == nil {
		return old
	}

	if old == nil {
		old = http.DefaultTransport
	}

	return defaultTracer.newTransport(old)
}

// NewHandler creates a new handlers which reads propagated headers
// from the current trace context.
//
// Supported options are:
//   - WithPublicEndpointFn conditionally configure the Handler to link the span with an incoming span context
//     instead of child association. Useful for public facing endpoints that don't need to attach to externally
//     defined traces
//
// Usage:
//
//		  trace.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) *roundtripperState {
//			trace.StartSpan(r.Context(), "my endpoint")
//			defer trace.End(r.Context())
//			... do actual request handling ...
//	   }), "my endpoint")
func NewHandler(handler http.Handler, operation string, opts ...HandlerOption) http.Handler {
	if defaultTracer == nil {
		return handler
	}

	return defaultTracer.newHandler(handler, operation, opts...)
}
