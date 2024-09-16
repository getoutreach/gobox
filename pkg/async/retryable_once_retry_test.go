// Copyright 2024 Outreach Corporation. All Rights Reserved.
//
// Description: additional tests for verifying the retry behavior
package async_test

import (
	"testing"

	. "github.com/getoutreach/gobox/pkg/async"
)

// TestOnceRetry verifies the retries behavior
func TestOnceRetry(t *testing.T) {
	o := new(one)
	var once RetryableOnce

	result := once.Do(func() bool {
		o.Increment()
		return false
	})
	if *o != 1 {
		t.Fatalf("Once.Do must be called once")
	}
	if result {
		t.Fatalf("Once.Do must return false to indicate 'not done'")
	}

	// This once.Do should still call the func since the first attempt requests a retry
	result = once.Do(func() bool {
		o.Increment()
		return true
	})
	if *o != 2 {
		t.Fatalf("Once.Do must be called twice")
	}
	if !result {
		t.Fatalf("Once.Do must return true to indicate 'done'")
	}

	result = once.Do(func() bool {
		o.Increment()
		return false
	})
	if *o != 2 {
		t.Fatalf("Once.Do must be called exactly twice")
	}
	if !result {
		t.Fatalf("Once.Do must return true to indicate 'done'")
	}
}
