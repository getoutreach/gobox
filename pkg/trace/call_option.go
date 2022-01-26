package trace

import (
	"time"

	"github.com/getoutreach/gobox/internal/call"
)

// WithScheduledTime set the call Info scheduled at time
func WithScheduledTime(t time.Time) call.CallOption {
	return func(c *call.Info) {
		c.Times.Scheduled = t
	}
}
