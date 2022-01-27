package trace_test

import (
	"context"
	"testing"
	"time"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/pkg/trace"
	"gotest.tools/v3/assert"
)

func TestWithOptions(t *testing.T) {
	ctx := context.Background()
	scheduledAt := time.Now()

	var callInfo *call.Info
	ctx = trace.StartCall(
		ctx,
		"test",
		trace.WithScheduledTime(scheduledAt),
		trace.AsGRPCCall(),
		call.Option(func(c *call.Info) {
			callInfo = c
		}),
	)

	assert.Equal(t, scheduledAt, callInfo.Times.Scheduled)
	assert.Equal(t, call.TypeGRPC, callInfo.Type)
}
