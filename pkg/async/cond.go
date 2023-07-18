// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Cond.go provides a context respecting sync condition

package async

import (
	"context"
	"sync"
	"sync/atomic"
)

// Cond is a sync.Cond that respects context cancellation.
// It provides equivalent functionality to sync.Cond (excepting there is no `Signal` method), except that
// the Wait method exits with error if the context cancels.
type Cond struct {
	pointer atomic.Pointer[chan struct{}]
}

// ch returns the channel that Waiters are waiting on, possibly creating one if it doesn't exist.
func (c *Cond) ch() chan struct{} {
	t := make(chan struct{})
	c.pointer.CompareAndSwap(nil, &t)
	return *c.pointer.Load()
}

// Wait waits for the state change Broadcast until context ends.
func (c *Cond) Wait(ctx context.Context) error {
	select {
	case <-c.ch():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Broadcast signals the state change to all Waiters
func (c *Cond) Broadcast() {
	// now that we retrieved the channel, new calls to Wait should get a new channel
	ch := c.pointer.Swap(nil)
	if ch != nil {
		close(*ch)
	}
}

// WaitForCondition checks if the condition is true or the context is done, otherwise
// it waits for the state change Broadcast.
//
// if it returns without error, it also locks the provided locker and the caller must call the returned function
// to unlock it. Until they call unlock, the state should not be changed.
func (c *Cond) WaitForCondition(ctx context.Context, locker sync.Locker, condition func() bool) (func(), error) {
	for {
		err := c.lockWithContext(ctx, locker)
		if err != nil {
			return func() {}, err
		}

		// we have the lock, we can safely check the condition
		ok := condition()

		if !ok {
			// condition not met
			// but we acquired the lock. so unlock it...
			locker.Unlock()

			// either way, wait for the next signal
			waitErr := c.Wait(ctx)
			if waitErr != nil {
				return func() {}, waitErr
			}
		} else {
			// condition met, return the unlock function and nil error
			// client must call the unlock function to unlock the mutex
			// client guaranteed the condition holds while mutex lock is held.
			return locker.Unlock, nil
		}
	}
}

// lockWithContext waits to either acquire the lock or for the context to end.
// It returns an error if context ends before it can acquire the lock
func (c *Cond) lockWithContext(ctx context.Context, locker sync.Locker) error {
	lockAcquired := make(chan struct{})
	go func() {
		locker.Lock()
		close(lockAcquired)
	}()
	select {
	case <-lockAcquired:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
