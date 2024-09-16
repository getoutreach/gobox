// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: implements RetryableOnce

package async

import (
	"sync"
	"sync/atomic"
)

// RetryableOnce is modified from sync.Once with a tweak to allow retry
type RetryableOnce struct {
	// done indicates whether the action has been performed.
	// It is first in the struct because it is used in the hot path.
	// The hot path is inlined at every call site.
	// Placing done first allows more compact instructions on some architectures (amd64/386),
	// and fewer instructions (to calculate offset) on other architectures.
	done atomic.Uint32
	m    sync.Mutex
}

// Do calls the function f if and only if one of the following of two cases
// 1. Do is being called for the first time for this instance of RetryableOnce
// 2. Do has been called multiple times before and f returns false in all those calls.
//
// If f panics, Do considers it as "done"; future calls of Do return
// without calling f.
// This function return true when future calls of Do will no longer call f
func (o *RetryableOnce) Do(f func() bool) bool {
	// Note: Here is an incorrect implementation of Do:
	//
	//	if o.done.CompareAndSwap(0, 1) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the o.done.Store must be delayed until after f returns.

	if o.done.Load() == 0 {
		// Outlined slow-path to allow inlining of the fast-path.
		return o.doSlow(f)
	}
	return true
}

func (o *RetryableOnce) doSlow(f func() bool) bool {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done.Load() == 0 {
		shouldRetry := false
		defer func() {
			if !shouldRetry {
				// if f panic or return true, future calls of Do
				// return without calling f
				o.done.Store(1)
			}
		}()

		shouldRetry = !f()
		return !shouldRetry
	}

	return true
}
