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
	scheduledAt := time.Now()

	var callInfo *call.Info
	trace.StartCall(
		context.Background(),
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

func TestAsOutboundCall(t *testing.T) {
	var callInfo *call.Info
	trace.StartCall(
		context.Background(),
		"test",
		trace.AsOutboundCall(),
		call.Option(func(c *call.Info) {
			callInfo = c
		}),
	)

	assert.Equal(t, call.TypeOutbound, callInfo.Type)
}
