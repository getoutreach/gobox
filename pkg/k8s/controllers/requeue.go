package controllers

import (
	"time"

	"github.com/getoutreach/services/pkg/retry"
)

const (
	// MinRequeueInterval is the min duration between tries on same CR. This is exposed for testing.
	MinRequeueInterval = 1 * time.Minute
	// MaxRequeueInterval is used when either the CRD or CR have issues or if there are permission
	// issues to read CR or update its status. It is also used as a max retry interval, after too many retries.
	// In this case we want controllers to keep trying, but very slow (waiting for the fix to be pushed).
	MaxRequeueInterval = 60 * time.Minute
)

var requeueConfig = retry.New(
	retry.WithDelay(MinRequeueInterval, MaxRequeueInterval),
	retry.WithMultiplier(2),
	// No need in jiiter in controllers, easier to test without it
	retry.WithJitter(0),
)

// requeueDuration returns the requeue interval to retry, based on number of times this CR failed so far.
func requeueDuration(failCount int) time.Duration {
	// updateStatus sets failCount after updating the CR's status
	// it will always be 1 if reported.
	if failCount == 0 {
		// if we did not set the failCount, updateStatus failed, likely due to permission issues
		return MaxRequeueInterval
	}

	return requeueConfig.Backoff(failCount - 1)
}
