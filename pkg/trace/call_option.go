// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides various call options functions

package trace

import (
	"time"

	"github.com/getoutreach/gobox/internal/call"
)

// WithScheduledTime set the call Info scheduled at time
func WithScheduledTime(t time.Time) call.Option {
	return func(c *call.Info) {
		c.Times.Scheduled = t
	}
}

// AsGRPCCall set the call type to GRPC
func AsGRPCCall() call.Option {
	return func(c *call.Info) {
		c.Type = call.TypeGRPC
	}
}

// AsHTTPCall set the call type to HTTP
func AsHTTPCall() call.Option {
	return func(c *call.Info) {
		c.Type = call.TypeHTTP
	}
}

// AsOutboundCall set the call type to Outbound
func AsOutboundCall() call.Option {
	return func(c *call.Info) {
		c.Type = call.TypeOutbound
	}
}
