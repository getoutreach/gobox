package controllers

// wrappedError is a helper to define PropagateError and PermanentError
type wrappedError struct {
	Original error
}

// Error delegates error interface impl to the Original
func (we wrappedError) Error() string {
	return we.Original.Error()
}

// PermanentError wraps an existing error to make it permanent (e.g. no more retries allowed).
type PermanentError struct {
	wrappedError
}

func Permanent(err error) error {
	return PermanentError{wrappedError: wrappedError{Original: err}}
}

// PropagateError wraps an existing error to mark it as propagatable to controller. If Reconciler's handler
// returns wraps the error with this struct, Reconciler unwraps it and return it to the controller infra.
type PropagateError struct {
	wrappedError
}

func Propagate(err error) error {
	return PropagateError{wrappedError: wrappedError{Original: err}}
}
