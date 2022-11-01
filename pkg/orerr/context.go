// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements custom error wrappers for context errors

package orerr

import (
	"context"
	"sync"
)

// CancelWithError returns a context and a cancel function where the
// cancel function can override the error reported by the context.
//
// This function is similar to context.WithCancel except that the
// cancel function can specify an error.  Note that the cancel
// function can be called with nil args to make it behave exactly like
// context.WithCancel.
func CancelWithError(ctx context.Context) (c context.Context, cancel func(err error)) {
	inner, innerCancel := context.WithCancel(ctx)
	result := &contextWithError{Context: inner, err: nil}
	return result, func(err error) {
		if err != nil {
			result.mu.Lock()
			result.err = err
			result.mu.Unlock()
		}
		innerCancel()
	}
}

type contextWithError struct {
	context.Context
	err error
	mu  sync.Mutex
}

// Err returns any error captured in CancelWithError
func (c *contextWithError) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		return c.Context.Err()
	}
	return c.err
}
