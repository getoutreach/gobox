// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Verifies that closing an otel tracer never blocks a
// process's exit for longer than closeTracerTimeout, even when the
// configured collector is unreachable.

//go:build !or_e2e

package trace

import (
	"context"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// TestCloseTracerBoundedByTimeout confirms closeTracer returns within
// closeTracerTimeout even when the exporter's collector never answers.
// 10.255.255.1 is a private-use address (RFC 1918) chosen so the
// connection attempt goes unanswered rather than failing fast with
// connection-refused -- the failure mode a genuinely unreachable
// collector produces.
func TestCloseTracerBoundedByTimeout(t *testing.T) {
	tr, err := NewOtelTracer(t.Context(), "gobox-test", &Config{
		Otel: Otel{
			Enabled:           true,
			CollectorEndpoint: "10.255.255.1:4317",
			SamplePercent:     100,
		},
	})
	assert.NilError(t, err)

	// Record and end a span so the batch processor has something
	// queued to export -- an empty batch has nothing to flush, so
	// closeTracer would return immediately regardless of the timeout.
	spanCtx := tr.startSpan(context.Background(), "test-span")
	tr.end(spanCtx)

	start := time.Now()
	tr.closeTracer(context.Background())
	elapsed := time.Since(start)

	// Generous upper bound: closeTracerTimeout applies once for
	// ForceFlush and once for Shutdown in the worst case, plus room for
	// scheduling noise.
	assert.Assert(t, elapsed < 3*closeTracerTimeout,
		"closeTracer took %v, expected it to return within roughly %v", elapsed, 2*closeTracerTimeout)
}
