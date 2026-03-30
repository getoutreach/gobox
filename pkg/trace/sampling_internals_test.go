//go:build !or_e2e

package trace

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestContextForceTrace(t *testing.T) {
	ctx := t.Context()
	assert.Assert(t, !isTracingForced(ctx))

	ctx = forceTracing(ctx)
	assert.Assert(t, isTracingForced(ctx))
}
