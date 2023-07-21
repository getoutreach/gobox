// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Cond.go provides a context respecting sync condition

package async

import (
	"context"
	"sync"
	"sync/atomic"
)

// Cond mimics sync.Cond in purpose, with the added goals of easing usability slightly and respecting context expiry.
//
// It provides functionality similar sync.Cond (excepting there is no `Signal` method), except:
// - the Wait method exits with error if the context cancels.
// - it provides WaitForCondition, which intends to encapsulate the common pattern of acquiring a lock,
// checking a condition, and releasing the lock before waiting for a state change if the condition is not met.
type Cond struct {
	pointer atomic.Pointer[chan struct{}]
	Mu      sync.Mutex
}

// ch returns the channel that Waiters are waiting on,
// possibly creating one if it hasn't been initialized
func (c *Cond) ch() chan struct{} {
	// non atomic check for nil channel
	load := c.pointer.Load()
	if load == nil {
		t := make(chan struct{})
		c.pointer.CompareAndSwap(nil, &t)
		return t
	}
	return *load
}

// Wait waits for the state change Broadcast until context ends.
// If the returned error is non-nil, then the context ended before the state change Broadcast.
func (c *Cond) Wait(ctx context.Context) error {
	ch := c.ch()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Broadcast signals the state change to all Waiters
func (c *Cond) Broadcast() {
	// swap in a new channel, close the old one
	newChan := make(chan struct{})
	ch := c.pointer.Swap(&newChan)
	if ch != nil {
		close(*ch)
	}
}

// WaitForCondition acquires Cond's lock, then checks if the condition is true. If the condition is not true,
// or the lock was not available, it releases the locker and waits for the state change Broadcast.
// If the context ends during any of these operations, the context error is returned.
//
// WaitForCondition returns an unlock function that should always be called to unlock the locker.
// unlock is safe to call regardless  of error.
// Error should only be returned if the context ends before the condition is met.
//
// If it returns without error, it also locks the provided locker and the caller must call the returned function
// to unlock it. Until they call unlock, the state should not be changed.
//
// This method encapsulates the instructions in sync.Cond.Wait:
//
// """
// Because c.L is not locked while Wait is waiting, the caller
// typically cannot assume that the condition is true when
// Wait returns. Instead, the caller should Wait in a loop:
//
//		c.L.Lock()
//		for !condition() {
//	    	c.Wait()
//		}
//		... make use of condition ...
//		c.L.Unlock()
//
// """
//
// Instead, you can do the following:
//
//		var c Cond
//
//		unlock, err := c.WaitForCondition(ctx, func() bool {
//		   // check condition
//		    return true
//		})
//		if err != nil {
//		    // context expired before condition was met
//		}
//	 	... make use of condition ...
//	 	unlock()
func (c *Cond) WaitForCondition(ctx context.Context, condition func() bool) (unlock func(),
	err error) {
	for {
		locked := c.Mu.TryLock()
		// we have the lock, we can safely check the condition
		ok := locked && condition()

		if !ok {
			// condition not met
			if locked {
				// but we acquired the lock. so unlock it...
				c.Mu.Unlock()
			}

			// either way, wait for the next signal
			waitErr := c.Wait(ctx)
			if waitErr != nil {
				return func() {}, waitErr
			}
		} else {
			// condition met, return the unlock function and nil error
			// client must call the unlock function to unlock the mutex
			// client guaranteed the condition holds while mutex lock is held.
			return func() {
				c.Mu.Unlock()
				c.Broadcast()
			}, nil
		}
	}
}
