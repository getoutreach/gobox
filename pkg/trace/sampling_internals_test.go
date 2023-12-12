//go:build !or_e2e

package trace

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
)

func TestContextForceTrace(t *testing.T) {
	ctx := context.Background()
	assert.Assert(t, !isTracingForced(ctx))

	ctx = forceTracing(ctx)
	assert.Assert(t, isTracingForced(ctx))
}
