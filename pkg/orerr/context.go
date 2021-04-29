package orerr

import "context"

// CancelWithError returns a context and a cancel function where the
// cancel function can override the error reported by the context.
//
// This function is similar to context.WithCancel except that the
// cancel function can specify an error.  Note that the cancel
// function can be called with nil args to make it behave exactly like
// context.WithCancel.
func CancelWithError(ctx context.Context) (c context.Context, cancel func(err error)) {
	inner, innerCancel := context.WithCancel(ctx)
	result := &contextWithError{inner, nil}
	return result, func(err error) {
		if err != nil {
			result.err = err
		}
		innerCancel()
	}
}

type contextWithError struct {
	context.Context
	err error
}

// Err returns any error captured in CancelWithError
func (c *contextWithError) Err() error {
	if c.err == nil {
		return c.Context.Err()
	}
	return c.err
}
