// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: Custom error with list of details compatible with JSON API spec
package orerr

import (
	"fmt"
)

// ErrSource describes source location of the error
type ErrSource struct {
	Pointer string
}

// ErrDetail describes detail of an error
// It's compatible with a single API error as per: https://jsonapi.org/format/#error-objects
type ErrDetail struct {
	ID     string
	Title  string
	Detail string
	Code   *string
	Source *ErrSource
	Meta   map[string]string
}

// NewErrDetail creates a new intance of ErrDetail
func NewErrDetail(id, title, detail string) ErrDetail {
	return ErrDetail{
		ID:     id,
		Title:  title,
		Detail: detail,
	}
}

// WithCode allows to specify error code
func (v ErrDetail) WithCode(code *string) ErrDetail {
	v.Code = code
	return v
}

// WithSourcePointer allows to specify source pointer
func (v ErrDetail) WithSourcePointer(pointer string) ErrDetail {
	v.Source = &ErrSource{
		Pointer: pointer,
	}
	return v
}

// WithField allows to specify violation meta data
func (v ErrDetail) WithMeta(m map[string]string) ErrDetail {
	v.Meta = m
	return v
}

// ErrDetails represents an error with list of details
type ErrDetails struct {
	// err is the original err
	err error

	// Details is the list of error details
	Details []ErrDetail
}

// NewErrDetails return ready made intance of the ErrDetails error.
// It wraps given error
func NewErrDetails(err error, details ...ErrDetail) error {
	return &ErrDetails{
		err:     err,
		Details: details,
	}
}

// Error implements the err interface.
func (e ErrDetails) Error() string {
	var details []string
	for _, d := range e.Details {
		details = append(details, fmt.Sprintf("%s(%s): %s", d.Title, d.ID, d.Detail))
	}

	return fmt.Sprintf("Details: %v, Wrapped: %s", details, e.err.Error())
}

// Unwrap returns the inner error.
func (e ErrDetails) Unwrap() error {
	return e.err
}

// WithDetails adds more details into the error
func (e *ErrDetails) WithDetails(details ...ErrDetail) *ErrDetails {
	e.Details = append(e.Details, details...)
	return e
}
