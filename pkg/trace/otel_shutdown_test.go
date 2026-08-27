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

// TestCloseTracerBoundedByTimeout confirms closeTracer returns well
// within closeTracerTimeout's grace period even when the exporter can
// never reach its collector -- a CLI process exiting should never
// block a user's terminal for the multi-second delays a hung network
// call can otherwise cause. 10.255.255.1 is a non-routable address
// (RFC 5737-adjacent private range unassigned on this host), chosen
// so the connection attempt goes unanswered rather than failing fast
// with connection-refused, the same way a real unreachable collector
// would behave.
func TestCloseTracerBoundedByTimeout(t *testing.T) {
	tr, err := NewOtelTracer(t.Context(), "gobox-test", &Config{
		Otel: Otel{
			Enabled:           true,
			CollectorEndpoint: "10.255.255.1:4317",
			SamplePercent:     100,
		},
	})
	assert.NilError(t, err)

	// Record and end a span so the batch processor actually has
	// something queued to export -- closeTracer on an empty batch has
	// nothing to flush and would pass even without the timeout fix.
	spanCtx := tr.startSpan(context.Background(), "test-span")
	tr.end(spanCtx)

	start := time.Now()
	tr.closeTracer(context.Background())
	elapsed := time.Since(start)

	// Generous upper bound: closeTracerTimeout applies twice in the
	// worst case (once for ForceFlush, once for Shutdown), plus room
	// for scheduling noise -- but this must stay far under the old
	// 3-second Shutdown-only timeout, which itself left ForceFlush
	// completely unbounded.
	assert.Assert(t, elapsed < 3*closeTracerTimeout,
		"closeTracer took %v, expected it to return within roughly %v", elapsed, 2*closeTracerTimeout)
}
