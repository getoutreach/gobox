package controllers

import "time"

// requeueIntervals are the intervals with which we will requeue failed CRs
// last value (also available as maxRequeueInterval) is used after we tried all
var requeueIntervals = []time.Duration{
	1 * time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	20 * time.Minute,
	60 * time.Minute,
}

// MinRequeueInterval is the min duration between tries on same CR. This is exposed for testing.
func MinRequeueInterval() time.Duration {
	return requeueIntervals[0]
}

// MaxRequeueInterval is used when either the CRD or CR have issues or if there are permission
// issues to read CR or update its status. It is also used as a max retry interval, after too many retries.
// In this case we want controllers to keep trying, but very slow (waiting for the fix to be pushed).
func MaxRequeueInterval() time.Duration {
	return requeueIntervals[len(requeueIntervals)-1]
}

// requeueDuration returns the requeue interval to retry, based on number of times this CR failed so far.
func requeueDuration(failCount int) time.Duration {
	// updateStatus sets failCount after updating the CR's status
	// it will always be 1 if reported.
	if failCount == 0 {
		// if we did not set the failCount, updateStatus failed, likely due to permission issues
		return MaxRequeueInterval()
	}

	if failCount <= len(requeueIntervals) {
		return requeueIntervals[failCount-1]
	}

	return MaxRequeueInterval()
}
