package trace

import (
	"net/http"
)

// NewTransport creates a new transport which propagates the current
// trace context.
//
// Usage:
//
//    client := &http.Client{Transport: trace.NewTransport(nil)}
//    resp, err := client.Get("/ping")
//
//
// For most cases, use the httpx/pkg/fetch package as it also logs the
// request, updates latency metrics and adds traces with full info
//
// Note: the request context must be derived from StartSpan/StartTrace etc.
func NewTransport(old http.RoundTripper) http.RoundTripper {
	if old == nil {
		old = http.DefaultTransport
	}
	return &roundtripper{old}
}

type roundtripper struct {
	old http.RoundTripper
}

func (rt roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	for k, v := range ToHeaders(r.Context()) {
		r.Header[k] = v
	}
	return rt.old.RoundTrip(r)
}
