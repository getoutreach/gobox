package trace_test

import (
	"context"
	"testing"
	"time"

	"github.com/getoutreach/gobox/internal/call"
	"github.com/getoutreach/gobox/pkg/trace"
	"gotest.tools/v3/assert"
)

func startCall(opt call.Option) *call.Info {
	var callInfo *call.Info
	trace.StartCall(
		context.Background(),
		"test",
		call.Option(func(c *call.Info) {
			callInfo = c
		}),
		opt,
	)
	return callInfo
}

func TestWithOptions(t *testing.T) {
	scheduledAt := time.Now()
	callInfo := startCall(trace.WithScheduledTime(scheduledAt))
	assert.Equal(t, scheduledAt, callInfo.Times.Scheduled)
}

func TestAsGRPCCall(t *testing.T) {
	callInfo := startCall(trace.AsGRPCCall())
	assert.Equal(t, call.TypeGRPC, callInfo.Type)
}

func TestAsOutboundCall(t *testing.T) {
	callInfo := startCall(trace.AsOutboundCall())
	assert.Equal(t, call.TypeOutbound, callInfo.Type)
}
