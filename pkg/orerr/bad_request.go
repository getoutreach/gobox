// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Bed request error
package orerr

import (
	"fmt"

	"github.com/getoutreach/gobox/pkg/statuscodes"
)

// Violation describe particular input violation. It might be field and domain specific.
type Violation struct {
	// Field describes the field path with request object graph.
	// When not provided the violation is related to a whole entitity.
	Field *string

	// Domain is optional identifier of the domain name of the validation subject.
	Domain *string

	// Reason of the error. In most of the cases the validation rule indentifier.
	Reason string

	// Additional structured details about this error. Could be used for localization of the error.
	Metadata map[string]string
}

// NewViolation creates a new intance of Violation
func NewViolation(reason string) *Violation {
	return &Violation{
		Reason: reason,
	}
}

// WithField allows to specify field path of the field that violation belongs to
func (v *Violation) WithField(field string) *Violation {
	v.Field = &field
	return v
}

// WithField allows to specify domain name of the field that violation belongs to
func (v *Violation) WithDomain(domain string) *Violation {
	v.Domain = &domain
	return v
}

// WithField allows to specify violation meta data
func (v *Violation) WithMeta(m map[string]string) *Violation {
	v.Metadata = m
	return v
}

// BadRequestError represents an invalidate input error
type BadRequestError struct {
	// Err is an original err
	Err error

	// Violations particular violations
	Violations []*Violation
}

// NewBadRequestError return ready made intance of the BadRequestError error.
// It wraps given error with the BadRequest status
func NewBadRequestError(err error, violations ...*Violation) error {
	err = New(err, WithStatus(statuscodes.BadRequest))
	return &BadRequestError{
		Err:        err,
		Violations: violations,
	}
}

// Error implements the err interface.
func (e BadRequestError) Error() string {
	return fmt.Sprintf("bad request: %s", e.Err.Error())
}

// Unwrap returns the inner error.
func (e BadRequestError) Unwrap() error {
	return e.Err
}

// WithViolations adds more violations into the error
func (e *BadRequestError) WithViolations(violations ...*Violation) *BadRequestError {
	e.Violations = append(e.Violations, violations...)
	return e
}
